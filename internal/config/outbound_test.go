package config

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestOutboundProxyValidate(t *testing.T) {
	tests := []struct {
		name    string
		proxy   OutboundProxy
		wantErr string
	}{
		{"empty is system", OutboundProxy{}, ""},
		{"system", OutboundProxy{Mode: OutboundProxySystem}, ""},
		{"direct", OutboundProxy{Mode: OutboundProxyDirect}, ""},
		{"http ok", OutboundProxy{Mode: OutboundProxyHTTP, Host: "127.0.0.1", Port: 7890}, ""},
		{"https ok", OutboundProxy{Mode: OutboundProxyHTTPS, Host: "proxy.example", Port: 443}, ""},
		{"socks alias", OutboundProxy{Mode: "socks", Host: "127.0.0.1", Port: 1080}, ""},
		{"missing host", OutboundProxy{Mode: OutboundProxyHTTP, Port: 8080}, "host"},
		{"host includes port", OutboundProxy{Mode: OutboundProxyHTTP, Host: "127.0.0.1:7890", Port: 8080}, "invalid outbound proxy host"},
		{"bad host", OutboundProxy{Mode: OutboundProxyHTTP, Host: "http://bad", Port: 8080}, "invalid outbound proxy host"},
		{"bad port", OutboundProxy{Mode: OutboundProxySOCKS5, Host: "127.0.0.1", Port: 0}, "port"},
		{"unknown mode", OutboundProxy{Mode: "ftp"}, "unsupported"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.proxy.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected err: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("err=%v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestOutboundProxyURL(t *testing.T) {
	got, err := (OutboundProxy{Mode: OutboundProxyHTTP, Host: "127.0.0.1", Port: 7890, Username: "u", Password: "p@ss"}).URL()
	if err != nil {
		t.Fatal(err)
	}
	if got.String() != "http://u:p%40ss@127.0.0.1:7890" {
		t.Fatalf("url = %s", got)
	}
	socks, err := (OutboundProxy{Mode: OutboundProxySOCKS5, Host: "10.0.0.1", Port: 1080}).URL()
	if err != nil {
		t.Fatal(err)
	}
	if socks.Scheme != "socks5h" || socks.Host != "10.0.0.1:1080" {
		t.Fatalf("socks = %s", socks)
	}
	direct, err := (OutboundProxy{Mode: OutboundProxyDirect}).URL()
	if err != nil || direct != nil {
		t.Fatalf("direct = %v err=%v", direct, err)
	}
}

func TestOutboundProxyPublicHidesPassword(t *testing.T) {
	public := (OutboundProxy{Mode: OutboundProxyHTTP, Host: "127.0.0.1", Port: 7890, Username: "user", Password: "secret"}).Public()
	if public.Mode != OutboundProxyHTTP || public.Username != "user" || !public.HasPassword || public.Host != "127.0.0.1" {
		t.Fatalf("public = %+v", public)
	}
}

func TestOutboundResolverSwitchesMode(t *testing.T) {
	resolver := NewOutbound(OutboundProxy{Mode: OutboundProxyDirect})
	req, err := http.NewRequest(http.MethodGet, "https://api.x.ai/v1/models", nil)
	if err != nil {
		t.Fatal(err)
	}
	got, err := resolver.Proxy(req)
	if err != nil || got != nil {
		t.Fatalf("direct proxy = %v err=%v", got, err)
	}
	resolver.Set(OutboundProxy{Mode: OutboundProxyHTTP, Host: "127.0.0.1", Port: 7890})
	got, err = resolver.Proxy(req)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.String() != "http://127.0.0.1:7890" {
		t.Fatalf("http proxy = %v", got)
	}
}

func TestValidateRejectsBadOutboundProxy(t *testing.T) {
	cfg := Default()
	cfg.OutboundProxy = OutboundProxy{Mode: OutboundProxyHTTP, Port: 7890}
	if err := Validate(cfg); err == nil || !strings.Contains(err.Error(), "host") {
		t.Fatalf("err=%v", err)
	}
}

func TestDefaultPublicOutboundProxyIsSystem(t *testing.T) {
	public := Default().Public()
	if public.OutboundProxy.Mode != OutboundProxySystem {
		t.Fatalf("mode = %q", public.OutboundProxy.Mode)
	}
}

func TestSystemModeUsesProxyFromEnvironment(t *testing.T) {
	got := fmt.Sprintf("%p", (OutboundProxy{}).Proxy())
	want := fmt.Sprintf("%p", http.ProxyFromEnvironment)
	if got != want {
		t.Fatalf("system proxy func = %s, want %s", got, want)
	}
}
