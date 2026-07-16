package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStoreSaveAndLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.json")
	store := NewStore(path)
	cfg := Default()
	cfg.AuthMode = AuthModeAPIKey
	cfg.APIKey = "xai-test"
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("temporary file remains: %v", err)
	}
	loaded, err := NewStore(path).Load()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.APIKey != "xai-test" || loaded.ListenPort != 8181 {
		t.Fatalf("loaded = %+v", loaded)
	}
	if current := store.Current(); current.APIKey != "xai-test" {
		t.Fatalf("current = %+v", current)
	}
}

func TestStoreLoadMissingFileCreatesDefaultsAndPersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	store := NewStore(path)
	got, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.ListenHost != "127.0.0.1" || got.ListenPort != 8181 {
		t.Fatalf("got = %+v", got)
	}
	if len(got.LocalKey) != LocalKeyLength {
		t.Fatalf("local key = %q", got.LocalKey)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected default config file: %v", err)
	}
	// Reload should keep the same generated key.
	loaded, err := NewStore(path).Load()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.LocalKey != got.LocalKey {
		t.Fatalf("persisted key changed: %q vs %q", loaded.LocalKey, got.LocalKey)
	}
}

func TestStoreRejectsInvalidFileWithoutChangingCurrent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	store := NewStore(path)
	initial, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"listenPort":0}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load(); err == nil {
		t.Fatal("expected invalid config error")
	}
	if current := store.Current(); current != initial {
		t.Fatalf("current changed = %+v", current)
	}
}

func TestStoreBacksUpInvalidFileAndResets(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"broken":`), 0o600); err != nil {
		t.Fatal(err)
	}
	store := NewStore(path)
	if _, err := store.Load(); err == nil {
		t.Fatal("expected invalid config")
	}
	backup, err := store.BackupInvalidAndReset()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(backup); err != nil {
		t.Fatalf("backup: %v", err)
	}
	cfg, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ListenHost != "127.0.0.1" || cfg.ListenPort != 8181 || len(cfg.LocalKey) != LocalKeyLength {
		t.Fatalf("cfg=%+v", cfg)
	}
}

func TestStoreFillsMissingLocalKeyOnLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"listenHost":"127.0.0.1","listenPort":8181}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	store := NewStore(path)
	cfg, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.LocalKey) != LocalKeyLength {
		t.Fatalf("local key = %q", cfg.LocalKey)
	}
	loaded, err := NewStore(path).Load()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.LocalKey != cfg.LocalKey {
		t.Fatalf("filled key not persisted: %q vs %q", loaded.LocalKey, cfg.LocalKey)
	}
}

func TestStoreOAuthAccessorsPersistTokens(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	store := NewStore(path)
	if _, err := store.Load(); err != nil {
		t.Fatal(err)
	}
	oauth := OAuth{AccessToken: "access", RefreshToken: "refresh"}
	if err := store.SaveOAuth(oauth); err != nil {
		t.Fatal(err)
	}
	if got := store.OAuth(); got.AccessToken != "access" || got.RefreshToken != "refresh" {
		t.Fatalf("oauth = %+v", got)
	}
	loaded, err := NewStore(path).Load()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.AuthMode != AuthModeOAuth || loaded.OAuth.RefreshToken != "refresh" {
		t.Fatalf("loaded = %+v", loaded)
	}
}
