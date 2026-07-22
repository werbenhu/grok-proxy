package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/werbenhu/grok-proxy/internal/auth"
	"github.com/werbenhu/grok-proxy/internal/config"
	"github.com/werbenhu/grok-proxy/internal/proxy"
	"github.com/werbenhu/grok-proxy/internal/upstream"
)

type OAuthClient interface {
	auth.TokenRefresher
	Start(context.Context) (auth.DeviceAuthorization, error)
	Poll(context.Context, string) (auth.Token, error)
}

type handlerSlot struct{ value atomic.Value }

func newHandlerSlot(handler http.Handler) *handlerSlot {
	slot := &handlerSlot{}
	slot.value.Store(handler)
	return slot
}
func (s *handlerSlot) Store(handler http.Handler) { s.value.Store(handler) }
func (s *handlerSlot) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.value.Load().(http.Handler).ServeHTTP(w, r)
}

type Service struct {
	mu        sync.Mutex
	store     *config.Store
	oauth     OAuthClient
	tokens    *auth.Source
	upstream  *upstream.Client
	server    *http.Server
	listener  net.Listener
	slot      *handlerSlot
	handler   *proxy.Server
	status    string
	lastError string
}

func New(store *config.Store, oauthClient OAuthClient, httpClient *http.Client) (*Service, error) {
	if store == nil {
		return nil, errors.New("config store must not be nil")
	}
	if oauthClient == nil {
		oauthClient = auth.NewOAuthClient(httpClient)
	}
	// A stored credential survives restarts, so the initial status mirrors a
	// manual stop: stopped when authorized, waiting when nothing is configured.
	status := StatusWaiting
	if store.Current().Public().HasCredential {
		status = StatusStopped
	}
	service := &Service{store: store, oauth: oauthClient, status: status}
	service.tokens = auth.NewSource(oauthClient, store)
	service.upstream = upstream.NewClient(httpClient, service)
	return service, nil
}

func (s *Service) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.server != nil {
		return nil
	}
	cfg := s.store.Current()
	if !cfg.Public().HasCredential {
		s.status = StatusWaiting
		s.lastError = ""
		return nil
	}
	listener, err := net.Listen("tcp", cfg.Address())
	if err != nil {
		s.status = StatusError
		s.lastError = err.Error()
		return fmt.Errorf("listen %s: %w", cfg.Address(), err)
	}
	s.startLocked(cfg, listener)
	return nil
}

func (s *Service) startLocked(cfg config.Config, listener net.Listener) {
	handler := proxy.New(cfg, s.upstream)
	slot := newHandlerSlot(handler)
	server := &http.Server{
		Handler: slot, ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 2 * time.Minute, MaxHeaderBytes: 1 << 20,
	}
	s.server, s.listener, s.slot, s.handler = server, listener, slot, handler
	s.status, s.lastError = StatusRunning, ""
	go func() { _ = server.Serve(listener) }()
}

func (s *Service) Stop(ctx context.Context) error {
	s.mu.Lock()
	server := s.server
	s.server, s.listener, s.slot, s.handler = nil, nil, nil, nil
	if s.store.Current().Public().HasCredential {
		s.status = StatusStopped
	} else {
		s.status = StatusWaiting
	}
	s.mu.Unlock()
	if server == nil {
		return nil
	}
	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		_ = server.Close()
		return err
	}
	return nil
}

func (s *Service) Restart(ctx context.Context) error {
	if err := s.Stop(ctx); err != nil {
		return err
	}
	return s.Start(ctx)
}

