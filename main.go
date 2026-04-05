package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/charmbracelet/bubbletea"
	"gorm.io/gorm"
)

var (
	db              *gorm.DB
	usageChan       chan RequestLog
	execPath        string
	sharedUpstreams *SharedUpstreams
	lb              *LoadBalancer
	proxyHandler    *ProxyHandler
	cfg             *Config
	startTime       = time.Now()
)

func main() {
	// Find config file - same directory as executable
	var err error
	execPath, err = os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get executable path: %v\n", err)
		os.Exit(1)
	}
	configPath := filepath.Join(filepath.Dir(execPath), "config.yaml")

	// Load configuration
	cfg, err = LoadConfig(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config from %s: %v\n", configPath, err)
		os.Exit(1)
	}

	// Initialize shared upstreams from config
	var upstreamList []*Upstream
	for _, cfg := range cfg.Upstreams {
		upstreamList = append(upstreamList, &Upstream{
			Name:     cfg.Name,
			URL:      cfg.URL,
			APIKey:   cfg.APIKey,
			AuthType: cfg.AuthType,
			Enabled:  cfg.Enabled,
			Timeout:  time.Duration(cfg.Timeout) * time.Second,
			Model:    cfg.Model,
		})
	}
	sharedUpstreams = NewSharedUpstreams(upstreamList)

	// Create load balancer
	lb = NewLoadBalancer(cfg.Upstreams)

	// Create log channel for request updates
	logChan := make(chan RequestLog, 100)

	// Create usage channel for SQLite persistence
	usageChan = make(chan RequestLog, 100)

	// Initialize SQLite for usage tracking
	dbPath := filepath.Join(filepath.Dir(execPath), "usage.db")
	db, err = initDB(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to init usage DB: %v\n", err)
		// Continue without usage tracking - non-fatal
	}

	// Start usage worker goroutine
	if db != nil {
		go StartUsageWorker(db, usageChan)
	}

	// Create proxy handler
	proxyHandler = NewProxyHandler(lb, cfg.Service.APIKey, cfg.Service.Model, logChan, usageChan)

	// Create TUI model with callbacks for upstream changes
	tuiModel := NewModel(cfg.Service.Name, cfg.Service.Version, cfg.Service.Port, sharedUpstreams.GetAll())
	tuiModel.defaultModel = cfg.Service.Model
	tuiModel.OnUpstreamAdded = func(u *Upstream) {
		sharedUpstreams.Add(u)
		lb.AddUpstream(u)
		if err := persistConfig(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to persist config: %v\n", err)
		}
	}
	tuiModel.OnUpstreamUpdated = func(u *Upstream, oldName string) {
		if oldName != "" && oldName != u.Name {
			sharedUpstreams.Delete(oldName)
			lb.DeleteUpstream(oldName)
		}
		sharedUpstreams.Update(u.Name, u)
		lb.UpdateUpstream(u)
		if err := persistConfig(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to persist config: %v\n", err)
		}
	}
	tuiModel.OnUpstreamDeleted = func(name string) {
		sharedUpstreams.Delete(name)
		lb.DeleteUpstream(name)
		if err := persistConfig(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to persist config: %v\n", err)
		}
	}
	tuiModel.OnUpstreamToggled = func(u *Upstream) {
		sharedUpstreams.Update(u.Name, u)
		lb.UpdateUpstream(u)
		if err := persistConfig(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to persist config: %v\n", err)
		}
	}
	tuiModel.OnDefaultModelChanged = func(model string) {
		cfg.Service.Model = model
		proxyHandler.defaultModel = model
		if err := persistConfig(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to persist config: %v\n", err)
		}
	}
	tuiModel.OnUpstreamModelSelected = func(u *Upstream) {
		sharedUpstreams.Update(u.Name, u)
		lb.UpdateUpstream(u)
		if err := persistConfig(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to persist upstream model: %v\n", err)
		}
	}
	tuiModel.OnReload = func() error {
		return doReload()
	}

	// Start HTTP server in background
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Service.Port),
		Handler: proxyHandler,
	}

	go func() {
		fmt.Printf("Starting HTTP server on port %d...\n", cfg.Service.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "HTTP server error: %v\n", err)
		}
	}()

	// Start SIGHUP handler for config hot reload
	sighupChan := make(chan os.Signal, 1)
	signal.Notify(sighupChan, syscall.SIGHUP)
	go func() {
		for range sighupChan {
			fmt.Println("Received SIGHUP, reloading configuration...")
			if err := doReload(); err != nil {
				fmt.Fprintf(os.Stderr, "Config reload failed: %v\n", err)
			}
		}
	}()

	// Run TUI
	p := tea.NewProgram(tuiModel, tea.WithAltScreen())
	go func() {
		for log := range logChan {
			p.Send(log)
		}
	}()

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "TUI error: %v\n", err)
	}

	// Graceful shutdown with 10s timeout
	fmt.Println("\nShutting down gracefully...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Shutdown error (forcing close): %v\n", err)
		server.Close()
	}

	// Close usage channel to signal worker to stop
	close(usageChan)

	fmt.Println("Goodbye!")
}

// persistConfig saves the current upstream configuration to config.yaml
// This is called after each TUI add/edit/delete/enable/disable to ensure
// runtime changes survive SIGHUP reload.
func persistConfig() error {
	configPath := filepath.Join(filepath.Dir(execPath), "config.yaml")

	// Build UpstreamConfig list from current SharedUpstreams state
	upstreams := sharedUpstreams.GetAll()
	upstreamConfigs := make([]UpstreamConfig, 0, len(upstreams))
	for _, u := range upstreams {
		upstreamConfigs = append(upstreamConfigs, UpstreamConfig{
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
	newCfg := &Config{
		Service:   cfg.Service,
		Upstreams: upstreamConfigs,
	}

	return SaveConfig(newCfg, configPath)
}

// doReload re-reads config.yaml and reinitializes the LoadBalancer
func doReload() error {
	configPath := filepath.Join(filepath.Dir(execPath), "config.yaml")

	newCfg, err := LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Re-initialize LoadBalancer from config
	newUpstreams := NewLoadBalancer(newCfg.Upstreams)

	// Create new upstream list from config
	var newList []*Upstream
	for _, uc := range newCfg.Upstreams {
		newList = append(newList, &Upstream{
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
	sharedUpstreams.ReplaceAll(newList)

	// Re-create load balancer (replace the old one)
	lb = newUpstreams

	// Update proxy handler's load balancer reference
	proxyHandler.lb = lb
	proxyHandler.defaultModel = newCfg.Service.Model

	// Update global cfg reference
	cfg = newCfg

	fmt.Println("Config reloaded successfully")
	return nil
}

// HandleSignals sets up signal handling for graceful shutdown
func HandleSignals(done chan<- bool) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan
	done <- true
}
