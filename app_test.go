package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/werbenhu/grok-proxy/internal/config"
	"github.com/werbenhu/grok-proxy/internal/service"
)

func TestWailsDTOsAvoidReservedAndTimeTypes(t *testing.T) {
	seen := map[reflect.Type]bool{}
	var inspect func(reflect.Type)
	inspect = func(value reflect.Type) {
		if value.Kind() == reflect.Pointer {
			value = value.Elem()
		}
		if seen[value] {
			return
		}
		seen[value] = true
		if value.Name() == "Public" {
			t.Errorf("reserved Wails type name %s", value.Name())
		}
		if value.PkgPath() == "time" {
			t.Errorf("binding-unsafe time type %s", value)
		}
		if value.Kind() == reflect.Struct {
			for index := 0; index < value.NumField(); index++ {
				inspect(value.Field(index).Type)
			}
		}
	}
	inspect(reflect.TypeOf(service.State{}))
	inspect(reflect.TypeOf(service.ConnectionTest{}))
}

func TestRequiredFrontendSourcesExist(t *testing.T) {
	for _, path := range []string{"frontend/src/main.ts", "frontend/src/api.ts", "frontend/src/style.css"} {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("required frontend source %s: %v", path, err)
		}
	}
}

func TestAppStateNeverReturnsSecrets(t *testing.T) {
	store := config.NewStore(filepath.Join(t.TempDir(), "config.json"))
	cfg := config.Default()
	cfg.AuthMode = config.AuthModeAPIKey
	cfg.APIKey = "xai-super-secret"
	cfg.LocalKey = "local-super-secret"
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	svc, err := service.New(store, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	app := NewAppWithService(svc)
	data, err := json.Marshal(app.GetState())
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"xai-super-secret", "local-super-secret"} {
		if strings.Contains(string(data), secret) {
			t.Fatalf("secret leaked: %s", data)
		}
	}
}

func TestValidateOpenURL(t *testing.T) {
	for _, value := range []string{"https://auth.x.ai/activate", "https://accounts.x.ai/"} {
		if err := validateOpenURL(value); err != nil {
			t.Errorf("%s: %v", value, err)
		}
	}
	for _, value := range []string{"http://auth.x.ai/", "https://evil.example/", "https://user@auth.x.ai/", "javascript:alert(1)"} {
		if err := validateOpenURL(value); err == nil {
			t.Errorf("expected rejection for %s", value)
		}
	}
}

func TestAppShutdownStopsService(t *testing.T) {
	store := config.NewStore(filepath.Join(t.TempDir(), "config.json"))
	if err := store.Save(config.Default()); err != nil {
		t.Fatal(err)
	}
	svc, _ := service.New(store, nil, nil)
	app := NewAppWithService(svc)
	app.shutdown(context.Background())
	if svc.State().Running {
		t.Fatal("service still running")
	}
}
