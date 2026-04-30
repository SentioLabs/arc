package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/sentiolabs/arc/internal/paste"
	pastesqlite "github.com/sentiolabs/arc/internal/paste/sqlite"
	_ "modernc.org/sqlite"
)

func TestArcPasteCreate(t *testing.T) {
	db, _ := sql.Open("sqlite", ":memory:")
	defer db.Close()
	_ = pastesqlite.Apply(context.Background(), db)
	e := echo.New()
	paste.NewHandlers(pastesqlite.New(db)).Register(e.Group("/api/paste"))

	body, _ := json.Marshal(map[string]any{
		"plan_blob":  []byte{1, 2, 3},
		"plan_iv":    []byte{4, 5, 6},
		"schema_ver": 1,
	})
	req := httptest.NewRequest("POST", "/api/paste", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}
}
