package config

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestDefaultIsValidAndLoopbackOnly(t *testing.T) {
	cfg := Default()
	if cfg.ListenHost != "127.0.0.1" || cfg.ListenPort != 8181 {
		t.Fatalf("default address = %s:%d", cfg.ListenHost, cfg.ListenPort)
	}
	if err := Validate(cfg); err != nil {
		t.Fatalf("default config invalid: %v", err)
	}
}

func TestValidateRequiresKeyOutsideLoopback(t *testing.T) {
	cfg := Default()
	cfg.ListenHost = "0.0.0.0"
	if err := Validate(cfg); err == nil || !strings.Contains(err.Error(), "本地代理密钥") {
		t.Fatalf("expected local key error, got %v", err)
	}
	cfg.LocalKey = "shared-secret"
	if err := Validate(cfg); err != nil {
		t.Fatalf("non-loopback with key: %v", err)
	}
}

func TestValidateRejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name string
		edit func(*Config)
	}{
		{"empty host", func(c *Config) { c.ListenHost = "" }},
		{"bad port", func(c *Config) { c.ListenPort = 70000 }},
		{"bad auth mode", func(c *Config) { c.AuthMode = "password" }},
		{"api key mode missing key", func(c *Config) { c.AuthMode = AuthModeAPIKey }},
		{"oauth mode missing refresh", func(c *Config) {
			c.AuthMode = AuthModeOAuth
			c.OAuth.AccessToken = "access"
			c.OAuth.ExpiresAt = time.Now()
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Default()
			tt.edit(&cfg)
			if err := Validate(cfg); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestPublicNeverExposesSecrets(t *testing.T) {
	cfg := Default()
	cfg.AuthMode = AuthModeAPIKey
	cfg.APIKey = "xai-secret-value"
	cfg.LocalKey = "local-secret-value"
	cfg.OAuth = OAuth{AccessToken: "access-secret", RefreshToken: "refresh-secret", ExpiresAt: time.Now().UTC()}
	encoded, err := json.Marshal(cfg.Public())
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"xai-secret-value", "local-secret-value", "access-secret", "refresh-secret"} {
		if bytes.Contains(encoded, []byte(secret)) {
			t.Fatalf("secret %q leaked: %s", secret, encoded)
		}
	}
	public := cfg.Public()
	if !public.HasCredential || !public.HasAPIKey || !public.HasLocalKey {
		t.Fatalf("public flags = %+v", public)
	}
	if public.APIKeyHint != "xai-••••alue" {
		t.Fatalf("hint = %q", public.APIKeyHint)
	}
}
