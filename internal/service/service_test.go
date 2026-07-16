package service

import (
	"context"
	"errors"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/werbenhu/grok-proxy/internal/auth"
	"github.com/werbenhu/grok-proxy/internal/config"
)

type fakeOAuth struct {
	start      auth.DeviceAuthorization
	token      auth.Token
	pollErr    error
	refreshErr error
}

func (f *fakeOAuth) Start(context.Context) (auth.DeviceAuthorization, error) { return f.start, nil }
func (f *fakeOAuth) Poll(context.Context, string) (auth.Token, error)        { return f.token, f.pollErr }
func (f *fakeOAuth) Refresh(context.Context, string) (auth.Token, error) {
	return f.token, f.refreshErr
}

func TestStartWithoutCredentialWaitsForConfiguration(t *testing.T) {
	service := newTestService(t, config.Default(), &fakeOAuth{})
	if err := service.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	state := service.State()
	if state.Running || state.Status != StatusWaiting {
		t.Fatalf("state=%+v", state)
	}
}

func TestStartAndStopAreIdempotent(t *testing.T) {
	cfg := config.Default()
	cfg.ListenPort = freePort(t)
	cfg.AuthMode = config.AuthModeAPIKey
	cfg.APIKey = "xai-key"
	service := newTestService(t, cfg, &fakeOAuth{})
	if err := service.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := service.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !service.State().Running {
		t.Fatal("not running")
	}
	if err := service.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := service.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	if service.State().Running {
		t.Fatal("still running")
	}
}

func TestSavePortConflictKeepsOldConfigAndListener(t *testing.T) {
	old := config.Default()
	old.ListenPort = freePort(t)
	old.AuthMode = config.AuthModeAPIKey
	old.APIKey = "xai-key"
	service := newTestService(t, old, &fakeOAuth{})
	if err := service.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = service.Stop(context.Background()) })
	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer occupied.Close()
	port := occupied.Addr().(*net.TCPAddr).Port
	_, err = service.Save(context.Background(), Settings{ListenHost: "127.0.0.1", ListenPort: port, AuthMode: config.AuthModeAPIKey})
	if err == nil {
		t.Fatal("expected port conflict")
	}
	state := service.State()
	if !state.Running || state.Config.ListenPort != old.ListenPort {
		t.Fatalf("state=%+v", state)
	}
	resp, err := http.Get("http://" + state.Address + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("health=%d", resp.StatusCode)
	}
}

func TestSaveCanRebindSamePortFromLoopbackAlias(t *testing.T) {
	old := config.Default()
	old.ListenPort = freePort(t)
	old.AuthMode = config.AuthModeAPIKey
	old.APIKey = "xai-key"
	service := newTestService(t, old, &fakeOAuth{})
	if err := service.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = service.Stop(context.Background()) })
	state, err := service.Save(context.Background(), Settings{ListenHost: "localhost", ListenPort: old.ListenPort, AuthMode: config.AuthModeAPIKey})
	if err != nil {
		t.Fatal(err)
	}
	if !state.Running || state.Config.ListenHost != "localhost" {
		t.Fatalf("state=%+v", state)
	}
	resp, err := http.Get("http://" + state.Address + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("health=%d", resp.StatusCode)
	}
}

func TestCompleteOAuthPersistsCredentialAndStartsProxy(t *testing.T) {
	cfg := config.Default()
	cfg.ListenPort = freePort(t)
	oauthClient := &fakeOAuth{token: auth.Token{AccessToken: "access", RefreshToken: "refresh", ExpiresAt: time.Now().Add(time.Hour)}}
	service := newTestService(t, cfg, oauthClient)
	state, err := service.CompleteOAuth(context.Background(), "device")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = service.Stop(context.Background()) })
	if !state.Running || !state.Config.HasOAuth || state.Config.AuthMode != config.AuthModeOAuth {
		t.Fatalf("state=%+v", state)
	}
	stored := service.store.Current()
	if stored.OAuth.RefreshToken != "refresh" || stored.APIKey != "" {
		t.Fatalf("stored=%+v", stored)
	}
}

func TestCompleteOAuthKeepsPendingTypedError(t *testing.T) {
	service := newTestService(t, config.Default(), &fakeOAuth{pollErr: auth.ErrAuthorizationPending})
	_, err := service.CompleteOAuth(context.Background(), "device")
	if !errors.Is(err, auth.ErrAuthorizationPending) {
		t.Fatalf("err=%v", err)
	}
}

func TestPermanentOAuthFailureRequiresReauthorization(t *testing.T) {
	cfg := config.Default()
	cfg.AuthMode = config.AuthModeOAuth
	cfg.OAuth = config.OAuth{AccessToken: "expired", RefreshToken: "revoked", ExpiresAt: time.Now().Add(-time.Hour)}
	service := newTestService(t, cfg, &fakeOAuth{refreshErr: auth.ErrReauthorizationRequired})
	_, err := service.Authorization(context.Background())
	if !errors.Is(err, auth.ErrReauthorizationRequired) {
		t.Fatalf("err=%v", err)
	}
	state := service.State()
	if state.Status != StatusReauthorization || state.Config.HasCredential || !strings.Contains(state.LastError, "重新授权") {
		t.Fatalf("state=%+v", state)
	}
}

func newTestService(t *testing.T, cfg config.Config, oauthClient OAuthClient) *Service {
	t.Helper()
	store := config.NewStore(filepath.Join(t.TempDir(), "config.json"))
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	service, err := New(store, oauthClient, nil)
	if err != nil {
		t.Fatal(err)
	}
	return service
}

func freePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()
	return port
}
