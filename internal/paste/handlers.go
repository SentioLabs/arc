package paste

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
)

// ID and token sizes for paste resources. Picked to give plenty of entropy
// without being painful to copy/paste manually.
const (
	// shareIDLen is the length, in characters, of a share's URL slug.
	shareIDLen = 8
	// editTokenBytes is the random byte count behind an edit token.
	// 32 bytes → 64 hex chars, matching the format used by `arc share` clients.
	editTokenBytes = 32
	// eventIDRandBytes is the random tail appended to event IDs after the
	// nanosecond timestamp prefix.
	eventIDRandBytes = 12
)

// 64-bit nanosecond timestamps are split into 8 bytes by repeated >> 8 shifts;
// these constants name the high-order shift amounts to keep the byte build-up
// readable.
const (
	tsShift56 = 56
	tsShift48 = 48
	tsShift40 = 40
	tsShift32 = 32
	tsShift24 = 24
	tsShift16 = 16
	tsShift8  = 8
)

// Handlers holds the paste HTTP handler dependencies.
type Handlers struct {
	store Storage
}

// NewHandlers creates a new Handlers with the given storage backend.
func NewHandlers(s Storage) *Handlers {
	return &Handlers{store: s}
}

// Register mounts the paste endpoints on the provided echo.Group.
func (h *Handlers) Register(g *echo.Group) {
	g.POST("", h.createPaste)
	g.GET("/:id", h.getPaste)
	g.PUT("/:id", h.updatePaste)
	g.DELETE("/:id", h.deletePaste)
	g.POST("/:id/blobs", h.appendEvent)
}

// createPaste handles POST /. Creates a new share with a freshly minted ID
// and edit token. The edit token is returned to the caller exactly once —
// there is no recovery path if it's lost.
func (h *Handlers) createPaste(c echo.Context) error {
	var req CreatePasteRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if len(req.PlanBlob) == 0 || len(req.PlanIV) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "plan_blob and plan_iv required")
	}
	id, err := newShareID()
	if err != nil {
		return err
	}
	token, err := newEditToken()
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	sh := Share{
		ID:        id,
		PlanBlob:  req.PlanBlob,
		PlanIV:    req.PlanIV,
		SchemaVer: req.SchemaVer,
		CreatedAt: now,
		UpdatedAt: now,
		ExpiresAt: req.ExpiresAt,
	}
	if err := h.store.CreateShare(c.Request().Context(), sh, token); err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, CreatePasteResponse{ID: id, EditToken: token})
}

// getPaste handles GET /:id. Returns the share row plus its event log.
// Anonymous — no auth — since the encrypted blobs are already key-gated
// client-side via the URL fragment.
func (h *Handlers) getPaste(c echo.Context) error {
	id := c.Param("id")
	sh, err := h.store.GetShare(c.Request().Context(), id)
	if err != nil {
		if errors.Is(err, ErrShareNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "not found")
		}
		return err
	}
	events, err := h.store.ListEvents(c.Request().Context(), id)
	if err != nil {
		return err
	}
	if events == nil {
		events = []Event{}
	}
	return c.JSON(http.StatusOK, GetPasteResponse{Share: *sh, Events: events})
}

func (h *Handlers) updatePaste(c echo.Context) error {
	id := c.Param("id")
	token, err := bearerToken(c)
	if err != nil {
		return err
	}
	var req struct {
		PlanBlob []byte `json:"plan_blob"`
		PlanIV   []byte `json:"plan_iv"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if err := h.store.UpdateSharePlan(c.Request().Context(), id, req.PlanBlob, req.PlanIV, token); err != nil {
		if errors.Is(err, ErrInvalidEditToken) {
			return echo.NewHTTPError(http.StatusForbidden, "invalid edit token")
		}
		if errors.Is(err, ErrShareNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "not found")
		}
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *Handlers) deletePaste(c echo.Context) error {
	id := c.Param("id")
	token, err := bearerToken(c)
	if err != nil {
		return err
	}
	if err := h.store.DeleteShare(c.Request().Context(), id, token); err != nil {
		if errors.Is(err, ErrInvalidEditToken) {
			return echo.NewHTTPError(http.StatusForbidden, "invalid edit token")
		}
		if errors.Is(err, ErrShareNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "not found")
		}
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *Handlers) appendEvent(c echo.Context) error {
	id := c.Param("id")
	if _, err := h.store.GetShare(c.Request().Context(), id); err != nil {
		if errors.Is(err, ErrShareNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "not found")
		}
		return err
	}
	var req AppendEventRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if len(req.Blob) == 0 || len(req.IV) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "blob and iv required")
	}
	eventID, err := newEventID()
	if err != nil {
		return err
	}
	e := Event{
		ID:        eventID,
		ShareID:   id,
		Blob:      req.Blob,
		IV:        req.IV,
		CreatedAt: time.Now().UTC(),
	}
	if err := h.store.AppendEvent(c.Request().Context(), e); err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, map[string]string{"id": eventID})
}

// bearerToken extracts the Bearer token from the Authorization header.
func bearerToken(c echo.Context) (string, error) {
	auth := c.Request().Header.Get("Authorization")
	const prefix = "Bearer "
	if !strings.HasPrefix(auth, prefix) {
		return "", echo.NewHTTPError(http.StatusUnauthorized, "missing bearer token")
	}
	return strings.TrimPrefix(auth, prefix), nil
}

// newShareID returns a Crockford base32 (lowercase, without i/l/o/u) ID.
func newShareID() (string, error) {
	const alphabet = "0123456789abcdefghjkmnpqrstvwxyz"
	buf := make([]byte, shareIDLen)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	out := make([]byte, shareIDLen)
	for i := range out {
		out[i] = alphabet[int(buf[i])%len(alphabet)]
	}
	return string(out), nil
}

// newEditToken returns a hex-encoded random token (editTokenBytes random bytes).
func newEditToken() (string, error) {
	buf := make([]byte, editTokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

// newEventID returns a time-prefixed random hex event ID. The nanosecond
// timestamp goes first so the event IDs sort lexicographically by creation
// time, which is convenient when scanning logs or storage.
func newEventID() (string, error) {
	buf := make([]byte, eventIDRandBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	ts := time.Now().UTC().UnixNano()
	return hex.EncodeToString([]byte{
		byte(ts >> tsShift56), byte(ts >> tsShift48), byte(ts >> tsShift40), byte(ts >> tsShift32),
		byte(ts >> tsShift24), byte(ts >> tsShift16), byte(ts >> tsShift8), byte(ts),
	}) + hex.EncodeToString(buf), nil
}
