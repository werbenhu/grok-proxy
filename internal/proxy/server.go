package proxy

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/werbenhu/grok-proxy/internal/config"
	"github.com/werbenhu/grok-proxy/internal/upstream"
)

const maxRequestBodyBytes = 32 << 20

type Upstream interface {
	Models(context.Context) (*http.Response, error)
	Responses(context.Context, []byte, bool) (*http.Response, error)
}

type Server struct {
	cfg      config.Config
	upstream Upstream
	stats    statisticsStore
}

func New(cfg config.Config, client Upstream) *Server { return &Server{cfg: cfg, upstream: client} }
func (s *Server) Stats() Statistics                  { return s.stats.snapshot() }

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/healthz" && r.Method == http.MethodGet {
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
		return
	}
	if !s.authorized(r) {
		if r.URL.Path == "/v1/messages" {
			writeAnthropicError(w, http.StatusUnauthorized, "authentication_error", "本地代理密钥无效")
		} else {
			writeOpenAIError(w, http.StatusUnauthorized, "invalid_api_key", "本地代理密钥无效")
		}
		return
	}
	finish := s.stats.begin()
	defer finish()
	switch {
	case r.URL.Path == "/v1/models" && r.Method == http.MethodGet:
		s.models(w, r)
	case r.URL.Path == "/v1/chat/completions" && r.Method == http.MethodPost:
		s.inference(w, r, "chat")
	case r.URL.Path == "/v1/messages":
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			writeAnthropicError(w, http.StatusMethodNotAllowed, "invalid_request_error", "method not allowed")
			return
		}
		if strings.TrimSpace(r.Header.Get("anthropic-version")) == "" {
			writeAnthropicError(w, http.StatusBadRequest, "invalid_request_error", "anthropic-version header is required")
			return
		}
		s.inference(w, r, "messages")
	default:
		writeOpenAIError(w, http.StatusNotFound, "not_found", "接口不存在")
	}
}

func (s *Server) authorized(r *http.Request) bool {
	expected := strings.TrimSpace(s.cfg.LocalKey)
	if expected == "" {
		return true
	}
	provided := strings.TrimSpace(r.Header.Get("x-api-key"))
	if provided == "" {
		value := strings.TrimSpace(r.Header.Get("Authorization"))
		if len(value) >= 7 && strings.EqualFold(value[:7], "Bearer ") {
			provided = strings.TrimSpace(value[7:])
		}
	}
	wantHash := sha256.Sum256([]byte(expected))
	gotHash := sha256.Sum256([]byte(provided))
	return subtle.ConstantTimeCompare(wantHash[:], gotHash[:]) == 1
}

func (s *Server) models(w http.ResponseWriter, r *http.Request) {
	if s.upstream == nil {
		s.handleError(w, "chat", errors.New("上游未初始化"))
		return
	}
	resp, err := s.upstream.Models(r.Context())
	if err != nil {
		s.handleError(w, "chat", err)
		return
	}
	defer resp.Body.Close()
	copyHeaders(w.Header(), resp.Header, false)
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

func (s *Server) handleError(w http.ResponseWriter, operation string, err error) {
	s.stats.failed(err)
	if errors.Is(err, context.Canceled) {
		return
	}
	status := http.StatusBadGateway
	message := err.Error()
	var httpErr *upstream.HTTPError
	if errors.As(err, &httpErr) {
		status = httpErr.StatusCode
		message = upstreamMessage(httpErr.Body, message)
	}
	if operation == "messages" {
		writeAnthropicError(w, status, anthropicErrorType(status), message)
		return
	}
	code := "upstream_error"
	if status == http.StatusUnauthorized {
		code = "invalid_api_key"
	}
	writeOpenAIError(w, status, code, message)
}

func upstreamMessage(body []byte, fallback string) string {
	var envelope struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
		Message string `json:"message"`
		Detail  string `json:"detail"`
	}
	if json.Unmarshal(body, &envelope) == nil {
		if envelope.Error.Message != "" {
			return envelope.Error.Message
		}
		if envelope.Message != "" {
			return envelope.Message
		}
		if envelope.Detail != "" {
			return envelope.Detail
		}
	}
	return fallback
}

func anthropicErrorType(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "invalid_request_error"
	case http.StatusUnauthorized:
		return "authentication_error"
	case http.StatusForbidden:
		return "permission_error"
	case http.StatusNotFound:
		return "not_found_error"
	case http.StatusRequestTimeout, http.StatusGatewayTimeout:
		return "timeout_error"
	case http.StatusTooManyRequests:
		return "rate_limit_error"
	case http.StatusServiceUnavailable:
		return "overloaded_error"
	default:
		return "api_error"
	}
}

func writeOpenAIError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]any{"message": message, "type": "invalid_request_error", "code": code}})
}
func writeAnthropicError(w http.ResponseWriter, status int, kind, message string) {
	writeJSON(w, status, map[string]any{"type": "error", "error": map[string]any{"type": kind, "message": message}})
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func copyHeaders(target, source http.Header, stream bool) {
	for _, key := range []string{"Content-Type", "x-request-id", "request-id", "Retry-After"} {
		if value := source.Values(key); len(value) > 0 {
			target[key] = append([]string(nil), value...)
		}
	}
	if stream {
		target.Set("Content-Type", "text/event-stream; charset=utf-8")
		target.Set("Cache-Control", "no-cache")
		target.Set("X-Accel-Buffering", "no")
	}
}

var _ http.Handler = (*Server)(nil)
