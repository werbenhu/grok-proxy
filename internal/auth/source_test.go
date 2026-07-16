package auth

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/werbenhu/grok-proxy/internal/config"
)

type memoryTokenStore struct {
	mu            sync.Mutex
	value         config.OAuth
	saves         int
	invalidations int
}

func (s *memoryTokenStore) InvalidateOAuth() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.value = config.OAuth{}
	s.invalidations++
	return nil
}

func (s *memoryTokenStore) OAuth() config.OAuth { s.mu.Lock(); defer s.mu.Unlock(); return s.value }
func (s *memoryTokenStore) SaveOAuth(value config.OAuth) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.value = value
	s.saves++
	return nil
}

type refreshFunc func(context.Context, string) (Token, error)

func (f refreshFunc) Refresh(ctx context.Context, token string) (Token, error) { return f(ctx, token) }

func TestSourceReturnsFreshAccessTokenWithoutRefresh(t *testing.T) {
	store := &memoryTokenStore{value: config.OAuth{AccessToken: "fresh", RefreshToken: "refresh", ExpiresAt: time.Now().Add(time.Hour)}}
	source := NewSource(refreshFunc(func(context.Context, string) (Token, error) { t.Fatal("unexpected refresh"); return Token{}, nil }), store)
	got, err := source.AccessToken(context.Background())
	if err != nil || got != "fresh" {
		t.Fatalf("token=%q err=%v", got, err)
	}
}

func TestSourceConcurrentRefreshHappensOnce(t *testing.T) {
	store := &memoryTokenStore{value: config.OAuth{AccessToken: "expired", RefreshToken: "refresh", ExpiresAt: time.Now().Add(-time.Minute)}}
	var calls atomic.Int32
	source := NewSource(refreshFunc(func(context.Context, string) (Token, error) {
		calls.Add(1)
		time.Sleep(20 * time.Millisecond)
		return Token{AccessToken: "new", RefreshToken: "refresh-2", ExpiresAt: time.Now().Add(time.Hour)}, nil
	}), store)
	var wg sync.WaitGroup
	errs := make(chan error, 5)
	for range 5 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			token, err := source.AccessToken(context.Background())
			if err != nil {
				errs <- err
				return
			}
			if token != "new" {
				t.Errorf("token=%q", token)
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	if calls.Load() != 1 || store.saves != 1 {
		t.Fatalf("calls=%d saves=%d", calls.Load(), store.saves)
	}
}

func TestSourceRequiresRefreshToken(t *testing.T) {
	store := &memoryTokenStore{}
	source := NewSource(refreshFunc(func(context.Context, string) (Token, error) { return Token{}, nil }), store)
	if _, err := source.AccessToken(context.Background()); err == nil {
		t.Fatal("expected missing credential error")
	}
}

func TestSourceInvalidatesPermanentlyRejectedOAuth(t *testing.T) {
	store := &memoryTokenStore{value: config.OAuth{AccessToken: "expired", RefreshToken: "revoked", ExpiresAt: time.Now().Add(-time.Minute)}}
	source := NewSource(refreshFunc(func(context.Context, string) (Token, error) {
		return Token{}, ErrReauthorizationRequired
	}), store)
	_, err := source.AccessToken(context.Background())
	if !errors.Is(err, ErrReauthorizationRequired) {
		t.Fatalf("err=%v", err)
	}
	if store.invalidations != 1 || store.value.RefreshToken != "" {
		t.Fatalf("invalidations=%d value=%+v", store.invalidations, store.value)
	}
}
