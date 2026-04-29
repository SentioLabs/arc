package api

import (
	"database/sql"

	"github.com/labstack/echo/v4"
	"github.com/sentiolabs/arc/internal/paste"
	pastesqlite "github.com/sentiolabs/arc/internal/paste/sqlite"
)

// registerPasteRoutes mounts the paste package's handlers under /api/paste.
// The caller passes the same DB used for arc's main storage; paste tables
// are added by pastesqlite.Apply during startup.
func registerPasteRoutes(e *echo.Echo, db *sql.DB) {
	store := pastesqlite.New(db)
	handlers := paste.NewHandlers(store)
	handlers.Register(e.Group("/api/paste"))
}
