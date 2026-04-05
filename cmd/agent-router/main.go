package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/charmbracelet/bubbletea"
	"gorm.io/gorm"

	"agent-router/internal/admin"
	"agent-router/internal/config"
	"agent-router/internal/proxy"
	"agent-router/internal/storage"
	"agent-router/internal/upstream"
	tui "agent-router/internal/tui"
)

// App holds all application dependencies, replacing global variables
type App struct {
	cfg                 *config.Config
	db                  *gorm.DB
	sharedUpstreams     *upstream.SharedUpstreams
	lb                  *upstream.LoadBalancer
	proxyHandler        *proxy.ProxyHandler
	adminHandler        *admin.AdminHandler
	usageChan           chan proxy.RequestLog
	logChan             chan proxy.RequestLog
	startTime           time.Time
	execPath            string
	configPath          string
	server              *http.Server
	primaryUpstreamName string // persisted primary upstream selection
}

// NewApp initializes all application dependencies
func NewApp() (*App, error) {
	app := &App{
		startTime: time.Now(),
	}

	// Find config file - same directory as executable
	var err error
	app.execPath, err = os.Executable()
	if err != nil {
		return nil, fmt.Errorf("failed to get executable path: %w", err)
	}
	configPath := filepath.Join(filepath.Dir(app.execPath), "config.yaml")
	app.configPath = configPath

	// Load configuration
	app.cfg, err = config.LoadConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config from %s: %w", configPath, err)
	}

	// Initialize shared upstreams from config
	var upstreamList []*upstream.Upstream
	for _, uc := range app.cfg.Upstreams {
		upstreamList = append(upstreamList, &upstream.Upstream{
			Name:     uc.Name,
			URL:      uc.URL,
			APIKey:   uc.APIKey,
			AuthType: uc.AuthType,
			Enabled:  uc.Enabled,
			Timeout:  time.Duration(uc.Timeout) * time.Second,
			Model:    uc.Model,
		})
	}
	app.sharedUpstreams = upstream.NewSharedUpstreams(upstreamList)

	// Create load balancer
	app.lb = upstream.NewLoadBalancer(app.cfg.Upstreams)

	// Restore primary upstream from config if set (B+ fix: survives restart)
	if app.cfg.PrimaryUpstream != "" {
		for _, u := range app.lb.GetEnabled() {
			if u.Name == app.cfg.PrimaryUpstream {
				app.lb.SetPrimary(u)
				app.primaryUpstreamName = app.cfg.PrimaryUpstream
				break
			}
		}
	}

	// Create log channel for request updates
	app.logChan = make(chan proxy.RequestLog, 100)

	// Create usage channel for SQLite persistence
	app.usageChan = make(chan proxy.RequestLog, 100)

	// Initialize SQLite for usage tracking
	dbPath := filepath.Join(filepath.Dir(app.execPath), "usage.db")
	app.db, err = storage.InitDB(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to init usage DB: %v\n", err)
		// Continue without usage tracking - non-fatal
	}

	// Start usage worker goroutine
	if app.db != nil {
		storage.StartUsageWorker(app.db, app.usageChan)
	}

	// Create proxy handler
	app.proxyHandler = proxy.NewProxyHandler(app.lb, app.cfg.Service.APIKey, app.cfg.Service.Model, app.logChan, app.usageChan)

	// Create admin handler
	app.adminHandler = admin.NewAdminHandler(
		app.cfg, app.db, storage.Stats, app.sharedUpstreams,
		app.startTime, app.doReload,
	)

	return app, nil
}

