// Durable plan handlers keep request presence validation separate from public values.
// Storage owns transaction boundaries, publication order, and project isolation.
// These adapters expose that contract without reading any client-selected path.
package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"unicode/utf8"

	"github.com/labstack/echo/v4"
	"github.com/sentiolabs/arc/internal/planfiles"
	"github.com/sentiolabs/arc/internal/storage"
	"github.com/sentiolabs/arc/internal/types"
)

const planContentField = "content"

const planUpgradeMessage = "upgrade the Arc CLI/server together: path-based plans were removed; " +
	"upload content to /api/v1/projects/{projectId}/plans with Idempotency-Key; " +
	"migrate retained legacy plans with arc server plans migrate; use archive instead of deletion"

// legacyPlanUpgrade rejects every global plan operation before binding request bodies.
// Local paths are neither read nor normalized on this route.
func (s *Server) legacyPlanUpgrade(c echo.Context) error {
	return errorJSON(c, http.StatusBadRequest, planUpgradeMessage)
}

// planError translates typed service failures into the coordinated HTTP contract.
// Integrity failures deliberately omit private server storage paths.
func planError(c echo.Context, err error) error {
	code := http.StatusInternalServerError
	var integrity *planfiles.IntegrityError
	switch {
	case errors.Is(err, storage.ErrPlanNotFound):
		code = http.StatusNotFound
	case errors.Is(err, storage.ErrPlanConflict), errors.Is(err, storage.ErrUnresolvedFeedback):
		code = http.StatusConflict
	case errors.Is(err, storage.ErrPlanPrecondition):
		code = http.StatusPreconditionRequired
	case errors.Is(err, storage.ErrPlanInvalid):
		code = http.StatusBadRequest
	case errors.As(err, &integrity):
		return errorJSON(
			c,
			http.StatusServiceUnavailable,
			"retained plan content failed integrity verification",
		)
	}
	return errorJSON(c, code, err.Error())
}

// Decode required presence separately from the exact public concurrency types.
// A zero version is valid; absent and explicit null both fail a precondition.
func decodePlanBody(c echo.Context, target any, required, preconditions []string) error {
	const maxBody = 6*planfiles.MaxContentBytes + 4096 // JSON escaping can expand exact upload bytes.
	body, err := io.ReadAll(io.LimitReader(c.Request().Body, maxBody+1))
	if err != nil {
		return fmt.Errorf("%w: cannot read body", storage.ErrPlanInvalid)
	}
	if len(body) > maxBody {
		return fmt.Errorf("%w: request too large", storage.ErrPlanInvalid)
	}
	if !validPlanJSONUnicode(body) {
		return fmt.Errorf(
			"%w: JSON must contain valid Unicode without replacement",
			storage.ErrPlanInvalid,
		)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil || fields == nil {
		return fmt.Errorf("%w: expected JSON object", storage.ErrPlanInvalid)
	}
	if _, ok := fields["file_path"]; ok {
		return fmt.Errorf("%w: %s", storage.ErrPlanInvalid, planUpgradeMessage)
	}
	for _, key := range preconditions {
		raw, ok := fields[key]
		if !ok || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			return fmt.Errorf("%w: %s", storage.ErrPlanPrecondition, key)
		}
	}
	for _, key := range required {
		raw, ok := fields[key]
		if !ok || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			return fmt.Errorf("%w: %s is required", storage.ErrPlanInvalid, key)
		}
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("%w: %w", storage.ErrPlanInvalid, err)
	}
	return nil
}

// planPage enforces one bounded pagination policy for all retained history.
// The service independently validates these bounds for non-HTTP callers.
func planPage(c echo.Context) (limit, offset int, err error) {
	limit = 50
	if value := c.QueryParam("limit"); value != "" {
		limit, err = strconv.Atoi(value)
		if err != nil {
			return 0, 0, storage.ErrPlanInvalid
		}
	}
	if value := c.QueryParam("offset"); value != "" {
		offset, err = strconv.Atoi(value)
		if err != nil {
			return 0, 0, storage.ErrPlanInvalid
		}
	}
	if limit < 1 || limit > 200 || offset < 0 {
		return 0, 0, storage.ErrPlanInvalid
	}
	return limit, offset, nil
}

// planRevisionParam requires an explicit positive revision for review operations.
// It never substitutes the current head from a mutable server read.
func planRevisionParam(c echo.Context) (int64, error) {
	value, err := strconv.ParseInt(c.Param("revision"), 10, 64)
	if err != nil || value < 1 {
		return 0, storage.ErrPlanInvalid
	}
	return value, nil
}

// createDurablePlan uploads exact content with an independently scoped request key.
// Replay responses carry the originally committed metadata and a replay marker.
func (s *Server) createDurablePlan(c echo.Context) error {
	var req storage.PlanUpload
	if err := decodePlanBody(c, &req, []string{planContentField}, nil); err != nil {
		return planError(c, err)
	}
	result, err := s.store.CreateDurablePlan(
		c.Request().Context(),
		c.Param("projectId"),
		c.Request().Header.Get("Idempotency-Key"),
		req,
	)
	if err != nil {
		return planError(c, err)
	}
	if result.Replay {
		return successJSON(c, result)
	}
	return createdJSON(c, result)
}

