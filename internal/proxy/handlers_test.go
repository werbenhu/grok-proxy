package proxy

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/werbenhu/grok-proxy/internal/config"
	"github.com/werbenhu/grok-proxy/internal/upstream"
)

func TestOpenAIJSONConversion(t *testing.T) {
	up := &fakeUpstream{responses: jsonResponse(`{"id":"resp_1","model":"grok-4","status":"completed","created_at":123,"output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"hello"}]}],"usage":{"input_tokens":3,"output_tokens":1}}`)}
	handler := New(config.Default(), up)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"grok-4","messages":[{"role":"user","content":"hi"}]}`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"object":"chat.completion"`) || !strings.Contains(rec.Body.String(), `"content":"hello"`) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var converted map[string]any
	if err := json.Unmarshal(up.lastBody, &converted); err != nil {
		t.Fatal(err)
	}
	if converted["model"] != "grok-4" || converted["input"] == nil {
		t.Fatalf("upstream body=%s", up.lastBody)
	}
}

func TestMessagesRequiresVersionAndConvertsJSON(t *testing.T) {
	up := &fakeUpstream{responses: jsonResponse(`{"id":"resp_2","model":"grok-4","status":"completed","output":[{"type":"message","content":[{"type":"output_text","text":"hello"}]}],"usage":{"input_tokens":4,"output_tokens":2}}`)}
	handler := New(config.Default(), up)
	body := `{"model":"grok-4","max_tokens":64,"messages":[{"role":"user","content":"hi"}]}`
	missing := httptest.NewRecorder()
	handler.ServeHTTP(missing, httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body)))
	if missing.Code != http.StatusBadRequest || !strings.Contains(missing.Body.String(), `"type":"error"`) {
		t.Fatalf("missing=%d %s", missing.Code, missing.Body.String())
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body))
	req.Header.Set("anthropic-version", "2023-06-01")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"type":"message"`) || !strings.Contains(rec.Body.String(), `"text":"hello"`) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestOpenAIAndAnthropicStreams(t *testing.T) {
	stream := "event: response.created\ndata: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_s\",\"model\":\"grok-4\"}}\n\n" +
		"event: response.output_text.delta\ndata: {\"type\":\"response.output_text.delta\",\"delta\":\"Hi\"}\n\n" +
		"event: response.completed\ndata: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_s\",\"model\":\"grok-4\",\"status\":\"completed\",\"usage\":{\"input_tokens\":1,\"output_tokens\":1}}}\n\n"
	for _, tt := range []struct {
		name, path, body, want string
		anthropic              bool
	}{
		{"openai", "/v1/chat/completions", `{"model":"grok-4","stream":true,"messages":[{"role":"user","content":"hi"}]}`, "[DONE]", false},
		{"anthropic", "/v1/messages", `{"model":"grok-4","stream":true,"max_tokens":64,"messages":[{"role":"user","content":"hi"}]}`, "event: message_stop", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			up := &fakeUpstream{responses: streamResponse(stream)}
			handler := New(config.Default(), up)
			req := httptest.NewRequest(http.MethodPost, tt.path, strings.NewReader(tt.body))
			if tt.anthropic {
				req.Header.Set("anthropic-version", "2023-06-01")
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "text/event-stream; charset=utf-8" || !strings.Contains(rec.Body.String(), tt.want) || !up.lastStream {
				t.Fatalf("status=%d headers=%v body=%s", rec.Code, rec.Header(), rec.Body.String())
			}
		})
	}
}

func TestProtocolAndUpstreamErrorsUseDownstreamShape(t *testing.T) {
	bad := New(config.Default(), &fakeUpstream{})
	rec := httptest.NewRecorder()
	bad.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"grok-4"}`)))
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"error"`) {
		t.Fatalf("bad=%d %s", rec.Code, rec.Body.String())
	}

	upErr := &upstream.HTTPError{StatusCode: http.StatusUnauthorized, Body: []byte(`{"error":{"message":"bad upstream token"}}`)}
	handler := New(config.Default(), &fakeUpstream{responses: func(context.Context, []byte, bool) (*http.Response, error) { return nil, upErr }})
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{"model":"grok-4","max_tokens":64,"messages":[{"role":"user","content":"hi"}]}`))
	req.Header.Set("anthropic-version", "2023-06-01")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized || !strings.Contains(rec.Body.String(), `"authentication_error"`) || !strings.Contains(rec.Body.String(), "bad upstream token") {
		t.Fatalf("upstream=%d %s", rec.Code, rec.Body.String())
	}

	generic := New(config.Default(), &fakeUpstream{responses: func(context.Context, []byte, bool) (*http.Response, error) { return nil, errors.New("network down") }})
	rec = httptest.NewRecorder()
	generic.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"grok-4","messages":[{"role":"user","content":"hi"}]}`)))
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("generic=%d %s", rec.Code, rec.Body.String())
	}
	stats := generic.Stats()
	if stats.TotalRequests != 1 || stats.ActiveRequests != 0 || !strings.Contains(stats.LastError, "network down") {
		t.Fatalf("stats=%+v", stats)
	}
}

func jsonResponse(body string) func(context.Context, []byte, bool) (*http.Response, error) {
	return func(context.Context, []byte, bool) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(body))}, nil
	}
}
func streamResponse(body string) func(context.Context, []byte, bool) (*http.Response, error) {
	return func(context.Context, []byte, bool) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: io.NopCloser(strings.NewReader(body))}, nil
	}
}
