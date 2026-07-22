package config

import (
	"crypto/rand"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"
)

const (
	AuthModeNone   = ""
	AuthModeAPIKey = "api_key"
	AuthModeOAuth  = "oauth"

	// LocalKeyLength is the default generated local proxy key size.
	LocalKeyLength = 16
)

// localKeyAlphabet is URL/header-safe and easy to copy.
const localKeyAlphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

type OAuth struct {
	AccessToken  string    `json:"accessToken,omitempty"`
	RefreshToken string    `json:"refreshToken,omitempty"`
	ExpiresAt    time.Time `json:"expiresAt,omitempty"`
}

type Config struct {
	ListenHost string `json:"listenHost"`
	ListenPort int    `json:"listenPort"`
	LocalKey   string `json:"localKey,omitempty"`
	AuthMode   string `json:"authMode,omitempty"`
	APIKey     string `json:"apiKey,omitempty"`
	OAuth      OAuth  `json:"oauth,omitempty"`
}

type PublicConfig struct {
	ListenHost    string `json:"listenHost"`
	ListenPort    int    `json:"listenPort"`
	AuthMode      string `json:"authMode"`
	HasCredential bool   `json:"hasCredential"`
	HasAPIKey     bool   `json:"hasApiKey"`
	HasOAuth      bool   `json:"hasOAuth"`
	HasLocalKey   bool   `json:"hasLocalKey"`
	APIKeyHint    string `json:"apiKeyHint,omitempty"`
	LocalKey      string `json:"localKey,omitempty"`
	LocalKeyHint  string `json:"localKeyHint,omitempty"`
	OAuthExpires  string `json:"oauthExpires,omitempty"`
}

func Default() Config {
	return Config{
		ListenHost: "127.0.0.1",
		ListenPort: 8181,
		LocalKey:   GenerateLocalKey(LocalKeyLength),
	}
}

// GenerateLocalKey returns a cryptographically random key of the given length.
func GenerateLocalKey(length int) string {
	if length <= 0 {
		length = LocalKeyLength
	}
	buf := make([]byte, length)
	if _, err := rand.Read(buf); err != nil {
		// Extremely unlikely; fall back to a time-based non-empty key.
		return fmt.Sprintf("%016x", time.Now().UnixNano())[:length]
	}
	out := make([]byte, length)
	for i, b := range buf {
		out[i] = localKeyAlphabet[int(b)%len(localKeyAlphabet)]
	}
	return string(out)
}

// EnsureLocalKey fills an empty local key and reports whether it changed.
func EnsureLocalKey(cfg Config) (Config, bool) {
	if strings.TrimSpace(cfg.LocalKey) != "" {
		return cfg, false
	}
	cfg.LocalKey = GenerateLocalKey(LocalKeyLength)
	return cfg, true
}

func Validate(cfg Config) error {
	host := strings.TrimSpace(cfg.ListenHost)
	if host == "" {
		return errors.New("listen address must not be empty")
	}
	if ip := net.ParseIP(strings.Trim(host, "[]")); ip == nil && !strings.EqualFold(host, "localhost") {
		return fmt.Errorf("invalid listen address %q", cfg.ListenHost)
	}
	if cfg.ListenPort < 1 || cfg.ListenPort > 65535 {
		return errors.New("listen port must be between 1 and 65535")
	}
	if strings.TrimSpace(cfg.LocalKey) == "" {
		return errors.New("local proxy key must not be empty")
	}
	switch cfg.AuthMode {
	case AuthModeNone:
		return nil
	case AuthModeAPIKey:
		if strings.TrimSpace(cfg.APIKey) == "" {
			return errors.New("API key mode requires xAI API key")
		}
	case AuthModeOAuth:
		if strings.TrimSpace(cfg.OAuth.RefreshToken) == "" {
			return errors.New("OAuth mode missing refresh token; reauthorization required")
		}
	default:
		return fmt.Errorf("unsupported auth mode %q", cfg.AuthMode)
	}
	return nil
}

func isLoopback(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(strings.Trim(host, "[]"))
	return ip != nil && ip.IsLoopback()
}

func (cfg Config) Address() string {
	return net.JoinHostPort(strings.TrimSpace(cfg.ListenHost), fmt.Sprint(cfg.ListenPort))
}

func (cfg Config) Public() PublicConfig {
	hasAPIKey := strings.TrimSpace(cfg.APIKey) != ""
	hasOAuth := strings.TrimSpace(cfg.OAuth.RefreshToken) != ""
	oauthExpires := ""
	if !cfg.OAuth.ExpiresAt.IsZero() {
		oauthExpires = cfg.OAuth.ExpiresAt.UTC().Format(time.RFC3339)
	}
	localKey := strings.TrimSpace(cfg.LocalKey)
	return PublicConfig{
		ListenHost: cfg.ListenHost, ListenPort: cfg.ListenPort, AuthMode: cfg.AuthMode,
		HasCredential: (cfg.AuthMode == AuthModeAPIKey && hasAPIKey) || (cfg.AuthMode == AuthModeOAuth && hasOAuth),
		HasAPIKey:     hasAPIKey, HasOAuth: hasOAuth, HasLocalKey: localKey != "",
		APIKeyHint: mask(cfg.APIKey), LocalKey: localKey, LocalKeyHint: mask(cfg.LocalKey), OAuthExpires: oauthExpires,
	}
}

func mask(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if len(value) <= 8 {
		return "••••"
	}
	return value[:4] + "••••" + value[len(value)-4:]
}
