package proxy

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/werbenhu/grok-proxy/internal/config"
)

func TestHealthAndModels(t *testing.T) {
	up := &fakeUpstream{modelsBody: `{"object":"list","data":[{"id":"grok-4","object":"model"}]}`}
	cfg := testConfig()
	handler := New(cfg, up)
	health := httptest.NewRecorder()
	handler.ServeHTTP(health, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if health.Code != http.StatusOK || !strings.Contains(health.Body.String(), `"status":"ok"`) {
		t.Fatalf("health=%d %s", health.Code, health.Body.String())
	}
	models := httptest.NewRecorder()
	handler.ServeHTTP(models, authenticatedRequest(cfg, http.MethodGet, "/v1/models", nil))
	if models.Code != http.StatusOK || models.Body.String() != up.modelsBody {
		t.Fatalf("models=%d %s", models.Code, models.Body.String())
	}
	missing := httptest.NewRecorder()
	handler.ServeHTTP(missing, authenticatedRequest(cfg, http.MethodGet, "/missing", nil))
	if missing.Code != http.StatusNotFound {
		t.Fatalf("missing=%d", missing.Code)
	}
}

func TestLocalKeyAcceptsBearerAndXAPIKey(t *testing.T) {
	cfg := config.Default()
	cfg.LocalKey = "local-secret"
	handler := New(cfg, &fakeUpstream{modelsBody: `{}`})
	tests := []struct {
		name, header, value string
		want                int
	}{
		{"missing", "", "", http.StatusUnauthorized},
		{"wrong", "Authorization", "Bearer wrong", http.StatusUnauthorized},
		{"bearer", "Authorization", "Bearer local-secret", http.StatusOK},
		{"anthropic", "x-api-key", "local-secret", http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
			if tt.header != "" {
				req.Header.Set(tt.header, tt.value)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != tt.want {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestMessagesLocalKeyFailureUsesAnthropicError(t *testing.T) {
	cfg := config.Default()
	cfg.LocalKey = "local-secret"
	handler := New(cfg, &fakeUpstream{})
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{"model":"grok-4","max_tokens":64,"messages":[{"role":"user","content":"hi"}]}`))
	req.Header.Set("anthropic-version", "2023-06-01")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusUnauthorized || !strings.Contains(recorder.Body.String(), `"type":"authentication_error"`) {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestMessagesWrongMethodUsesAnthropicError(t *testing.T) {
	cfg := testConfig()
	handler := New(cfg, &fakeUpstream{})
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, authenticatedRequest(cfg, http.MethodGet, "/v1/messages", nil))
	if recorder.Code != http.StatusMethodNotAllowed || !strings.Contains(recorder.Body.String(), `"type":"invalid_request_error"`) {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestBodyLimit(t *testing.T) {
	cfg := testConfig()
	handler := New(cfg, &fakeUpstream{})
	body := strings.NewReader(strings.Repeat("x", maxRequestBodyBytes+1))
	req := authenticatedRequest(cfg, http.MethodPost, "/v1/chat/completions", body)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestRequestCancellationReachesUpstream(t *testing.T) {
	canceled := make(chan struct{})
	up := &fakeUpstream{responses: func(ctx context.Context, _ []byte, _ bool) (*http.Response, error) {
		<-ctx.Done()
		close(canceled)
		return nil, ctx.Err()
	}}
	cfg := testConfig()
	handler := New(cfg, up)
	ctx, cancel := context.WithCancel(context.Background())
	req := authenticatedRequest(cfg, http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"grok-4","messages":[{"role":"user","content":"hi"}]}`)).WithContext(ctx)
	done := make(chan struct{})
	go func() { handler.ServeHTTP(httptest.NewRecorder(), req); close(done) }()
	cancel()
	<-done
	<-canceled
}

type fakeUpstream struct {
	modelsBody string
	responses  func(context.Context, []byte, bool) (*http.Response, error)
	lastBody   []byte
	lastStream bool
}

func testConfig() config.Config {
	cfg := config.Default()
	cfg.LocalKey = "test-local-key"
	return cfg
}

func authenticatedRequest(cfg config.Config, method, target string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, target, body)
	req.Header.Set("Authorization", "Bearer "+cfg.LocalKey)
	return req
}

func (f *fakeUpstream) Models(context.Context) (*http.Response, error) {
	return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(f.modelsBody))}, nil
}
func (f *fakeUpstream) Responses(ctx context.Context, body []byte, stream bool) (*http.Response, error) {
	f.lastBody = append([]byte(nil), body...)
	f.lastStream = stream
	if f.responses != nil {
		return f.responses(ctx, body, stream)
	}
	return nil, nil
}