func (s *Service) Save(ctx context.Context, input Settings) (State, error) {
	s.mu.Lock()
	old := s.store.Current()
	candidate := applySettings(old, input)
	if err := config.Validate(candidate); err != nil {
		s.mu.Unlock()
		return s.State(), err
	}
	running := s.server != nil
	if running && old.Address() == candidate.Address() {
		if err := s.store.Save(candidate); err != nil {
			s.mu.Unlock()
			return s.State(), err
		}
		handler := proxy.New(candidate, s.upstream)
		s.slot.Store(handler)
		s.handler = handler
		s.status, s.lastError = StatusRunning, ""
		s.mu.Unlock()
		return s.State(), nil
	}
	// Proxy is running and the listen address changed: rebind in place.
	// Keep the old listener if the new port/host cannot be bound.
	if running && candidate.Public().HasCredential {
		oldServer := s.server
		if s.listener != nil {
			_ = s.listener.Close()
		}
		s.server, s.listener, s.slot, s.handler = nil, nil, nil, nil
		listener, listenErr := net.Listen("tcp", candidate.Address())
		if listenErr != nil {
			rollbackErr := s.restoreLocked(old)
			if rollbackErr != nil {
				s.status = StatusError
				s.lastError = fmt.Sprintf("listen %s failed; rollback to %s also failed: %v", candidate.Address(), old.Address(), rollbackErr)
			} else {
				s.lastError = listenErr.Error()
			}
			s.mu.Unlock()
			shutdownServer(ctx, oldServer)
			return s.State(), fmt.Errorf("listen %s: %w", candidate.Address(), listenErr)
		}
		if err := s.store.Save(candidate); err != nil {
			_ = listener.Close()
			rollbackErr := s.restoreLocked(old)
			if rollbackErr != nil {
				s.status = StatusError
				s.lastError = fmt.Sprintf("save config failed; rollback to %s also failed: %v", old.Address(), rollbackErr)
			}
			s.mu.Unlock()
			shutdownServer(ctx, oldServer)
			return s.State(), err
		}
		s.startLocked(candidate, listener)
		s.mu.Unlock()
		shutdownServer(ctx, oldServer)
		return s.State(), nil
	}

	// Not running: save only; never auto-start.
	if err := s.store.Save(candidate); err != nil {
		s.mu.Unlock()
		return s.State(), err
	}
	if candidate.Public().HasCredential {
		s.status = StatusStopped
	} else {
		s.status = StatusWaiting
	}
	s.lastError = ""
	s.mu.Unlock()
	return s.State(), nil
}

func (s *Service) restoreLocked(cfg config.Config) error {
	listener, err := net.Listen("tcp", cfg.Address())
	if err != nil {
		return err
	}
	s.startLocked(cfg, listener)
	return nil
}

func shutdownServer(ctx context.Context, server *http.Server) {
	if server == nil {
		return
	}
	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	_ = server.Shutdown(shutdownCtx)
	cancel()
}

func applySettings(current config.Config, input Settings) config.Config {
	next := current
	if strings.TrimSpace(input.ListenHost) != "" {
		next.ListenHost = strings.TrimSpace(input.ListenHost)
	}
	if input.ListenPort != 0 {
		next.ListenPort = input.ListenPort
	}
	if input.AuthMode != "" {
		next.AuthMode = input.AuthMode
	}
	if input.APIKey != "" {
		next.APIKey = strings.TrimSpace(input.APIKey)
	}
	if input.LocalKey != "" {
		next.LocalKey = strings.TrimSpace(input.LocalKey)
	}
	// Local proxy key is required; ignore clear requests and keep the current key
	// when the field is left blank so existing configs stay usable.
	if next.AuthMode == config.AuthModeAPIKey && input.APIKey != "" {
		next.OAuth = config.OAuth{}
	}
	if next.AuthMode == config.AuthModeOAuth {
		next.APIKey = ""
	}
	return next
}

func (s *Service) State() State {
	s.mu.Lock()
	defer s.mu.Unlock()
	cfg := s.store.Current()
	address, openAI, anthropic := endpointState(cfg)
	state := State{Config: cfg.Public(), Running: s.server != nil, Status: s.status, Address: address, OpenAIBaseURL: openAI, AnthropicBaseURL: anthropic, LastError: s.lastError}
	if s.handler != nil {
		state.Stats = s.handler.Stats()
	}
	return state
}

