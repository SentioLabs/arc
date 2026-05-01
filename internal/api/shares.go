// Share keyring API handlers — author-side registry of paste shares created
// from this machine. The keyring stores the secrets (edit_token, key_b64url)
// that authenticate this user as the share author to the hosting paste server.
package api

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/sentiolabs/arc/internal/storage"
	"github.com/sentiolabs/arc/internal/types"
)

// listShares returns all keyring entries, newest first.
func (s *Server) listShares(c echo.Context) error {
	shares, err := s.store.ListShares(c.Request().Context())
	if err != nil {
		return errorJSON(c, http.StatusInternalServerError, err.Error())
	}
	return successJSON(c, shares)
}

// getShare returns a single keyring entry by share ID.
func (s *Server) getShare(c echo.Context) error {
	id := c.Param("id")
	share, err := s.store.GetShare(c.Request().Context(), id)
	if err != nil {
		if errors.Is(err, storage.ErrShareNotFound) {
			return errorJSON(c, http.StatusNotFound, err.Error())
		}
		return errorJSON(c, http.StatusInternalServerError, err.Error())
	}
	return successJSON(c, share)
}

// upsertShare inserts or replaces a keyring entry by share ID. The store
// stamps CreatedAt when callers omit it, so handlers and the legacy import
// path both end up with consistent timestamps.
func (s *Server) upsertShare(c echo.Context) error {
	var share types.Share
	if err := c.Bind(&share); err != nil {
		return errorJSON(c, http.StatusBadRequest, "invalid request body")
	}
	if err := share.Validate(); err != nil {
		return errorJSON(c, http.StatusBadRequest, err.Error())
	}
	if err := s.store.UpsertShare(c.Request().Context(), &share); err != nil {
		return errorJSON(c, http.StatusInternalServerError, err.Error())
	}
	return successJSON(c, &share)
}

// deleteShare removes a keyring entry by share ID. Idempotent: 204 even if
// the ID doesn't exist.
func (s *Server) deleteShare(c echo.Context) error {
	id := c.Param("id")
	if err := s.store.DeleteShare(c.Request().Context(), id); err != nil {
		return errorJSON(c, http.StatusInternalServerError, err.Error())
	}
	return c.NoContent(http.StatusNoContent)
}
