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
	sh := PasteShare{
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
		events = []PasteEvent{}
	}
	return c.JSON(http.StatusOK, GetPasteResponse{PasteShare: *sh, Events: events})
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
	e := PasteEvent{
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

// newShareID returns an 8-character Crockford base32 (lowercase, without i/l/o/u) ID.
func newShareID() (string, error) {
	const alphabet = "0123456789abcdefghjkmnpqrstvwxyz"
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	out := make([]byte, 8)
	for i := range out {
		out[i] = alphabet[int(buf[i])%len(alphabet)]
	}
	return string(out), nil
}

// newEditToken returns a 64-character hex-encoded random token (32 random bytes).
func newEditToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

// newEventID returns a time-prefixed random hex event ID.
func newEventID() (string, error) {
	buf := make([]byte, 12)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	ts := time.Now().UTC().UnixNano()
	return hex.EncodeToString([]byte{
		byte(ts >> 56), byte(ts >> 48), byte(ts >> 40), byte(ts >> 32),
		byte(ts >> 24), byte(ts >> 16), byte(ts >> 8), byte(ts),
	}) + hex.EncodeToString(buf), nil
}