// Run starts the HTTP server and TUI
func (app *App) Run() error {
	// Create HTTP handler with path-based routing
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/admin/") {
			app.adminHandler.ServeHTTP(w, r)
		} else {
			app.proxyHandler.ServeHTTP(w, r)
		}
	})

	// Start HTTP server in background
	app.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", app.cfg.Service.Port),
		Handler: handler,
	}

	go func() {
		fmt.Printf("Starting HTTP server on port %d...\n", app.cfg.Service.Port)
		if err := app.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "HTTP server error: %v\n", err)
		}
	}()

	// Start SIGHUP handler for config hot reload
	sighupChan := make(chan os.Signal, 1)
	signal.Notify(sighupChan, syscall.SIGHUP)
	go func() {
		for range sighupChan {
			fmt.Println("Received SIGHUP, reloading configuration...")
			if err := app.doReload(); err != nil {
				fmt.Fprintf(os.Stderr, "Config reload failed: %v\n", err)
			}
		}
	}()

	// Create TUI model with callbacks for upstream changes
	tuiModel := tui.NewModel(app.cfg.Service.Name, app.cfg.Service.Version, app.cfg.Service.Port, app.sharedUpstreams.GetAll())
	tuiModel.DefaultModel = app.cfg.Service.Model
	tuiModel.Callbacks.OnUpstreamAdded = func(u *upstream.Upstream) {
		app.sharedUpstreams.Add(u)
		app.lb.AddUpstream(u)
		if err := app.persistConfig(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to persist config: %v\n", err)
		}
	}
	tuiModel.Callbacks.OnUpstreamUpdated = func(u *upstream.Upstream, oldName string) {
		if oldName != "" && oldName != u.Name {
			app.sharedUpstreams.Delete(oldName)
			app.lb.DeleteUpstream(oldName)
		}
		app.sharedUpstreams.Update(u.Name, u)
		app.lb.UpdateUpstream(u)
		if err := app.persistConfig(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to persist config: %v\n", err)
		}
	}
	tuiModel.Callbacks.OnUpstreamDeleted = func(name string) {
		app.sharedUpstreams.Delete(name)
		app.lb.DeleteUpstream(name)
		if err := app.persistConfig(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to persist config: %v\n", err)
		}
	}
	tuiModel.Callbacks.OnUpstreamToggled = func(u *upstream.Upstream) {
		app.sharedUpstreams.Update(u.Name, u)
		app.lb.UpdateUpstream(u)
		if err := app.persistConfig(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to persist config: %v\n", err)
		}
	}
	tuiModel.Callbacks.OnDefaultModelChanged = func(model string) {
		app.cfg.Service.Model = model
		app.proxyHandler.SetDefaultModel(model)
		if err := app.persistConfig(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to persist config: %v\n", err)
		}
	}
	tuiModel.Callbacks.OnUpstreamModelSelected = func(u *upstream.Upstream) {
		app.sharedUpstreams.Update(u.Name, u)
		app.lb.UpdateUpstream(u)
		if err := app.persistConfig(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to persist upstream model: %v\n", err)
		}
	}
	tuiModel.Callbacks.OnReload = func() error {
		return app.doReload()
	}
	// Primary upstream callbacks (wired for Task 2)
	tuiModel.Callbacks.OnPrimarySelected = func(u *upstream.Upstream) {
		app.lb.SetPrimary(u)
		app.primaryUpstreamName = u.Name
		app.cfg.PrimaryUpstream = u.Name
		if err := config.SaveConfig(app.cfg, app.configPath); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to persist primary upstream: %v\n", err)
		}
	}
	tuiModel.Callbacks.OnPrimaryCleared = func() {
		app.lb.ClearPrimary()
		app.primaryUpstreamName = ""
		app.cfg.PrimaryUpstream = ""
		if err := config.SaveConfig(app.cfg, app.configPath); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to persist primary upstream: %v\n", err)
		}
	}

	// Run TUI
	p := tea.NewProgram(tuiModel, tea.WithAltScreen())
	go func() {
		for log := range app.logChan {
			p.Send(log)
		}
	}()

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "TUI error: %v\n", err)
	}

	// Graceful shutdown
	return app.Shutdown()
}

// Shutdown gracefully stops the HTTP server and closes channels
func (app *App) Shutdown() error {
	fmt.Println("\nShutting down gracefully...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if app.server != nil {
		if err := app.server.Shutdown(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "Shutdown error (forcing close): %v\n", err)
			app.server.Close()
		}
	}

	// Close usage channel to signal worker to stop
	if app.usageChan != nil {
		close(app.usageChan)
	}

	fmt.Println("Goodbye!")
	return nil
}

// persistConfig saves the current upstream configuration to config.yaml
func (app *App) persistConfig() error {
	configPath := filepath.Join(filepath.Dir(app.execPath), "config.yaml")

	// Build UpstreamConfig list from current SharedUpstreams state
	upstreams := app.sharedUpstreams.GetAll()
	upstreamConfigs := make([]config.UpstreamConfig, 0, len(upstreams))
	for _, u := range upstreams {
		upstreamConfigs = append(upstreamConfigs, config.UpstreamConfig{
			Name:     u.Name,
			URL:      u.URL,
			APIKey:   u.APIKey,
			AuthType: u.AuthType,
			Enabled:  u.Enabled,
			Timeout:  int(u.Timeout.Seconds()),
			Model:    u.Model,
		})
	}

	// Create a new Config with current state
	newCfg := &config.Config{
		Service:         app.cfg.Service,
		Upstreams:       upstreamConfigs,
		PrimaryUpstream: app.cfg.PrimaryUpstream,
	}

	return config.SaveConfig(newCfg, configPath)
}

// doReload re-reads config.yaml and reinitializes the LoadBalancer
func (app *App) doReload() error {
	configPath := filepath.Join(filepath.Dir(app.execPath), "config.yaml")

	newCfg, err := config.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Create new LoadBalancer from config
	app.lb = upstream.NewLoadBalancer(newCfg.Upstreams)

	// Restore primary upstream if set in config (B+ fix: survives SIGHUP reload)
	if newCfg.PrimaryUpstream != "" {
		for _, u := range app.lb.GetEnabled() {
			if u.Name == newCfg.PrimaryUpstream {
				app.lb.SetPrimary(u)
				app.primaryUpstreamName = newCfg.PrimaryUpstream
				break
			}
		}
	}

	// Create new upstream list from config
	var newList []*upstream.Upstream
	for _, uc := range newCfg.Upstreams {
		newList = append(newList, &upstream.Upstream{
			Name:     uc.Name,
			URL:      uc.URL,
			APIKey:   uc.APIKey,
			AuthType: uc.AuthType,
			Enabled:  uc.Enabled,
			Timeout:  time.Duration(uc.Timeout) * time.Second,
			Model:    uc.Model,
		})
	}

	// Update shared upstreams (thread-safe via ReplaceAll)
	app.sharedUpstreams.ReplaceAll(newList)

	// Update proxy handler's load balancer reference
	app.proxyHandler.SetLoadBalancer(app.lb)
	app.proxyHandler.SetDefaultModel(newCfg.Service.Model)

	// Update config reference
	app.cfg = newCfg

	// Recreate admin handler with new config
	app.adminHandler = admin.NewAdminHandler(
		app.cfg, app.db, storage.Stats, app.sharedUpstreams,
		app.startTime, app.doReload,
	)

	fmt.Println("Config reloaded successfully")
	return nil
}

func main() {
	app, err := NewApp()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize: %v\n", err)
		os.Exit(1)
	}

	if err := app.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Application error: %v\n", err)
		os.Exit(1)
	}
}
