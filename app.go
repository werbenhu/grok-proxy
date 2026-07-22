package main

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/energye/systray"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"github.com/werbenhu/grok-proxy/internal/auth"
	"github.com/werbenhu/grok-proxy/internal/config"
	"github.com/werbenhu/grok-proxy/internal/service"
)

type App struct {
	ctx           context.Context
	service       *service.Service
	configWarning string
	quitting      atomic.Bool
	tray          *trayMenu
}

// 托盘菜单文案，跟随前端语言切换。
var trayLocales = map[string]struct{ show, quit string }{
	"zh": {show: "打开主界面", quit: "退出"},
	"en": {show: "Open GrokProxy", quit: "Quit"},
}

// trayMenu 持有托盘菜单项引用与当前语言。菜单在 systray 自己的 goroutine
// 上创建，而 SetLocale 由前端随时调用，两边用互斥锁串行化。
type trayMenu struct {
	mu     sync.Mutex
	locale string
	show   *systray.MenuItem
	quit   *systray.MenuItem
}

func newTrayMenu() *trayMenu { return &trayMenu{locale: "zh"} }

// normalizeTrayLocale 与前端 detectLocale 的回退规则保持一致：
// 中文（含 zh-CN 等变体）用 zh，其余一律用 en。
func normalizeTrayLocale(locale string) string {
	if strings.HasPrefix(strings.ToLower(locale), "zh") {
		return "zh"
	}
	return "en"
}

func (m *trayMenu) setLocale(locale string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.locale = normalizeTrayLocale(locale)
	m.applyLocked()
}

// locale 返回当前语言快照，供测试断言使用。
func (m *trayMenu) Locale() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.locale
}

// applyLocked 在菜单项尚未创建时只是记住语言，等托盘就绪后再生效。
func (m *trayMenu) applyLocked() {
	text, ok := trayLocales[m.locale]
	if !ok || text.show == "" {
		// 未知语言或缺词条时回退到英文，避免菜单出现空标题。
		text = trayLocales["en"]
	}
	if m.show != nil {
		m.show.SetTitle(text.show)
	}
	if m.quit != nil {
		m.quit.SetTitle(text.quit)
	}
}

func NewApp() (*App, error) {
	directory, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("get user config dir: %w", err)
	}
	store := config.NewStore(filepath.Join(directory, "GrokProxy", "config.json"))
	configWarning := ""
	if _, err := store.Load(); err != nil {
		backup, recoverErr := store.BackupInvalidAndReset()
		if recoverErr != nil {
			return nil, errors.Join(err, recoverErr)
		}
		configWarning = fmt.Sprintf("invalid config backed up to %s; defaults restored", backup)
	}
	svc, err := service.New(store, nil, nil)
	if err != nil {
		return nil, err
	}
	app := NewAppWithService(svc)
	app.configWarning = configWarning
	return app, nil
}

func NewAppWithService(svc *service.Service) *App {
	return &App{service: svc, tray: newTrayMenu()}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	if a.configWarning != "" {
		runtime.LogWarning(ctx, a.configWarning)
	}
	a.initSystray()
	// Proxy must be started manually from the UI.
	// Check the stored Grok authorization in the background: a still-valid
	// token keeps the UI "connected" across restarts, an expired one flips
	// the status to reauthorization_required without waiting for traffic.
	go func() {
		ctx, cancel := context.WithTimeout(a.ctx, 30*time.Second)
		defer cancel()
		a.service.RefreshAuth(ctx)
	}()
}

func (a *App) beforeClose(ctx context.Context) bool {
	// 托盘菜单选择退出时必须放行，否则 runtime.Quit 会被这里拦截，
	// 进程会一直残留。
	if a.quitting.Load() {
		return false
	}
	runtime.WindowHide(ctx)
	return true
}

func (a *App) initSystray() {
	go systray.Run(func() {
		systray.SetIcon(trayIcon)
		systray.SetTooltip("GrokProxy")

		a.tray.mu.Lock()
		a.tray.show = systray.AddMenuItem("", "")
		systray.AddSeparator()
		a.tray.quit = systray.AddMenuItem("", "")
		a.tray.applyLocked()
		mShow, mQuit := a.tray.show, a.tray.quit
		a.tray.mu.Unlock()

		mShow.Click(func() {
			runtime.WindowShow(a.ctx)
			runtime.WindowUnminimise(a.ctx)
		})
		mQuit.Click(func() {
			a.quitting.Store(true)
			systray.Quit()
			runtime.Quit(a.ctx)
		})
	}, nil)
}

// SetLocale 由前端在切换语言时调用，同步更新托盘菜单文案；前端启动时
// 也会调用一次，保证持久化的语言选择对托盘生效。
func (a *App) SetLocale(locale string) { a.tray.setLocale(locale) }

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
		return fmt.Errorf("app not started")
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
		return fmt.Errorf("invalid URL: %w", err)
	}
	if parsed.Scheme != "https" || parsed.User != nil || parsed.Hostname() == "" {
		return fmt.Errorf("only secure xAI authorization URLs are allowed")
	}
	host := strings.ToLower(parsed.Hostname())
	if host != "auth.x.ai" && host != "accounts.x.ai" {
		return fmt.Errorf("opening non-xAI authorization URLs is not allowed")
	}
	return nil
}