// getDurablePlan returns metadata including the current head and concurrency versions.
// Consumers combine this with an exact revision read when deriving supersession.
func (s *Server) getDurablePlan(c echo.Context) error {
	result, err := s.store.GetDurablePlan(
		c.Request().Context(),
		c.Param("projectId"),
		c.Param("planId"),
	)
	if err != nil {
		return planError(c, err)
	}
	return successJSON(c, result)
}

// listDurablePlans selects active or archived retained artifacts in one project.
// Archived artifacts remain readable and continue to own their history.
func (s *Server) listDurablePlans(c echo.Context) error {
	limit, offset, err := planPage(c)
	if err != nil {
		return planError(c, err)
	}
	result, err := s.store.ListDurablePlans(
		c.Request().Context(),
		c.Param("projectId"),
		c.QueryParam("archived") == "true",
		limit,
		offset,
	)
	if err != nil {
		return planError(c, err)
	}
	return successJSON(c, result)
}

// savePlanRevision requires the head actually used by the editing client.
// A replay can succeed even when later revisions or archive changed that head.
func (s *Server) savePlanRevision(c echo.Context) error {
	var req storage.PlanSave
	if err := decodePlanBody(c, &req, []string{planContentField}, []string{"expected_revision"}); err != nil {
		return planError(c, err)
	}
	result, err := s.store.SavePlanRevision(
		c.Request().Context(),
		c.Param("projectId"),
		c.Param("planId"),
		c.Request().Header.Get("Idempotency-Key"),
		req,
	)
	if err != nil {
		return planError(c, err)
	}
	if result.Replay {
		return successJSON(c, result)
	}
	return createdJSON(c, result)
}

// readPlanRevision returns verified retained bytes, never a client filesystem path.
// Missing or modified content surfaces as a typed integrity failure.
func (s *Server) readPlanRevision(c echo.Context) error {
	revision, err := planRevisionParam(c)
	if err != nil {
		return planError(c, err)
	}
	result, err := s.store.ReadPlanRevision(
		c.Request().Context(),
		c.Param("projectId"),
		c.Param("planId"),
		revision,
	)
	if err != nil {
		return planError(c, err)
	}
	return successJSON(c, result)
}

// listPlanRevisions exposes exact historical review state without rewriting it.
// Pagination is descending so callers can inspect the latest revisions first.
func (s *Server) listPlanRevisions(c echo.Context) error {
	limit, offset, err := planPage(c)
	if err != nil {
		return planError(c, err)
	}
	result, err := s.store.ListPlanRevisions(
		c.Request().Context(),
		c.Param("projectId"),
		c.Param("planId"),
		limit,
		offset,
	)
	if err != nil {
		return planError(c, err)
	}
	return successJSON(c, result)
}

// updateDurablePlan supports title edits and reversible archive/restore.
// Its metadata version is separate from content and feedback version counters.
func (s *Server) updateDurablePlan(c echo.Context) error {
	var req storage.PlanMetadataUpdate
	if err := decodePlanBody(c, &req, nil, []string{"expected_version"}); err != nil {
		return planError(c, err)
	}
	result, err := s.store.UpdateDurablePlan(
		c.Request().Context(),
		c.Param("projectId"),
		c.Param("planId"),
		req,
	)
	if err != nil {
		return planError(c, err)
	}
	return successJSON(c, result)
}

// decidePlanRevision checks presence before decoding the exact public review shape.
// Explicit zero feedback/review versions are valid initial preconditions.
func (s *Server) decidePlanRevision(c echo.Context) error {
	revision, err := planRevisionParam(c)
	if err != nil {
		return planError(c, err)
	}
	var req types.PlanReviewRequest
	if err := decodePlanBody(
		c,
		&req,
		[]string{"status"},
		[]string{"expected_head", "expected_review_version", "expected_feedback_version"},
	); err != nil {
		return planError(c, err)
	}
	if req.ExpectedHead < 1 || req.ExpectedReviewVersion < 0 || req.ExpectedFeedbackVersion < 0 {
		return planError(c, storage.ErrPlanInvalid)
	}
	result, err := s.store.DecidePlanRevision(
		c.Request().Context(),
		c.Param("projectId"),
		c.Param("planId"),
		revision,
		req,
	)
	if err != nil {
		return planError(c, err)
	}
	return successJSON(c, result)
}

// createRevisionComment binds new feedback to its original revision and anchor.
// The service increments feedback version in the same transaction.
func (s *Server) createRevisionComment(c echo.Context) error {
	revision, err := planRevisionParam(c)
	if err != nil {
		return planError(c, err)
	}
	var req storage.PlanCommentCreate
	if err := decodePlanBody(c, &req, []string{planContentField}, nil); err != nil {
		return planError(c, err)
	}
	result, err := s.store.CreateRevisionComment(
		c.Request().Context(),
		c.Param("projectId"),
		c.Param("planId"),
		revision,
		req,
	)
	if err != nil {
		return planError(c, err)
	}
	return createdJSON(c, result)
}

