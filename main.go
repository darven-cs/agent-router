package main

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/charmbracelet/bubbletea"
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

	// Create load balancer
	lb := NewLoadBalancer(cfg.Upstreams)

	// Create log channel for request updates
	logChan := make(chan RequestLog, 100)

	// Create proxy handler
	proxyHandler := NewProxyHandler(lb, cfg.Service.APIKey, logChan)

	// Create TUI model
	upstreams := lb.GetEnabled()
	tuiModel := NewModel(cfg.Service.Name, cfg.Service.Version, cfg.Service.Port, upstreams)

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

	// Start TUI with log channel reader
	p := tea.NewProgram(tuiModel, tea.WithAltScreen())
	go func() {
		for log := range logChan {
			p.Send(log)
		}
	}()

	// Run TUI
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "TUI error: %v\n", err)
	}

	// Graceful shutdown
	fmt.Println("\nShutting down...")
	if err := server.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "Server shutdown error: %v\n", err)
	}
	fmt.Println("Goodbye!")
}

// HandleSignals sets up signal handling for graceful shutdown
func HandleSignals(done chan<- bool) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan
	done <- true
}
