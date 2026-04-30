// Package main is arc-paste, a tiny standalone binary that exposes only the paste API
// and serves the SvelteKit SPA. Designed for public deployment as a
// zero-knowledge paste service for arc plan reviews.
package main

import (
	"context"
	"database/sql"
	"log"
	"os"
	"path/filepath"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	_ "modernc.org/sqlite" // match arc's driver

	"github.com/sentiolabs/arc/internal/paste"
	pastesqlite "github.com/sentiolabs/arc/internal/paste/sqlite"
	"github.com/sentiolabs/arc/web"
)

func main() {
	addr := envOr("ARC_PASTE_ADDR", ":7433")
	dbPath := envOr("ARC_PASTE_DB", "./arc-paste.db")

	// Ensure the parent directory exists. SQLite will create the db file but
	// not the directory, which matters when running under scratch / distroless
	// images that mount a fresh volume at a path the binary has never seen.
	if dir := filepath.Dir(dbPath); dir != "." && dir != "/" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			log.Fatalf("create db dir %q: %v", dir, err)
		}
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	if err := pastesqlite.Apply(context.Background(), db); err != nil {
		log.Fatalf("apply migrations: %v", err)
	}

	store := pastesqlite.New(db)
	handlers := paste.NewHandlers(store)

	e := echo.New()
	e.HideBanner = true
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.CORS())

	// Mount paste handlers at /api/paste
	handlers.Register(e.Group("/api/paste"))

	// Serve embedded SPA with fallback to index.html for routing
	web.RegisterSPA(e)

	if err := e.Start(addr); err != nil {
		log.Fatalf("start server: %v", err)
	}
}

// envOr returns the value of env variable key, or defaultVal if not set.
func envOr(key, defaultVal string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return defaultVal
}
