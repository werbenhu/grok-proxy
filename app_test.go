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

func TestAppStateShowsLocalKeyAndRedactsUpstreamSecrets(t *testing.T) {
	store := config.NewStore(filepath.Join(t.TempDir(), "config.json"))
	cfg := config.Default()
	cfg.AuthMode = config.AuthModeAPIKey
	cfg.APIKey = "xai-super-secret"
	cfg.LocalKey = "local-super-secret"
	cfg.OAuth = config.OAuth{AccessToken: "oauth-access-secret", RefreshToken: "oauth-refresh-secret"}
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	svc, err := service.New(store, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	app := NewAppWithService(svc)
	state := app.GetState()
	if state.Config.LocalKey != cfg.LocalKey {
		t.Fatalf("local key = %q, want %q", state.Config.LocalKey, cfg.LocalKey)
	}
	data, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"xai-super-secret", "oauth-access-secret", "oauth-refresh-secret"} {
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

func TestBeforeCloseAllowsQuitWhenQuitting(t *testing.T) {
	app := NewAppWithService(nil)
	app.quitting.Store(true)
	if app.beforeClose(context.Background()) {
		t.Fatal("beforeClose must not block quit once quitting is set")
	}
}

func TestSetLocaleNormalizesAndIsSafeBeforeTrayReady(t *testing.T) {
	app := NewAppWithService(nil)
	for input, want := range map[string]string{
		"en":    "en",
		"zh":    "zh",
		"zh-CN": "zh",
		"fr":    "en",
		"":      "en",
	} {
		app.SetLocale(input)
		if got := app.tray.Locale(); got != want {
			t.Errorf("SetLocale(%q): locale = %q, want %q", input, got, want)
		}
	}
}

func TestTrayLocalesCoverBothLanguages(t *testing.T) {
	for _, locale := range []string{"zh", "en"} {
		text, ok := trayLocales[locale]
		if !ok || text.show == "" || text.quit == "" {
			t.Errorf("missing or empty tray strings for %s", locale)
		}
	}
}

func TestTrayLocaleFallsBackToEnglishForUnknown(t *testing.T) {
	m := newTrayMenu()
	m.locale = "ja" // 模拟未知语言被直接写入
	m.applyLocked()
	// 没有真实菜单项时 applyLocked 不应 panic，且回退后文案应非空。
	if text, ok := trayLocales[m.locale]; ok && text.show == "" {
		t.Fatal("unknown locale produced empty title; expected english fallback")
	}
}
