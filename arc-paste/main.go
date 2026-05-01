// Package main is arc-paste, a tiny standalone binary that exposes only the paste API
// and serves the SvelteKit SPA. Designed for public deployment as a
// zero-knowledge paste service for arc plan reviews.
package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	_ "modernc.org/sqlite" // match arc's driver

	"github.com/sentiolabs/arc/internal/paste"
	pastesqlite "github.com/sentiolabs/arc/internal/paste/sqlite"
	"github.com/sentiolabs/arc/web"
)

// dbDirMode is the permission used when creating the parent directory of the
// SQLite database. World-readable on purpose — the file mode itself (which
// SQLite controls) is what protects the data.
const dbDirMode = 0o755

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	addr := envOr("ARC_PASTE_ADDR", ":7433")
	dbPath := envOr("ARC_PASTE_DB", "./arc-paste.db")

	// Ensure the parent directory exists. SQLite will create the db file but
	// not the directory, which matters when running under scratch / distroless
	// images that mount a fresh volume at a path the binary has never seen.
	if dir := filepath.Dir(dbPath); dir != "." && dir != "/" {
		if err := os.MkdirAll(dir, dbDirMode); err != nil {
			return fmt.Errorf("create db dir %q: %w", dir, err)
		}
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer db.Close()

	if err := pastesqlite.Apply(context.Background(), db); err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}

	handlers := paste.NewHandlers(pastesqlite.New(db))
	return newRouter(handlers).Start(addr)
}

// newRouter wires the arc-paste HTTP surface. Extracted so tests can exercise
// route precedence (/, /api/v1/*, SPA fallback) without binding a real listener.
func newRouter(handlers *paste.Handlers) *echo.Echo {
	e := echo.New()
	e.HideBanner = true
	e.Use(middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		LogStatus: true,
		LogURI:    true,
		LogMethod: true,
		LogError:  true,
		LogValuesFunc: func(_ echo.Context, v middleware.RequestLoggerValues) error {
			log.Printf("%s %s -> %d (err=%v)", v.Method, v.URI, v.Status, v.Error)
			return nil
		},
	}))
	e.Use(middleware.Recover())
	e.Use(middleware.CORS())

	// arc-paste only owns /api/paste/*. The arc SPA shell probes /api/v1/* on
	// boot; without this guard those probes fall through to the SPA fallback
	// and return HTML, which the browser then fails to JSON.parse.
	e.Any("/api/v1/*", func(c echo.Context) error {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "not found"})
	})

	// Paste deployments don't serve the arc app shell — the only useful
	// destination is /share/<id>. Mirrors the Caddy edge config so dev and
	// prod behave the same.
	e.Any("/", func(c echo.Context) error {
		return c.String(http.StatusNotFound, "Not found")
	})

	handlers.Register(e.Group("/api/paste"))
	web.RegisterSPA(e)
	return e
}

// envOr returns the value of env variable key, or defaultVal if not set.
func envOr(key, defaultVal string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return defaultVal
}
