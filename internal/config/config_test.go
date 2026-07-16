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
	if len(cfg.LocalKey) != LocalKeyLength {
		t.Fatalf("default local key length = %d, want %d", len(cfg.LocalKey), LocalKeyLength)
	}
	if err := Validate(cfg); err != nil {
		t.Fatalf("default config invalid: %v", err)
	}
}

func TestValidateRequiresLocalKey(t *testing.T) {
	cfg := Default()
	cfg.LocalKey = ""
	if err := Validate(cfg); err == nil || !strings.Contains(err.Error(), "本地代理密钥不能为空") {
		t.Fatalf("expected empty local key error, got %v", err)
	}
	cfg.LocalKey = "shared-secret"
	cfg.ListenHost = "0.0.0.0"
	if err := Validate(cfg); err != nil {
		t.Fatalf("non-loopback with key: %v", err)
	}
}

func TestGenerateLocalKeyLengthAndCharset(t *testing.T) {
	key := GenerateLocalKey(LocalKeyLength)
	if len(key) != LocalKeyLength {
		t.Fatalf("len = %d", len(key))
	}
	for _, r := range key {
		if !strings.ContainsRune(localKeyAlphabet, r) {
			t.Fatalf("unexpected rune %q in %q", r, key)
		}
	}
	other := GenerateLocalKey(LocalKeyLength)
	if key == other {
		t.Fatal("expected different random keys")
	}
}

func TestEnsureLocalKeyFillsEmpty(t *testing.T) {
	cfg := Config{ListenHost: "127.0.0.1", ListenPort: 8181}
	next, filled := EnsureLocalKey(cfg)
	if !filled || len(next.LocalKey) != LocalKeyLength {
		t.Fatalf("filled=%v key=%q", filled, next.LocalKey)
	}
	same, filledAgain := EnsureLocalKey(next)
	if filledAgain || same.LocalKey != next.LocalKey {
		t.Fatalf("should keep existing key: filled=%v key=%q", filledAgain, same.LocalKey)
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

func TestPublicNeverExposesUpstreamSecrets(t *testing.T) {
	cfg := Default()
	cfg.AuthMode = AuthModeAPIKey
	cfg.APIKey = "xai-secret-value"
	cfg.LocalKey = "local-secret-value"
	cfg.OAuth = OAuth{AccessToken: "access-secret", RefreshToken: "refresh-secret", ExpiresAt: time.Now().UTC()}
	encoded, err := json.Marshal(cfg.Public())
	if err != nil {
		t.Fatal(err)
	}
	// Local proxy key is intentionally returned for desktop UI/snippets.
	for _, secret := range []string{"xai-secret-value", "access-secret", "refresh-secret"} {
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
	if public.LocalKey != "local-secret-value" {
		t.Fatalf("local key = %q", public.LocalKey)
	}
}