func (s *Service) BeginOAuth(ctx context.Context) (auth.DeviceAuthorization, error) {
	return s.oauth.Start(ctx)
}

// RefreshAuth proactively refreshes a stale OAuth access token so an expired
// or revoked authorization surfaces as reauthorization_required at startup,
// instead of only when the first proxied request fails. API key mode needs no
// upfront validation and is left untouched.
func (s *Service) RefreshAuth(ctx context.Context) {
	cfg := s.store.Current()
	if cfg.AuthMode != config.AuthModeOAuth || strings.TrimSpace(cfg.OAuth.RefreshToken) == "" {
		return
	}
	_, _ = s.Authorization(ctx)
}

func (s *Service) CompleteOAuth(ctx context.Context, deviceCode string) (State, error) {
	token, err := s.oauth.Poll(ctx, deviceCode)
	if err != nil {
		return s.State(), err
	}
	if err := s.store.SaveOAuth(config.OAuth{AccessToken: token.AccessToken, RefreshToken: token.RefreshToken, ExpiresAt: token.ExpiresAt}); err != nil {
		return s.State(), err
	}
	s.mu.Lock()
	// Refresh the running proxy handler if already started; do not auto-start.
	if s.server != nil {
		cfg := s.store.Current()
		handler := proxy.New(cfg, s.upstream)
		s.slot.Store(handler)
		s.handler = handler
		s.status, s.lastError = StatusRunning, ""
	} else if s.store.Current().Public().HasCredential {
		s.status = StatusStopped
		s.lastError = ""
	}
	s.mu.Unlock()
	return s.State(), nil
}

func (s *Service) ClearCredential(ctx context.Context) (State, error) {
	if err := s.Stop(ctx); err != nil {
		return s.State(), err
	}
	cfg := s.store.Current()
	cfg.AuthMode = config.AuthModeNone
	cfg.APIKey = ""
	cfg.OAuth = config.OAuth{}
	if err := s.store.Save(cfg); err != nil {
		return s.State(), err
	}
	s.mu.Lock()
	s.status = StatusWaiting
	s.lastError = ""
	s.mu.Unlock()
	return s.State(), nil
}

func (s *Service) TestConnection(ctx context.Context) (ConnectionTest, error) {
	started := time.Now()
	response, err := s.upstream.Models(ctx)
	latency := time.Since(started)
	if err != nil {
		return ConnectionTest{LatencyMS: latency.Milliseconds(), Message: err.Error()}, err
	}
	defer response.Body.Close()
	var payload struct {
		Data []json.RawMessage `json:"data"`
	}
	_ = json.NewDecoder(response.Body).Decode(&payload)
	return ConnectionTest{OK: true, LatencyMS: latency.Milliseconds(), Message: formatModelCount(len(payload.Data))}, nil
}

func (s *Service) Authorization(ctx context.Context) (upstream.Authorization, error) {
	cfg := s.store.Current()
	switch cfg.AuthMode {
	case config.AuthModeAPIKey:
		if strings.TrimSpace(cfg.APIKey) == "" {
			return upstream.Authorization{}, auth.ErrCredentialMissing
		}
		return upstream.Authorization{Mode: upstream.ModeAPIKey, Token: cfg.APIKey}, nil
	case config.AuthModeOAuth:
		token, err := s.tokens.AccessToken(ctx)
		if err != nil {
			if errors.Is(err, auth.ErrReauthorizationRequired) {
				s.mu.Lock()
				s.status, s.lastError = StatusReauthorization, auth.ErrReauthorizationRequired.Error()
				s.mu.Unlock()
			}
			return upstream.Authorization{}, err
		}
		return upstream.Authorization{Mode: upstream.ModeOAuth, Token: token}, nil
	default:
		return upstream.Authorization{}, auth.ErrCredentialMissing
	}
}

var _ upstream.CredentialSource = (*Service)(nil)
