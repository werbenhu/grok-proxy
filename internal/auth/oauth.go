package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	defaultClientID       = "b1a00492-073a-47ea-816f-4c329264a828"
	defaultScope          = "openid profile email offline_access grok-cli:access api:access"
	defaultDeviceURL      = "https://auth.x.ai/oauth2/device/code"
	defaultTokenURL       = "https://auth.x.ai/oauth2/token"
	maxOAuthResponseBytes = 1 << 20
)

type DeviceAuthorization struct {
	DeviceCode              string        `json:"deviceCode"`
	UserCode                string        `json:"userCode"`
	VerificationURI         string        `json:"verificationUri"`
	VerificationURIComplete string        `json:"verificationUriComplete"`
	Interval                time.Duration `json:"-"`
	ExpiresIn               time.Duration `json:"-"`
	IntervalSeconds         int           `json:"intervalSeconds"`
	ExpiresInSeconds        int           `json:"expiresInSeconds"`
}

type Token struct {
	AccessToken  string
	RefreshToken string
	IDToken      string
	ExpiresAt    time.Time
}

type OAuthClient struct {
	http      *http.Client
	clientID  string
	scope     string
	deviceURL string
	tokenURL  string
}

func NewOAuthClient(httpClient *http.Client) *OAuthClient {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &OAuthClient{http: httpClient, clientID: defaultClientID, scope: defaultScope, deviceURL: defaultDeviceURL, tokenURL: defaultTokenURL}
}

func (c *OAuthClient) Start(ctx context.Context) (DeviceAuthorization, error) {
	body, status, err := c.postForm(ctx, c.deviceURL, url.Values{"client_id": {c.clientID}, "scope": {c.scope}})
	if err != nil {
		return DeviceAuthorization{}, err
	}
	if status < 200 || status >= 300 {
		return DeviceAuthorization{}, fmt.Errorf("xAI Device OAuth 返回 HTTP %d: %s", status, diagnostic(body))
	}
	var value struct {
		DeviceCode              string `json:"device_code"`
		UserCode                string `json:"user_code"`
		VerificationURI         string `json:"verification_uri"`
		VerificationURIComplete string `json:"verification_uri_complete"`
		Interval                int    `json:"interval"`
		ExpiresIn               int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &value); err != nil {
		return DeviceAuthorization{}, fmt.Errorf("解析 xAI Device OAuth: %w", err)
	}
	if value.DeviceCode == "" || value.UserCode == "" || value.VerificationURI == "" {
		return DeviceAuthorization{}, fmt.Errorf("xAI Device OAuth 返回字段不完整")
	}
	if value.Interval <= 0 {
		value.Interval = 5
	}
	if value.ExpiresIn <= 0 {
		value.ExpiresIn = 1800
	}
	return DeviceAuthorization{
		DeviceCode: value.DeviceCode, UserCode: value.UserCode, VerificationURI: value.VerificationURI,
		VerificationURIComplete: value.VerificationURIComplete, Interval: time.Duration(value.Interval) * time.Second,
		ExpiresIn: time.Duration(value.ExpiresIn) * time.Second, IntervalSeconds: value.Interval, ExpiresInSeconds: value.ExpiresIn,
	}, nil
}

func (c *OAuthClient) Poll(ctx context.Context, deviceCode string) (Token, error) {
	return c.exchange(ctx, url.Values{
		"grant_type": {"urn:ietf:params:oauth:grant-type:device_code"}, "client_id": {c.clientID}, "device_code": {deviceCode},
	}, "")
}

func (c *OAuthClient) Refresh(ctx context.Context, refreshToken string) (Token, error) {
	return c.exchange(ctx, url.Values{
		"grant_type": {"refresh_token"}, "client_id": {c.clientID}, "refresh_token": {refreshToken},
	}, refreshToken)
}

func (c *OAuthClient) exchange(ctx context.Context, form url.Values, fallbackRefresh string) (Token, error) {
	body, status, err := c.postForm(ctx, c.tokenURL, form)
	if err != nil {
		return Token{}, err
	}
	var value struct {
		AccessToken      string `json:"access_token"`
		RefreshToken     string `json:"refresh_token"`
		IDToken          string `json:"id_token"`
		ExpiresIn        int    `json:"expires_in"`
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	if err := json.Unmarshal(body, &value); err != nil {
		return Token{}, fmt.Errorf("解析 xAI OAuth: %w", err)
	}
	if status < 200 || status >= 300 {
		if fallbackRefresh != "" && (status == http.StatusUnauthorized || value.Error == "invalid_grant" || value.Error == "access_denied" || value.Error == "expired_token") {
			return Token{}, ErrReauthorizationRequired
		}
		switch value.Error {
		case "authorization_pending":
			return Token{}, ErrAuthorizationPending
		case "slow_down":
			return Token{}, ErrSlowDown
		case "access_denied", "expired_token":
			return Token{}, ErrAuthorizationDenied
		default:
			return Token{}, fmt.Errorf("xAI OAuth HTTP %d (%s): %s", status, value.Error, value.ErrorDescription)
		}
	}
	if value.AccessToken == "" {
		return Token{}, fmt.Errorf("xAI OAuth 响应缺少 access_token")
	}
	if value.ExpiresIn <= 0 {
		value.ExpiresIn = 3600
	}
	if value.RefreshToken == "" {
		value.RefreshToken = fallbackRefresh
	}
	return Token{AccessToken: value.AccessToken, RefreshToken: value.RefreshToken, IDToken: value.IDToken, ExpiresAt: time.Now().UTC().Add(time.Duration(value.ExpiresIn) * time.Second)}, nil
}

func (c *OAuthClient) postForm(ctx context.Context, endpoint string, form url.Values) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxOAuthResponseBytes+1))
	if err != nil {
		return nil, resp.StatusCode, err
	}
	if len(body) > maxOAuthResponseBytes {
		return nil, resp.StatusCode, fmt.Errorf("xAI OAuth 响应过大")
	}
	return body, resp.StatusCode, nil
}

func diagnostic(body []byte) string {
	value := strings.TrimSpace(string(body))
	if len(value) > 256 {
		value = value[:256]
	}
	if value == "" {
		return strconv.Itoa(http.StatusBadGateway)
	}
	return value
}