// updateRevisionComment preserves history through versioned edits and tombstones.
// Generic resolution flags cannot bypass an addressed disposition with a reason.
func (s *Server) updateRevisionComment(c echo.Context) error {
	revision, err := planRevisionParam(c)
	if err != nil {
		return planError(c, err)
	}
	var req storage.PlanCommentUpdate
	if err := decodePlanBody(c, &req, nil, []string{"expected_version"}); err != nil {
		return planError(c, err)
	}
	req.Delete = c.Request().Method == http.MethodDelete
	result, err := s.store.UpdateRevisionComment(
		c.Request().Context(),
		c.Param("projectId"),
		c.Param("planId"),
		revision,
		c.Param("commentId"),
		req,
	)
	if err != nil {
		return planError(c, err)
	}
	return successJSON(c, result)
}

// listRevisionComments optionally includes prior feedback with original provenance.
// No anchor is silently moved to the current revision.
func (s *Server) listRevisionComments(c echo.Context) error {
	revision, err := planRevisionParam(c)
	if err != nil {
		return planError(c, err)
	}
	limit, offset, err := planPage(c)
	if err != nil {
		return planError(c, err)
	}
	result, err := s.store.ListRevisionComments(
		c.Request().Context(),
		c.Param("projectId"),
		c.Param("planId"),
		revision,
		c.QueryParam("include_prior") == "true",
		limit,
		offset,
	)
	if err != nil {
		return planError(c, err)
	}
	return successJSON(c, result)
}

// addPlanDisposition binds a reason to an exact comment version and review target.
// Its feedback precondition invalidates stale approval contexts.
func (s *Server) addPlanDisposition(c echo.Context) error {
	revision, err := planRevisionParam(c)
	if err != nil {
		return planError(c, err)
	}
	var req storage.PlanDispositionRequest
	if err := decodePlanBody(
		c,
		&req,
		[]string{"comment_id", "disposition", "reason"},
		[]string{"expected_comment_version", "expected_feedback_version"},
	); err != nil {
		return planError(c, err)
	}
	result, err := s.store.AddPlanDisposition(
		c.Request().Context(),
		c.Param("projectId"),
		c.Param("planId"),
		revision,
		req,
	)
	if err != nil {
		return planError(c, err)
	}
	return createdJSON(c, result)
}

// listPlanDispositions exposes append-only history through the chosen revision.
// Clients can distinguish addressed carry-forward from target-only deferrals.
func (s *Server) listPlanDispositions(c echo.Context) error {
	revision, err := planRevisionParam(c)
	if err != nil {
		return planError(c, err)
	}
	limit, offset, err := planPage(c)
	if err != nil {
		return planError(c, err)
	}
	result, err := s.store.ListPlanDispositions(
		c.Request().Context(),
		c.Param("projectId"),
		c.Param("planId"),
		revision,
		limit,
		offset,
	)
	if err != nil {
		return planError(c, err)
	}
	return successJSON(c, result)
}

// validPlanJSONUnicode prevents encoding/json from silently replacing malformed
// UTF-8 or unpaired UTF-16 escapes. Valid surrogate pairs decode to exact UTF-8.
// Escaped backslashes are skipped so a literal "\\uD800" remains ordinary text.
func validPlanJSONUnicode(body []byte) bool {
	if !utf8.Valid(body) {
		return false
	}
	for i := 0; i < len(body); i++ {
		if body[i] != '\\' {
			continue
		}
		i++
		if i >= len(body) {
			return false
		}
		if body[i] != 'u' {
			continue
		}
		next, ok := planJSONUnicodeEscape(body, i)
		if !ok {
			return false
		}
		i = next
	}
	return true
}

// planJSONUnicodeEscape validates one UTF-16 code unit or a paired surrogate.
// The returned index points to the last consumed hex digit for the outer scan.
func planJSONUnicodeEscape(body []byte, offset int) (int, bool) {
	const hexDigits = 4
	const escapedUnitBytes = 6
	if offset+4 >= len(body) {
		return 0, false
	}
	value, err := strconv.ParseUint(string(body[offset+1:offset+5]), 16, 16)
	if err != nil || value >= 0xdc00 && value <= 0xdfff {
		return 0, false
	}
	last := offset + hexDigits
	if value < 0xd800 || value > 0xdbff {
		return last, true
	}
	if last+6 >= len(body) || body[last+1] != '\\' || body[last+2] != 'u' {
		return 0, false
	}
	low, err := strconv.ParseUint(string(body[last+3:last+7]), 16, 16)
	return last + escapedUnitBytes, err == nil && low >= 0xdc00 && low <= 0xdfff
}
