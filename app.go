package main

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"github.com/werbenhu/grok-proxy/internal/auth"
	"github.com/werbenhu/grok-proxy/internal/config"
	"github.com/werbenhu/grok-proxy/internal/service"
)

type App struct {
	ctx           context.Context
	service       *service.Service
	configWarning string
}

func NewApp() (*App, error) {
	directory, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("获取用户配置目录: %w", err)
	}
	store := config.NewStore(filepath.Join(directory, "GrokProxy", "config.json"))
	configWarning := ""
	if _, err := store.Load(); err != nil {
		backup, recoverErr := store.BackupInvalidAndReset()
		if recoverErr != nil {
			return nil, errors.Join(err, recoverErr)
		}
		configWarning = fmt.Sprintf("原配置无效，已备份到 %s 并恢复默认设置", backup)
	}
	svc, err := service.New(store, nil, nil)
	if err != nil {
		return nil, err
	}
	app := NewAppWithService(svc)
	app.configWarning = configWarning
	return app, nil
}

func NewAppWithService(svc *service.Service) *App { return &App{service: svc} }

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	if a.configWarning != "" {
		runtime.LogWarning(ctx, a.configWarning)
	}
	// Proxy must be started manually from the UI.
}

func (a *App) shutdown(ctx context.Context) { _ = a.service.Stop(ctx) }

func (a *App) GetState() service.State { return a.service.State() }

func (a *App) SaveSettings(input service.Settings) (service.State, error) {
	return a.service.Save(a.context(), input)
}
func (a *App) StartProxy() (service.State, error) {
	err := a.service.Start(a.context())
	return a.service.State(), err
}
func (a *App) StopProxy() (service.State, error) {
	err := a.service.Stop(a.context())
	return a.service.State(), err
}
func (a *App) BeginOAuth() (auth.DeviceAuthorization, error) {
	return a.service.BeginOAuth(a.context())
}
func (a *App) CompleteOAuth(deviceCode string) (service.State, error) {
	return a.service.CompleteOAuth(a.context(), deviceCode)
}
func (a *App) ClearCredential() (service.State, error) { return a.service.ClearCredential(a.context()) }
func (a *App) TestConnection() (service.ConnectionTest, error) {
	ctx, cancel := context.WithTimeout(a.context(), 30*time.Second)
	defer cancel()
	return a.service.TestConnection(ctx)
}

func (a *App) OpenURL(raw string) error {
	if err := validateOpenURL(raw); err != nil {
		return err
	}
	if a.ctx == nil {
		return fmt.Errorf("应用尚未启动")
	}
	runtime.BrowserOpenURL(a.ctx, raw)
	return nil
}

func (a *App) context() context.Context {
	if a.ctx != nil {
		return a.ctx
	}
	return context.Background()
}

func validateOpenURL(raw string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return fmt.Errorf("URL 无效: %w", err)
	}
	if parsed.Scheme != "https" || parsed.User != nil || parsed.Hostname() == "" {
		return fmt.Errorf("只允许打开安全的 xAI 授权地址")
	}
	host := strings.ToLower(parsed.Hostname())
	if host != "auth.x.ai" && host != "accounts.x.ai" {
		return fmt.Errorf("不允许打开非 xAI 授权地址")
	}
	return nil
}
