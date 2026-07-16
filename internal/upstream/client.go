package upstream

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	defaultAPIBaseURL   = "https://api.x.ai/v1"
	defaultOAuthBaseURL = "https://cli-chat-proxy.grok.com/v1"
	maxDiagnosticBytes  = 2 << 20
	clientVersion       = "0.2.99"
	clientIdentifier    = "grok-shell"
)

type HTTPError struct {
	StatusCode int
	Header     http.Header
	Body       []byte
	Truncated  bool
}

func (e *HTTPError) Error() string {
	if e == nil {
		return "xAI 上游返回未知错误"
	}
	if detail := diagnosticSnippet(e.Body); detail != "" {
		return fmt.Sprintf("xAI 上游返回 HTTP %d: %s", e.StatusCode, detail)
	}
	return fmt.Sprintf("xAI 上游返回 HTTP %d", e.StatusCode)
}

// diagnosticSnippet extracts a short, human-readable summary from an upstream error body.
// JSON error.message is preferred; otherwise HTML/plain text is stripped and truncated.
func diagnosticSnippet(body []byte) string {
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return ""
	}
	var envelope struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Code    string `json:"code"`
		} `json:"error"`
		Message string `json:"message"`
		Detail  string `json:"detail"`
	}
	if json.Unmarshal(body, &envelope) == nil {
		switch {
		case strings.TrimSpace(envelope.Error.Message) != "":
			return strings.TrimSpace(envelope.Error.Message)
		case strings.TrimSpace(envelope.Message) != "":
			return strings.TrimSpace(envelope.Message)
		case strings.TrimSpace(envelope.Detail) != "":
			return strings.TrimSpace(envelope.Detail)
		}
	}
	// Strip common HTML wrappers so nginx/Cloudflare 404 pages remain readable.
	plain := trimmed
	for _, tag := range []string{"<html>", "</html>", "<head>", "</head>", "<body>", "</body>", "<center>", "</center>", "<hr>", "<hr/>", "<title>", "</title>", "<h1>", "</h1>"} {
		plain = strings.ReplaceAll(plain, tag, " ")
	}
	plain = strings.Join(strings.Fields(plain), " ")
	if plain == "" {
		return ""
	}
	const maxLen = 240
	if len(plain) > maxLen {
		return plain[:maxLen] + "…"
	}
	return plain
}

type Client struct {
	http         *http.Client
	credentials  CredentialSource
	apiBaseURL   string
	oauthBaseURL string
	agentID      string
	sessionID    string
}

func NewClient(httpClient *http.Client, credentials CredentialSource) *Client {
	if httpClient == nil {
		transport := &http.Transport{
			Proxy: http.ProxyFromEnvironment, ForceAttemptHTTP2: true, MaxIdleConns: 64, MaxIdleConnsPerHost: 32,
			IdleConnTimeout: 90 * time.Second, TLSHandshakeTimeout: 10 * time.Second, ResponseHeaderTimeout: 60 * time.Second,
		}
		httpClient = &http.Client{Transport: transport}
	}
	return &Client{http: httpClient, credentials: credentials, apiBaseURL: defaultAPIBaseURL, oauthBaseURL: defaultOAuthBaseURL, agentID: randomHex(16), sessionID: randomUUID()}
}

func (c *Client) Models(ctx context.Context) (*http.Response, error) {
	return c.do(ctx, http.MethodGet, "/models", nil, false, "")
}

func (c *Client) Responses(ctx context.Context, body []byte, stream bool) (*http.Response, error) {
	var envelope struct {
		Model string `json:"model"`
	}
	_ = json.Unmarshal(body, &envelope)
	return c.do(ctx, http.MethodPost, "/responses", body, stream, envelope.Model)
}

func (c *Client) do(ctx context.Context, method, path string, body []byte, stream bool, model string) (*http.Response, error) {
	if c.credentials == nil {
		return nil, fmt.Errorf("缺少上游凭据源")
	}
	authorization, err := c.credentials.Authorization(ctx)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(authorization.Token) == "" {
		return nil, fmt.Errorf("上游凭据为空")
	}
	baseURL := c.apiBaseURL
	if authorization.Mode == ModeOAuth {
		baseURL = c.oauthBaseURL
	}
	var reader io.Reader
	if len(body) > 0 {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(baseURL, "/")+"/"+strings.TrimLeft(path, "/"), reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+authorization.Token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "GrokProxy/2")
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	if stream {
		req.Header.Set("Accept", "text/event-stream")
		req.Header.Set("Accept-Encoding", "identity")
	}
	if authorization.Mode == ModeOAuth {
		c.applyOAuthHeaders(req, model)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	if err := normalizeGzip(resp); err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()
		data, readErr := io.ReadAll(io.LimitReader(resp.Body, maxDiagnosticBytes+1))
		if readErr != nil {
			return nil, readErr
		}
		truncated := len(data) > maxDiagnosticBytes
		if truncated {
			data = data[:maxDiagnosticBytes]
		}
		return nil, &HTTPError{StatusCode: resp.StatusCode, Header: resp.Header.Clone(), Body: data, Truncated: truncated}
	}
	return resp, nil
}

func (c *Client) applyOAuthHeaders(req *http.Request, model string) {
	requestID := randomHex(16)
	conversationID := randomHex(16)
	req.Header.Set("X-XAI-Token-Auth", "xai-grok-cli")
	req.Header.Set("x-grok-client-version", clientVersion)
	req.Header.Set("x-grok-client-identifier", clientIdentifier)
	req.Header.Set("x-grok-client-surface", "tui")
	req.Header.Set("x-grok-client-name", clientIdentifier)
	req.Header.Set("x-grok-agent-id", c.agentID)
	req.Header.Set("x-grok-session-id", c.sessionID)
	req.Header.Set("x-grok-conv-id", conversationID)
	req.Header.Set("x-grok-req-id", requestID)
	req.Header.Set("x-grok-conversation-id", conversationID)
	req.Header.Set("x-grok-request-id", requestID)
	req.Header.Set("User-Agent", "grok-shell/"+clientVersion+" (windows; amd64)")
	if model != "" {
		req.Header.Set("x-grok-model-override", model)
	}
}

func normalizeGzip(response *http.Response) error {
	if response == nil || response.Body == nil || !strings.EqualFold(strings.TrimSpace(response.Header.Get("Content-Encoding")), "gzip") {
		return nil
	}
	reader, err := gzip.NewReader(response.Body)
	if err != nil {
		_ = response.Body.Close()
		return err
	}
	response.Body = &gzipBody{Reader: reader, source: response.Body}
	response.Header.Del("Content-Encoding")
	response.Header.Del("Content-Length")
	response.ContentLength = -1
	return nil
}

type gzipBody struct {
	*gzip.Reader
	source io.Closer
}

func (b *gzipBody) Close() error {
	readerErr := b.Reader.Close()
	sourceErr := b.source.Close()
	if readerErr != nil {
		return readerErr
	}
	return sourceErr
}

func randomHex(length int) string {
	value := make([]byte, length)
	if _, err := rand.Read(value); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(value)
}

func randomUUID() string {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	value[6] = (value[6] & 0x0f) | 0x40
	value[8] = (value[8] & 0x3f) | 0x80
	encoded := hex.EncodeToString(value)
	return encoded[:8] + "-" + encoded[8:12] + "-" + encoded[12:16] + "-" + encoded[16:20] + "-" + encoded[20:]
}
