package config

import (
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
)

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
	LocalKeyHint  string `json:"localKeyHint,omitempty"`
	OAuthExpires  string `json:"oauthExpires,omitempty"`
}

func Default() Config {
	return Config{ListenHost: "127.0.0.1", ListenPort: 8181}
}

func Validate(cfg Config) error {
	host := strings.TrimSpace(cfg.ListenHost)
	if host == "" {
		return errors.New("监听地址不能为空")
	}
	if ip := net.ParseIP(strings.Trim(host, "[]")); ip == nil && !strings.EqualFold(host, "localhost") {
		return fmt.Errorf("监听地址 %q 无效", cfg.ListenHost)
	}
	if cfg.ListenPort < 1 || cfg.ListenPort > 65535 {
		return errors.New("监听端口必须在 1 到 65535 之间")
	}
	if !isLoopback(host) && strings.TrimSpace(cfg.LocalKey) == "" {
		return errors.New("非回环监听必须设置本地代理密钥")
	}
	switch cfg.AuthMode {
	case AuthModeNone:
		return nil
	case AuthModeAPIKey:
		if strings.TrimSpace(cfg.APIKey) == "" {
			return errors.New("API Key 模式必须设置 xAI API Key")
		}
	case AuthModeOAuth:
		if strings.TrimSpace(cfg.OAuth.RefreshToken) == "" {
			return errors.New("OAuth 模式缺少 refresh token，需要重新授权")
		}
	default:
		return fmt.Errorf("不支持的授权模式 %q", cfg.AuthMode)
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
	return PublicConfig{
		ListenHost: cfg.ListenHost, ListenPort: cfg.ListenPort, AuthMode: cfg.AuthMode,
		HasCredential: (cfg.AuthMode == AuthModeAPIKey && hasAPIKey) || (cfg.AuthMode == AuthModeOAuth && hasOAuth),
		HasAPIKey:     hasAPIKey, HasOAuth: hasOAuth, HasLocalKey: strings.TrimSpace(cfg.LocalKey) != "",
		APIKeyHint: mask(cfg.APIKey), LocalKeyHint: mask(cfg.LocalKey), OAuthExpires: oauthExpires,
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
