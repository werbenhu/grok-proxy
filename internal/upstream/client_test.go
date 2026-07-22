package upstream

import (
	"compress/gzip"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type credentialFunc func(context.Context) (Authorization, error)

func (f credentialFunc) Authorization(ctx context.Context) (Authorization, error) { return f(ctx) }

func TestClientUsesAPIKeyHeadersAndPaths(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		if got := r.Header.Get("Authorization"); got != "Bearer xai-key" {
			t.Errorf("authorization = %q", got)
		}
		if got := r.Header.Get("X-XAI-Token-Auth"); got != "" {
			t.Errorf("oauth header = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/v1/models" {
			_, _ = io.WriteString(w, `{"object":"list","data":[]}`)
			return
		}
		_, _ = io.WriteString(w, `{"id":"resp_1","status":"completed","output":[]}`)
	}))
	defer server.Close()
	client := NewClient(server.Client(), credentialFunc(func(context.Context) (Authorization, error) {
		return Authorization{Mode: ModeAPIKey, Token: "xai-key"}, nil
	}))
	client.apiBaseURL = server.URL + "/v1"
	models, err := client.Models(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	_ = models.Body.Close()
	response, err := client.Responses(context.Background(), []byte(`{"model":"grok-4","input":"hi"}`), false)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	compact, err := client.ResponsesCompact(context.Background(), []byte(`{"model":"grok-4","input":"hi"}`))
	if err != nil {
		t.Fatal(err)
	}
	_ = compact.Body.Close()
	if strings.Join(paths, ",") != "/v1/models,/v1/responses,/v1/responses/compact" {
		t.Fatalf("paths = %v", paths)
	}
}

func TestClientUsesOAuthGrokHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for key, want := range map[string]string{
			"Authorization": "Bearer oauth-token", "X-XAI-Token-Auth": "xai-grok-cli",
			"x-grok-client-version": "0.2.99", "x-grok-client-identifier": "grok-shell", "x-grok-model-override": "grok-4",
		} {
			if got := r.Header.Get(key); got != want {
				t.Errorf("%s = %q, want %q", key, got, want)
			}
		}
		for _, key := range []string{"x-grok-agent-id", "x-grok-session-id", "x-grok-req-id", "x-grok-conv-id"} {
			if r.Header.Get(key) == "" {
				t.Errorf("missing %s", key)
			}
		}
		_, _ = io.WriteString(w, `{}`)
	}))
	defer server.Close()
	client := NewClient(server.Client(), credentialFunc(func(context.Context) (Authorization, error) {
		return Authorization{Mode: ModeOAuth, Token: "oauth-token"}, nil
	}))
	client.oauthBaseURL = server.URL
	resp, err := client.Responses(context.Background(), []byte(`{"model":"grok-4","input":"hi"}`), true)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
}

func TestClientNormalizesGzipResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Encoding", "gzip")
		writer := gzip.NewWriter(w)
		_, _ = writer.Write([]byte(`{"ok":true}`))
		_ = writer.Close()
	}))
	defer server.Close()
	transport := server.Client().Transport.(*http.Transport).Clone()
	transport.DisableCompression = true
	client := NewClient(&http.Client{Transport: transport}, credentialFunc(func(context.Context) (Authorization, error) {
		return Authorization{Mode: ModeAPIKey, Token: "key"}, nil
	}))
	client.apiBaseURL = server.URL
	resp, err := client.Models(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if string(body) != `{"ok":true}` || resp.Header.Get("Content-Encoding") != "" {
		t.Fatalf("body=%s headers=%v", body, resp.Header)
	}
}

func TestClientBoundsUpstreamErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = io.WriteString(w, strings.Repeat("x", maxDiagnosticBytes+100))
	}))
	defer server.Close()
	client := NewClient(server.Client(), credentialFunc(func(context.Context) (Authorization, error) {
		return Authorization{Mode: ModeAPIKey, Token: "key"}, nil
	}))
	client.apiBaseURL = server.URL
	_, err := client.Models(context.Background())
	var httpErr *HTTPError
	if !errors.As(err, &httpErr) || httpErr.StatusCode != http.StatusBadGateway || len(httpErr.Body) != maxDiagnosticBytes || !httpErr.Truncated {
		t.Fatalf("err = %#v", err)
	}
}

func TestHTTPErrorIncludesBodySnippet(t *testing.T) {
	err := &HTTPError{StatusCode: http.StatusNotFound, Body: []byte(`{"error":{"message":"model not found"}}`)}
	if got := err.Error(); !strings.Contains(got, "model not found") {
		t.Fatalf("error = %q", got)
	}
	htmlErr := &HTTPError{StatusCode: http.StatusNotFound, Body: []byte(`<html><head><title>404 Not Found</title></head><body><center><h1>404 Not Found</h1></center></body></html>`)}
	if got := htmlErr.Error(); !strings.Contains(got, "404 Not Found") || strings.Contains(got, "<html>") {
		t.Fatalf("html error = %q", got)
	}
}

func TestClientPropagatesCancellation(t *testing.T) {
	started := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { close(started); <-r.Context().Done() }))
	defer server.Close()
	client := NewClient(server.Client(), credentialFunc(func(context.Context) (Authorization, error) {
		return Authorization{Mode: ModeAPIKey, Token: "key"}, nil
	}))
	client.apiBaseURL = server.URL
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { _, err := client.Models(ctx); done <- err }()
	<-started
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v", err)
	}
}
