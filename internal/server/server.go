// Package server provides the arc server runtime.
package server

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/sentiolabs/arc/internal/api"
	"github.com/sentiolabs/arc/internal/storage/sqlite"
)

// Config holds server configuration.
type Config struct {
	Address string // Server address (e.g., ":7432")
	DBPath  string // Database path (empty for default)
}

// DefaultDataDir returns the default data directory (~/.arc).
func DefaultDataDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".arc")
}

// DefaultDBPath returns the default database path.
func DefaultDBPath() string {
	return filepath.Join(DefaultDataDir(), "data.db")
}

// PIDPath returns the path to the PID file.
func PIDPath() string {
	return filepath.Join(DefaultDataDir(), "server.pid")
}

// LogPath returns the path to the log file.
func LogPath() string {
	return filepath.Join(DefaultDataDir(), "server.log")
}

// Run starts the server and blocks until shutdown.
// It handles graceful shutdown on SIGINT/SIGTERM.
func Run(cfg Config) error {
	// Apply defaults
	if cfg.Address == "" {
		cfg.Address = ":7432"
	}

	// Initialize storage
	store, err := sqlite.New(cfg.DBPath)
	if err != nil {
		return fmt.Errorf("initialize storage: %w", err)
	}
	defer store.Close()

	// Create API server
	server := api.New(api.Config{
		Address: cfg.Address,
		Store:   store,
	})

	// Start server in goroutine
	errCh := make(chan error, 1)
	go func() {
		log.Printf("Starting arc server on %s", cfg.Address)
		log.Printf("Database: %s", store.Path())
		if err := server.Start(); err != nil {
			errCh <- err
		}
	}()

	// Wait for interrupt signal or server error
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-quit:
		log.Println("Shutdown signal received")
	case err := <-errCh:
		return fmt.Errorf("server error: %w", err)
	}

	// Graceful shutdown
	log.Println("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}

	log.Println("Server stopped")
	return nil
}

// RunWithRestart runs the server with automatic restart on crash.
func RunWithRestart(cfg Config) {
	for {
		err := Run(cfg)
		if err != nil {
			log.Printf("Server crashed: %v, restarting in 5s...", err)
			time.Sleep(5 * time.Second)
			continue
		}
		// Clean shutdown (SIGTERM)
		break
	}
}
