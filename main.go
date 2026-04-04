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
	db        *gorm.DB
	usageChan chan RequestLog
)

func main() {
	// Find config file - same directory as executable
	execPath, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get executable path: %v\n", err)
		os.Exit(1)
	}
	configPath := filepath.Join(filepath.Dir(execPath), "config.yaml")

	// Load configuration
	cfg, err := LoadConfig(configPath)
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
		})
	}
	sharedUpstreams := NewSharedUpstreams(upstreamList)

	// Create load balancer
	lb := NewLoadBalancer(cfg.Upstreams)

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
	proxyHandler := NewProxyHandler(lb, cfg.Service.APIKey, logChan, usageChan)

	// Create TUI model with callbacks for upstream changes
	tuiModel := NewModel(cfg.Service.Name, cfg.Service.Version, cfg.Service.Port, sharedUpstreams.GetAll())
	tuiModel.OnUpstreamAdded = func(u *Upstream) {
		sharedUpstreams.Add(u)
		lb.AddUpstream(u)
	}
	tuiModel.OnUpstreamUpdated = func(u *Upstream, oldName string) {
		if oldName != "" && oldName != u.Name {
			sharedUpstreams.Delete(oldName)
			lb.DeleteUpstream(oldName)
		}
		sharedUpstreams.Update(u.Name, u)
		lb.UpdateUpstream(u)
	}
	tuiModel.OnUpstreamDeleted = func(name string) {
		sharedUpstreams.Delete(name)
		lb.DeleteUpstream(name)
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

// HandleSignals sets up signal handling for graceful shutdown
func HandleSignals(done chan<- bool) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan
	done <- true
}
