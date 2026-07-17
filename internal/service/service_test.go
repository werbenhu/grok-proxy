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

func TestCompleteOAuthPersistsCredentialWithoutStartingProxy(t *testing.T) {
	cfg := config.Default()
	cfg.ListenPort = freePort(t)
	oauthClient := &fakeOAuth{token: auth.Token{AccessToken: "access", RefreshToken: "refresh", ExpiresAt: time.Now().Add(time.Hour)}}
	service := newTestService(t, cfg, oauthClient)
	state, err := service.CompleteOAuth(context.Background(), "device")
	if err != nil {
		t.Fatal(err)
	}
	if state.Running || !state.Config.HasOAuth || state.Config.AuthMode != config.AuthModeOAuth {
		t.Fatalf("state=%+v", state)
	}
	if state.Status != StatusStopped {
		t.Fatalf("status=%s, want stopped until manual start", state.Status)
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

func TestNewInitialStatusReflectsStoredCredential(t *testing.T) {
	if service := newTestService(t, config.Default(), &fakeOAuth{}); service.State().Status != StatusWaiting {
		t.Fatalf("status=%s, want waiting without credential", service.State().Status)
	}
	cfg := config.Default()
	cfg.AuthMode = config.AuthModeAPIKey
	cfg.APIKey = "xai-key"
	if service := newTestService(t, cfg, &fakeOAuth{}); service.State().Status != StatusStopped {
		t.Fatalf("status=%s, want stopped with stored credential", service.State().Status)
	}
}

func TestRefreshAuthRefreshesStaleOAuthToken(t *testing.T) {
	cfg := config.Default()
	cfg.AuthMode = config.AuthModeOAuth
	cfg.OAuth = config.OAuth{AccessToken: "stale", RefreshToken: "refresh", ExpiresAt: time.Now().Add(-time.Hour)}
	oauthClient := &fakeOAuth{token: auth.Token{AccessToken: "fresh", RefreshToken: "refresh2", ExpiresAt: time.Now().Add(time.Hour)}}
	service := newTestService(t, cfg, oauthClient)
	service.RefreshAuth(context.Background())
	stored := service.store.Current()
	if stored.OAuth.AccessToken != "fresh" || stored.OAuth.RefreshToken != "refresh2" {
		t.Fatalf("stored=%+v", stored.OAuth)
	}
	if state := service.State(); state.Status != StatusStopped || !state.Config.HasCredential {
		t.Fatalf("state=%+v", state)
	}
}

func TestRefreshAuthMarksReauthorizationWhenRefreshFails(t *testing.T) {
	cfg := config.Default()
	cfg.AuthMode = config.AuthModeOAuth
	cfg.OAuth = config.OAuth{AccessToken: "stale", RefreshToken: "revoked", ExpiresAt: time.Now().Add(-time.Hour)}
	service := newTestService(t, cfg, &fakeOAuth{refreshErr: auth.ErrReauthorizationRequired})
	service.RefreshAuth(context.Background())
	state := service.State()
	if state.Status != StatusReauthorization || state.Config.HasCredential {
		t.Fatalf("state=%+v", state)
	}
}

func TestRefreshAuthLeavesFreshOrNonOAuthCredentialAlone(t *testing.T) {
	cfg := config.Default()
	cfg.AuthMode = config.AuthModeOAuth
	cfg.OAuth = config.OAuth{AccessToken: "fresh", RefreshToken: "refresh", ExpiresAt: time.Now().Add(time.Hour)}
	service := newTestService(t, cfg, &fakeOAuth{refreshErr: errors.New("must not be called")})
	service.RefreshAuth(context.Background())
	if state := service.State(); state.Status != StatusStopped || !state.Config.HasCredential {
		t.Fatalf("state=%+v", state)
	}

	apiKeyCfg := config.Default()
	apiKeyCfg.AuthMode = config.AuthModeAPIKey
	apiKeyCfg.APIKey = "xai-key"
	apiKeyService := newTestService(t, apiKeyCfg, &fakeOAuth{refreshErr: errors.New("must not be called")})
	apiKeyService.RefreshAuth(context.Background())
	if state := apiKeyService.State(); state.Status != StatusStopped || !state.Config.HasCredential {
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
