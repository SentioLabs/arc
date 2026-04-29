package paste_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	_ "modernc.org/sqlite"

	"github.com/sentiolabs/arc/internal/paste"
	"github.com/sentiolabs/arc/internal/paste/sqlite"
)

func newTestServer(t *testing.T) *echo.Echo {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := sqlite.Apply(context.Background(), db); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
	e := echo.New()
	paste.NewHandlers(sqlite.New(db)).Register(e.Group("/api/paste"))
	return e
}

func TestCreatePaste(t *testing.T) {
	e := newTestServer(t)
	body, _ := json.Marshal(paste.CreatePasteRequest{PlanBlob: []byte{1, 2}, PlanIV: []byte{3, 4}, SchemaVer: 1})
	req := httptest.NewRequest(http.MethodPost, "/api/paste", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp paste.CreatePasteResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.ID == "" || resp.EditToken == "" {
		t.Errorf("missing id or edit_token in response: %+v", resp)
	}
}

func TestCreatePasteEmptyBody(t *testing.T) {
	e := newTestServer(t)
	body, _ := json.Marshal(paste.CreatePasteRequest{SchemaVer: 1}) // no PlanBlob or PlanIV
	req := httptest.NewRequest(http.MethodPost, "/api/paste", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestGetPaste(t *testing.T) {
	e := newTestServer(t)

	// Create a share first
	body, _ := json.Marshal(paste.CreatePasteRequest{PlanBlob: []byte{1, 2}, PlanIV: []byte{3, 4}, SchemaVer: 1})
	req := httptest.NewRequest(http.MethodPost, "/api/paste", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create failed with %d: %s", rec.Code, rec.Body.String())
	}
	var created paste.CreatePasteResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &created)

	// GET the share
	req2 := httptest.NewRequest(http.MethodGet, "/api/paste/"+created.ID, nil)
	rec2 := httptest.NewRecorder()
	e.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec2.Code, rec2.Body.String())
	}
	var got paste.GetPasteResponse
	if err := json.Unmarshal(rec2.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal get response: %v", err)
	}
	if got.ID != created.ID {
		t.Errorf("expected id %q, got %q", created.ID, got.ID)
	}
	if got.Events == nil {
		// Events may be nil when empty slice; just ensure no panic
	}
}

func TestGetPasteNotFound(t *testing.T) {
	e := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/paste/doesnotexist", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestUpdatePasteWithToken(t *testing.T) {
	e := newTestServer(t)

	// Create
	body, _ := json.Marshal(paste.CreatePasteRequest{PlanBlob: []byte{1}, PlanIV: []byte{2}, SchemaVer: 1})
	req := httptest.NewRequest(http.MethodPost, "/api/paste", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	var created paste.CreatePasteResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &created)

	// Update with correct token
	upd, _ := json.Marshal(map[string]interface{}{"plan_blob": []byte{9}, "plan_iv": []byte{8}})
	req2 := httptest.NewRequest(http.MethodPut, "/api/paste/"+created.ID, bytes.NewReader(upd))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Authorization", "Bearer "+created.EditToken)
	rec2 := httptest.NewRecorder()
	e.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d: %s", rec2.Code, rec2.Body.String())
	}
}

func TestUpdatePasteWrongToken(t *testing.T) {
	e := newTestServer(t)

	// Create
	body, _ := json.Marshal(paste.CreatePasteRequest{PlanBlob: []byte{1}, PlanIV: []byte{2}, SchemaVer: 1})
	req := httptest.NewRequest(http.MethodPost, "/api/paste", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	var created paste.CreatePasteResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &created)

	// Update with wrong token
	upd, _ := json.Marshal(map[string]interface{}{"plan_blob": []byte{9}, "plan_iv": []byte{8}})
	req2 := httptest.NewRequest(http.MethodPut, "/api/paste/"+created.ID, bytes.NewReader(upd))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Authorization", "Bearer wrongtoken")
	rec2 := httptest.NewRecorder()
	e.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d: %s", rec2.Code, rec2.Body.String())
	}
}

func TestUpdatePasteMissingAuth(t *testing.T) {
	e := newTestServer(t)

	// Create
	body, _ := json.Marshal(paste.CreatePasteRequest{PlanBlob: []byte{1}, PlanIV: []byte{2}, SchemaVer: 1})
	req := httptest.NewRequest(http.MethodPost, "/api/paste", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	var created paste.CreatePasteResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &created)

	// Update with no Authorization header
	upd, _ := json.Marshal(map[string]interface{}{"plan_blob": []byte{9}, "plan_iv": []byte{8}})
	req2 := httptest.NewRequest(http.MethodPut, "/api/paste/"+created.ID, bytes.NewReader(upd))
	req2.Header.Set("Content-Type", "application/json")
	rec2 := httptest.NewRecorder()
	e.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d: %s", rec2.Code, rec2.Body.String())
	}
}

func TestDeletePasteWithToken(t *testing.T) {
	e := newTestServer(t)

	// Create
	body, _ := json.Marshal(paste.CreatePasteRequest{PlanBlob: []byte{1}, PlanIV: []byte{2}, SchemaVer: 1})
	req := httptest.NewRequest(http.MethodPost, "/api/paste", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	var created paste.CreatePasteResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &created)

	// Delete with correct token
	req2 := httptest.NewRequest(http.MethodDelete, "/api/paste/"+created.ID, nil)
	req2.Header.Set("Authorization", "Bearer "+created.EditToken)
	rec2 := httptest.NewRecorder()
	e.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d: %s", rec2.Code, rec2.Body.String())
	}
}

func TestAppendEvent(t *testing.T) {
	e := newTestServer(t)

	// Create share
	body, _ := json.Marshal(paste.CreatePasteRequest{PlanBlob: []byte{1, 2}, PlanIV: []byte{3, 4}, SchemaVer: 1})
	req := httptest.NewRequest(http.MethodPost, "/api/paste", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	var created paste.CreatePasteResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &created)

	// Append event
	evBody, _ := json.Marshal(paste.AppendEventRequest{Blob: []byte{5, 6}, IV: []byte{7, 8}})
	req2 := httptest.NewRequest(http.MethodPost, "/api/paste/"+created.ID+"/blobs", bytes.NewReader(evBody))
	req2.Header.Set("Content-Type", "application/json")
	rec2 := httptest.NewRecorder()
	e.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec2.Code, rec2.Body.String())
	}
	var evResp map[string]string
	_ = json.Unmarshal(rec2.Body.Bytes(), &evResp)
	if evResp["id"] == "" {
		t.Errorf("expected event id in response: %+v", evResp)
	}

	// GET shows the event
	req3 := httptest.NewRequest(http.MethodGet, "/api/paste/"+created.ID, nil)
	rec3 := httptest.NewRecorder()
	e.ServeHTTP(rec3, req3)
	var got paste.GetPasteResponse
	_ = json.Unmarshal(rec3.Body.Bytes(), &got)
	if len(got.Events) != 1 {
		t.Errorf("expected 1 event, got %d", len(got.Events))
	}
}

func TestAppendEventToMissingShare(t *testing.T) {
	e := newTestServer(t)
	evBody, _ := json.Marshal(paste.AppendEventRequest{Blob: []byte{5, 6}, IV: []byte{7, 8}})
	req := httptest.NewRequest(http.MethodPost, "/api/paste/doesnotexist/blobs", bytes.NewReader(evBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}
