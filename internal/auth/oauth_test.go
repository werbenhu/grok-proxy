package auth

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestStartDeviceAuthorization(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s", r.Method)
		}
		if err := r.ParseForm(); err != nil {
			t.Error(err)
		}
		if r.Form.Get("client_id") != defaultClientID || !strings.Contains(r.Form.Get("scope"), "grok-cli:access") {
			t.Errorf("form = %v", r.Form)
		}
		_, _ = io.WriteString(w, `{"device_code":"device","user_code":"ABCD-EFGH","verification_uri":"https://auth.x.ai/activate","verification_uri_complete":"https://auth.x.ai/activate?user_code=ABCD-EFGH","interval":0,"expires_in":0}`)
	}))
	defer server.Close()
	client := newTestOAuthClient(server.Client(), server.URL)
	got, err := client.Start(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.DeviceCode != "device" || got.UserCode != "ABCD-EFGH" || got.Interval != 5*time.Second || got.ExpiresIn != 30*time.Minute {
		t.Fatalf("authorization = %+v", got)
	}
}

func TestOAuthExchangeErrors(t *testing.T) {
	tests := []struct {
		name string
		body string
		want error
	}{
		{"pending", `{"error":"authorization_pending"}`, ErrAuthorizationPending},
		{"slow down", `{"error":"slow_down"}`, ErrSlowDown},
		{"denied", `{"error":"access_denied"}`, ErrAuthorizationDenied},
		{"expired", `{"error":"expired_token"}`, ErrAuthorizationDenied},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := oauthServer(t, http.StatusBadRequest, tt.body)
			defer server.Close()
			client := newTestOAuthClient(server.Client(), server.URL)
			_, err := client.Poll(context.Background(), "device")
			if !errors.Is(err, tt.want) {
				t.Fatalf("err = %v", err)
			}
		})
	}
}

func TestRefreshKeepsFallbackRefreshToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		values, _ := url.ParseQuery(string(body))
		if values.Get("grant_type") != "refresh_token" || values.Get("refresh_token") != "old-refresh" {
			t.Errorf("form = %v", values)
		}
		_, _ = io.WriteString(w, `{"access_token":"new-access","expires_in":3600}`)
	}))
	defer server.Close()
	client := newTestOAuthClient(server.Client(), server.URL)
	got, err := client.Refresh(context.Background(), "old-refresh")
	if err != nil {
		t.Fatal(err)
	}
	if got.AccessToken != "new-access" || got.RefreshToken != "old-refresh" || time.Until(got.ExpiresAt) < 59*time.Minute {
		t.Fatalf("token = %+v", got)
	}
}

func TestRefreshMapsPermanentRejectionToReauthorization(t *testing.T) {
	server := oauthServer(t, http.StatusBadRequest, `{"error":"invalid_grant"}`)
	defer server.Close()
	client := newTestOAuthClient(server.Client(), server.URL)
	_, err := client.Refresh(context.Background(), "revoked")
	if !errors.Is(err, ErrReauthorizationRequired) {
		t.Fatalf("err=%v", err)
	}
}

func TestOAuthRejectsMissingFieldsAndOversizedResponses(t *testing.T) {
	missing := oauthServer(t, http.StatusOK, `{}`)
	client := newTestOAuthClient(missing.Client(), missing.URL)
	if _, err := client.Start(context.Background()); err == nil {
		t.Fatal("expected missing fields error")
	}
	missing.Close()

	large := oauthServer(t, http.StatusOK, `{"access_token":"`+strings.Repeat("x", maxOAuthResponseBytes)+`"}`)
	client = newTestOAuthClient(large.Client(), large.URL)
	if _, err := client.Poll(context.Background(), "device"); err == nil || !strings.Contains(err.Error(), "过大") {
		t.Fatalf("err = %v", err)
	}
	large.Close()
}

func oauthServer(t *testing.T, status int, body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}))
}

func newTestOAuthClient(httpClient *http.Client, endpoint string) *OAuthClient {
	client := NewOAuthClient(httpClient)
	client.deviceURL = endpoint
	client.tokenURL = endpoint
	return client
}
