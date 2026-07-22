package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Store struct {
	mu      sync.RWMutex
	path    string
	current Config
}

func NewStore(path string) *Store {
	return &Store{path: path, current: Default()}
}

func (s *Store) Path() string { return s.path }

func (s *Store) Load() (Config, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		cfg := Default()
		if err := s.saveLocked(cfg); err != nil {
			return Config{}, err
		}
		return cfg, nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var cfg Config
	if err := decoder.Decode(&cfg); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return Config{}, err
	}
	cfg, filled := EnsureLocalKey(cfg)
	if err := Validate(cfg); err != nil {
		return Config{}, fmt.Errorf("validate config: %w", err)
	}
	if filled {
		if err := s.saveLocked(cfg); err != nil {
			return Config{}, err
		}
		return cfg, nil
	}
	s.current = cfg
	return cfg, nil
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return fmt.Errorf("parse config: multiple JSON values")
		}
		return fmt.Errorf("parse config: %w", err)
	}
	return nil
}

func (s *Store) Save(cfg Config) error {
	if err := Validate(cfg); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveLocked(cfg)
}

func (s *Store) saveLocked(cfg Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	data = append(data, '\n')
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	temporary := s.path + ".tmp"
	file, err := os.OpenFile(temporary, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("create temp config: %w", err)
	}
	cleanup := func() { _ = file.Close(); _ = os.Remove(temporary) }
	if _, err := file.Write(data); err != nil {
		cleanup()
		return fmt.Errorf("write temp config: %w", err)
	}
	if err := file.Sync(); err != nil {
		cleanup()
		return fmt.Errorf("sync temp config: %w", err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(temporary)
		return fmt.Errorf("close temp config: %w", err)
	}
	if err := os.Chmod(temporary, 0o600); err != nil {
		_ = os.Remove(temporary)
		return fmt.Errorf("set config permissions: %w", err)
	}
	if err := os.Rename(temporary, s.path); err != nil {
		_ = os.Remove(temporary)
		return fmt.Errorf("replace config: %w", err)
	}
	s.current = cfg
	return nil
}

func (s *Store) Current() Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.current
}

func (s *Store) OAuth() OAuth {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.current.OAuth
}

func (s *Store) SaveOAuth(value OAuth) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cfg := s.current
	cfg.AuthMode = AuthModeOAuth
	cfg.APIKey = ""
	cfg.OAuth = value
	if err := Validate(cfg); err != nil {
		return err
	}
	return s.saveLocked(cfg)
}

func (s *Store) InvalidateOAuth() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cfg := s.current
	cfg.AuthMode = AuthModeNone
	cfg.OAuth = OAuth{}
	return s.saveLocked(cfg)
}

func (s *Store) BackupInvalidAndReset() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	backup := s.path + ".invalid-" + time.Now().UTC().Format("20060102T150405.000000000Z")
	if err := os.Rename(s.path, backup); err != nil {
		return "", fmt.Errorf("back up invalid config: %w", err)
	}
	if err := s.saveLocked(Default()); err != nil {
		return backup, fmt.Errorf("reset invalid config: %w", err)
	}
	return backup, nil
}
