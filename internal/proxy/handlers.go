package proxy

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/werbenhu/grok-proxy/internal/protocol/conversation"
)

const maxResponseBodyBytes = 64 << 20

func (s *Server) inference(w http.ResponseWriter, r *http.Request, operation string) {
	body, err := io.ReadAll(io.LimitReader(r.Body, maxRequestBodyBytes+1))
	if err != nil {
		s.protocolError(w, operation, fmt.Errorf("读取请求体: %w", err))
		return
	}
	if len(body) > maxRequestBodyBytes {
		if operation == conversation.OperationMessages {
			writeAnthropicError(w, http.StatusRequestEntityTooLarge, "invalid_request_error", "request body exceeds 32 MiB")
		} else {
			writeOpenAIError(w, http.StatusRequestEntityTooLarge, "request_too_large", "request body exceeds 32 MiB")
		}
		return
	}
	var metadata struct {
		Model     string          `json:"model"`
		Stream    bool            `json:"stream"`
		Messages  json.RawMessage `json:"messages"`
		MaxTokens *int            `json:"max_tokens"`
	}
	if err := json.Unmarshal(body, &metadata); err != nil {
		s.protocolError(w, operation, fmt.Errorf("请求 JSON 无效: %w", err))
		return
	}
	if strings.TrimSpace(metadata.Model) == "" || len(metadata.Messages) == 0 || string(metadata.Messages) == "null" {
		s.protocolError(w, operation, errors.New("model and messages are required"))
		return
	}
	if operation == conversation.OperationMessages && metadata.MaxTokens == nil {
		s.protocolError(w, operation, errors.New("model, max_tokens, and messages are required"))
		return
	}
	converted, options, err := conversation.ConvertRequestWithOptions(body, metadata.Model, operation)
	if err != nil {
		s.protocolError(w, operation, err)
		return
	}
	if s.upstream == nil {
		s.handleError(w, operation, errors.New("上游未初始化"))
		return
	}
	resp, err := s.upstream.Responses(r.Context(), converted, metadata.Stream)
	if err != nil {
		s.handleError(w, operation, err)
		return
	}
	if resp == nil || resp.Body == nil {
		s.handleError(w, operation, errors.New("上游返回空响应"))
		return
	}
	defer resp.Body.Close()
	if metadata.Stream {
		copyHeaders(w.Header(), resp.Header, true)
		w.WriteHeader(resp.StatusCode)
		convertedStream := conversation.ConvertResponseStreamWithOptions(resp.Body, operation, options)
		defer convertedStream.Close()
		_, err = io.Copy(&flushWriter{writer: w}, convertedStream)
		if err != nil {
			s.stats.failed(err)
		}
		return
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBodyBytes+1))
	if err != nil {
		s.handleError(w, operation, err)
		return
	}
	if len(data) > maxResponseBodyBytes {
		s.handleError(w, operation, errors.New("上游响应超过 64 MiB"))
		return
	}
	convertedResponse, err := conversation.ConvertResponseJSONWithOptions(data, operation, options)
	if err != nil {
		s.handleError(w, operation, err)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(convertedResponse)
}

func (s *Server) protocolError(w http.ResponseWriter, operation string, err error) {
	s.stats.failed(err)
	if operation == conversation.OperationMessages {
		writeAnthropicError(w, http.StatusBadRequest, "invalid_request_error", err.Error())
		return
	}
	writeOpenAIError(w, http.StatusBadRequest, "invalid_request", err.Error())
}

type flushWriter struct{ writer http.ResponseWriter }

func (w *flushWriter) Write(value []byte) (int, error) {
	n, err := w.writer.Write(value)
	if flusher, ok := w.writer.(http.Flusher); ok {
		flusher.Flush()
	}
	return n, err
}
