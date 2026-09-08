package config

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	// OutboundProxySystem follows HTTP_PROXY / HTTPS_PROXY / ALL_PROXY.
	OutboundProxySystem = "system"
	// OutboundProxyDirect disables all outbound proxies, including env vars.
	OutboundProxyDirect = "direct"
	OutboundProxyHTTP   = "http"
	OutboundProxyHTTPS  = "https"
	OutboundProxySOCKS5 = "socks5"
)

// OutboundProxy is the user-configured proxy used when this process
// reaches xAI / OAuth on the public internet. It is independent of the
// local listen address that clients connect to.
type OutboundProxy struct {
	Mode     string `json:"mode,omitempty"`
	Host     string `json:"host,omitempty"`
	Port     int    `json:"port,omitempty"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

type PublicOutboundProxy struct {
	Mode        string `json:"mode"`
	Host        string `json:"host,omitempty"`
	Port        int    `json:"port,omitempty"`
	Username    string `json:"username,omitempty"`
	HasPassword bool   `json:"hasPassword"`
}

func (p OutboundProxy) normalizedMode() string {
	switch strings.ToLower(strings.TrimSpace(p.Mode)) {
	case "", OutboundProxySystem:
		return OutboundProxySystem
	case OutboundProxyDirect:
		return OutboundProxyDirect
	case OutboundProxyHTTP:
		return OutboundProxyHTTP
	case OutboundProxyHTTPS:
		return OutboundProxyHTTPS
	case OutboundProxySOCKS5, "socks", "socks5h":
		return OutboundProxySOCKS5
	default:
		return strings.ToLower(strings.TrimSpace(p.Mode))
	}
}

func (p OutboundProxy) explicit() bool {
	switch p.normalizedMode() {
	case OutboundProxyHTTP, OutboundProxyHTTPS, OutboundProxySOCKS5:
		return true
	default:
		return false
	}
}

func (p OutboundProxy) Validate() error {
	switch mode := p.normalizedMode(); mode {
	case OutboundProxySystem, OutboundProxyDirect:
		return nil
	case OutboundProxyHTTP, OutboundProxyHTTPS, OutboundProxySOCKS5:
		host := strings.TrimSpace(p.Host)
		if host == "" {
			return errors.New("outbound proxy host must not be empty")
		}
		if strings.ContainsAny(host, " /\\") || strings.Contains(host, "://") {
			return fmt.Errorf("invalid outbound proxy host %q", p.Host)
		}
		if strings.Contains(host, ":") && net.ParseIP(strings.Trim(host, "[]")) == nil {
			return fmt.Errorf("invalid outbound proxy host %q", p.Host)
		}
		if p.Port < 1 || p.Port > 65535 {
			return errors.New("outbound proxy port must be between 1 and 65535")
		}
		return nil
	default:
		return fmt.Errorf("unsupported outbound proxy mode %q", p.Mode)
	}
}

// URL returns the proxy URL for an explicit mode. System and direct
// modes return (nil, nil). SOCKS5 uses socks5h so the proxy resolves DNS.
func (p OutboundProxy) URL() (*url.URL, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	mode := p.normalizedMode()
	if !p.explicit() {
		return nil, nil
	}
	scheme := mode
	if mode == OutboundProxySOCKS5 {
		scheme = "socks5h"
	}
	value := &url.URL{
		Scheme: scheme,
		Host:   net.JoinHostPort(strings.TrimSpace(p.Host), strconv.Itoa(p.Port)),
	}
	user := strings.TrimSpace(p.Username)
	pass := p.Password
	if user != "" || pass != "" {
		value.User = url.UserPassword(user, pass)
	}
	return value, nil
}

// Proxy is an http.Transport.Proxy function for the current settings.
func (p OutboundProxy) Proxy() func(*http.Request) (*url.URL, error) {
	switch p.normalizedMode() {
	case OutboundProxyDirect:
		return func(*http.Request) (*url.URL, error) { return nil, nil }
	case OutboundProxyHTTP, OutboundProxyHTTPS, OutboundProxySOCKS5:
		return func(*http.Request) (*url.URL, error) { return p.URL() }
	default:
		return http.ProxyFromEnvironment
	}
}

func (p OutboundProxy) Public() PublicOutboundProxy {
	return PublicOutboundProxy{
		Mode:        p.normalizedMode(),
		Host:        strings.TrimSpace(p.Host),
		Port:        p.Port,
		Username:    strings.TrimSpace(p.Username),
		HasPassword: p.Password != "",
	}
}

// Outbound is a thread-safe proxy resolver shared by outbound HTTP clients.
type Outbound struct {
	mu    sync.RWMutex
	proxy OutboundProxy
}

func NewOutbound(proxy OutboundProxy) *Outbound {
	return &Outbound{proxy: proxy}
}

func (o *Outbound) Set(proxy OutboundProxy) {
	o.mu.Lock()
	o.proxy = proxy
	o.mu.Unlock()
}

func (o *Outbound) Current() OutboundProxy {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.proxy
}

func (o *Outbound) Proxy(req *http.Request) (*url.URL, error) {
	o.mu.RLock()
	proxy := o.proxy
	o.mu.RUnlock()
	return proxy.Proxy()(req)
}

func NewHTTPClient(resolver *Outbound) (*http.Client, *http.Transport) {
	if resolver == nil {
		resolver = NewOutbound(OutboundProxy{})
	}
	transport := &http.Transport{
		Proxy:                 resolver.Proxy,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          64,
		MaxIdleConnsPerHost:   32,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 60 * time.Second,
	}
	return &http.Client{Transport: transport}, transport
}
