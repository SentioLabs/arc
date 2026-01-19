// Command server starts the beads-central REST API server.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sentiolabs/beads-central/internal/api"
	"github.com/sentiolabs/beads-central/internal/storage/sqlite"
)

func main() {
	var (
		addr   = flag.String("addr", ":7432", "Server address")
		dbPath = flag.String("db", "", "Database path (default: ~/.beads-server/data.db)")
	)
	flag.Parse()

	// Initialize storage
	store, err := sqlite.New(*dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}
	defer store.Close()

	// Create server
	server := api.New(api.Config{
		Address: *addr,
		Store:   store,
	})

	// Start server in goroutine
	go func() {
		fmt.Printf("Starting beads-central server on %s\n", *addr)
		fmt.Printf("Database: %s\n", store.Path())
		if err := server.Start(); err != nil {
			log.Printf("Server error: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	// Graceful shutdown
	fmt.Println("\nShutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server shutdown failed: %v", err)
	}
	fmt.Println("Server stopped")
}
