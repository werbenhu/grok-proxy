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

	"github.com/werbenhu/grok-proxy/internal/upstream"
)

func TestOpenAIJSONConversion(t *testing.T) {
	up := &fakeUpstream{responses: jsonResponse(`{"id":"resp_1","model":"grok-4","status":"completed","created_at":123,"output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"hello"}]}],"usage":{"input_tokens":3,"output_tokens":1}}`)}
	cfg := testConfig()
	handler := New(cfg, up)
	req := authenticatedRequest(cfg, http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"grok-4","messages":[{"role":"user","content":"hi"}]}`))
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

func TestResponsesPassthroughPreservesImagesAndToolOutputs(t *testing.T) {
	responseBody := `{"id":"resp_native","model":"grok-4.5","status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"done"}]}]}`
	requestBody := `{
  "model": "grok-4.5",
  "input": [
    {"role":"user","content":[{"type":"input_image","image_url":"data:image/png;base64,iVBORw0KGgoAAA"}]},
    {"type":"function_call_output","call_id":"call_1","output":[{"type":"input_text","text":"screenshot"},{"type":"input_image","image_url":"data:image/png;base64,AAAA"}]}
  ],
  "tools": [{"type":"function","name":"view_image","description":"Inspect an image","parameters":{"type":"object"}}],
  "tool_choice": "auto"
}`
	up := &fakeUpstream{responses: jsonResponse(responseBody)}
	cfg := testConfig()
	handler := New(cfg, up)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, authenticatedRequest(cfg, http.MethodPost, "/v1/responses", strings.NewReader(requestBody)))
	if rec.Code != http.StatusOK || rec.Body.String() != responseBody {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if string(up.lastBody) != requestBody {
		t.Fatalf("request was not passed through verbatim:\n%s", up.lastBody)
	}
}

func TestResponsesStreamPassthrough(t *testing.T) {
	stream := "event: response.created\ndata: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_s\"}}\n\n" +
		"event: response.output_text.delta\ndata: {\"type\":\"response.output_text.delta\",\"delta\":\"Hi\"}\n\n"
	requestBody := `{"model":"grok-4.5","stream":true,"input":"hi"}`
	up := &fakeUpstream{responses: streamResponse(stream)}
	cfg := testConfig()
	rec := httptest.NewRecorder()
	New(cfg, up).ServeHTTP(rec, authenticatedRequest(cfg, http.MethodPost, "/v1/responses", strings.NewReader(requestBody)))
	if rec.Code != http.StatusOK || rec.Body.String() != stream || !up.lastStream {
		t.Fatalf("status=%d stream=%v body=%q", rec.Code, up.lastStream, rec.Body.String())
	}
	if string(up.lastBody) != requestBody {
		t.Fatalf("request was not passed through verbatim: %s", up.lastBody)
	}
}

func TestResponsesCompactPassthrough(t *testing.T) {
	requestBody := `{"model":"grok-4.5","input":[{"role":"user","content":[{"type":"input_text","text":"compact this"}]}]}`
	responseBody := `{"id":"resp_compact","status":"completed","output":[{"type":"compaction","encrypted_content":"opaque"}]}`
	up := &fakeUpstream{responsesCompact: func(context.Context, []byte) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(responseBody))}, nil
	}}
	cfg := testConfig()
	rec := httptest.NewRecorder()
	New(cfg, up).ServeHTTP(rec, authenticatedRequest(cfg, http.MethodPost, "/v1/responses/compact", strings.NewReader(requestBody)))
	if rec.Code != http.StatusOK || rec.Body.String() != responseBody {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if string(up.lastCompactBody) != requestBody {
		t.Fatalf("compact request was not passed through verbatim: %s", up.lastCompactBody)
	}
}

func TestResponsesValidation(t *testing.T) {
	cfg := testConfig()
	up := &fakeUpstream{}
	handler := New(cfg, up)

	missingInput := httptest.NewRecorder()
	handler.ServeHTTP(missingInput, authenticatedRequest(cfg, http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"grok-4.5"}`)))
	if missingInput.Code != http.StatusBadRequest || !strings.Contains(missingInput.Body.String(), "model and input are required") {
		t.Fatalf("missing input=%d %s", missingInput.Code, missingInput.Body.String())
	}

	streamCompact := httptest.NewRecorder()
	handler.ServeHTTP(streamCompact, authenticatedRequest(cfg, http.MethodPost, "/v1/responses/compact", strings.NewReader(`{"model":"grok-4.5","stream":true,"input":"hi"}`)))
	if streamCompact.Code != http.StatusBadRequest || !strings.Contains(streamCompact.Body.String(), "stream is not supported") {
		t.Fatalf("stream compact=%d %s", streamCompact.Code, streamCompact.Body.String())
	}
	if up.lastBody != nil || up.lastCompactBody != nil {
		t.Fatal("invalid Responses request reached upstream")
	}
}

func TestMessagesRequiresVersionAndConvertsJSON(t *testing.T) {
	up := &fakeUpstream{responses: jsonResponse(`{"id":"resp_2","model":"grok-4","status":"completed","output":[{"type":"message","content":[{"type":"output_text","text":"hello"}]}],"usage":{"input_tokens":4,"output_tokens":2}}`)}
	cfg := testConfig()
	handler := New(cfg, up)
	body := `{"model":"grok-4","max_tokens":64,"messages":[{"role":"user","content":"hi"}]}`
	missing := httptest.NewRecorder()
	handler.ServeHTTP(missing, authenticatedRequest(cfg, http.MethodPost, "/v1/messages", strings.NewReader(body)))
	if missing.Code != http.StatusBadRequest || !strings.Contains(missing.Body.String(), `"type":"error"`) {
		t.Fatalf("missing=%d %s", missing.Code, missing.Body.String())
	}
	req := authenticatedRequest(cfg, http.MethodPost, "/v1/messages", strings.NewReader(body))
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
			cfg := testConfig()
			handler := New(cfg, up)
			req := authenticatedRequest(cfg, http.MethodPost, tt.path, strings.NewReader(tt.body))
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
	cfg := testConfig()
	bad := New(cfg, &fakeUpstream{})
	rec := httptest.NewRecorder()
	bad.ServeHTTP(rec, authenticatedRequest(cfg, http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"grok-4"}`)))
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"error"`) {
		t.Fatalf("bad=%d %s", rec.Code, rec.Body.String())
	}

	upErr := &upstream.HTTPError{StatusCode: http.StatusUnauthorized, Body: []byte(`{"error":{"message":"bad upstream token"}}`)}
	handler := New(cfg, &fakeUpstream{responses: func(context.Context, []byte, bool) (*http.Response, error) { return nil, upErr }})
	req := authenticatedRequest(cfg, http.MethodPost, "/v1/messages", strings.NewReader(`{"model":"grok-4","max_tokens":64,"messages":[{"role":"user","content":"hi"}]}`))
	req.Header.Set("anthropic-version", "2023-06-01")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized || !strings.Contains(rec.Body.String(), `"authentication_error"`) || !strings.Contains(rec.Body.String(), "bad upstream token") {
		t.Fatalf("upstream=%d %s", rec.Code, rec.Body.String())
	}

	generic := New(cfg, &fakeUpstream{responses: func(context.Context, []byte, bool) (*http.Response, error) { return nil, errors.New("network down") }})
	rec = httptest.NewRecorder()
	generic.ServeHTTP(rec, authenticatedRequest(cfg, http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"grok-4","messages":[{"role":"user","content":"hi"}]}`)))
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
