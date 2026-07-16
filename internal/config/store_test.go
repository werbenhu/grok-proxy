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

func TestStoreLoadMissingFileCreatesDefaultsInMemory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	store := NewStore(path)
	got, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got != Default() {
		t.Fatalf("got = %+v", got)
	}
}

func TestStoreRejectsInvalidFileWithoutChangingCurrent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	store := NewStore(path)
	if _, err := store.Load(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"listenPort":0}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load(); err == nil {
		t.Fatal("expected invalid config error")
	}
	if current := store.Current(); current != Default() {
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
	if err != nil || cfg != Default() {
		t.Fatalf("cfg=%+v err=%v", cfg, err)
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
