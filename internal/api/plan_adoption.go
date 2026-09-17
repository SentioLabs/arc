package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/sentiolabs/arc/internal/storage"
	"github.com/sentiolabs/arc/internal/types"
)

const (
	expectedContainerVersionField = "expected_container_version"
	expectedPinField              = "expected_pin"
	targetPinField                = "target_pin"
	evidenceField                 = "evidence"
	closeReasonField              = "reason"
)

// Nullable pins express attachment and detachment without ambiguous defaults.
// No HTTP handler reserves proposal-local IDs or writes partial reconciliation.
// adoptPlan validates wire presence before passing exact shared values to storage.
// Dry-run and apply use identical staged validation; only apply requires a key.
// A replay is returned from immutable history, not a freshly resolved container.
func (s *Server) adoptPlan(c echo.Context) error {
	var raw map[string]json.RawMessage
	preconditions := []string{expectedContainerVersionField, "expected_governance_generation"}
	if err := decodePlanBody(c, &raw, nil, preconditions); err != nil {
		return planError(c, err)
	}
	for _, key := range []string{expectedPinField, targetPinField} {
		if err := requiredNullable(raw, key); err != nil {
			return planError(c, err)
		}
	}
	body, err := json.Marshal(raw)
	if err != nil {
		return planError(c, err)
	}
	var req types.PlanAdoptionRequest
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		return planError(c, storage.ErrPlanInvalid)
	}
	if err := validateAdoptionPresence(raw); err != nil {
		return adoptionError(c, err)
	}
	result, err := s.store.AdoptPlan(
		c.Request().Context(),
		projectID(c),
		c.Param("id"),
		c.Request().Header.Get("Idempotency-Key"),
		req,
	)
	if err != nil {
		return adoptionError(c, err)
	}
	return c.JSON(http.StatusOK, result)
}

// validateAdoptionPresence checks nested preconditions before typed decoding can
// collapse omitted fields into zero values. Explicit null pins remain valid,
// while a missing task expectation cannot silently become an unlinked context.
func validateAdoptionPresence(raw map[string]json.RawMessage) error {
	var tasks []map[string]json.RawMessage
	if err := json.Unmarshal(rawOrArray(raw["tasks"]), &tasks); err != nil {
		return storage.ErrPlanInvalid
	}
	for _, task := range tasks {
		expected, err := decodeExecutionExpected(task["expected"])
		if err != nil {
			return err
		}
		if expected == nil {
			return storage.ErrPlanPrecondition
		}
	}
	var pins []map[string]json.RawMessage
	if err := json.Unmarshal(rawOrArray(raw["container_pins"]), &pins); err != nil {
		return storage.ErrPlanInvalid
	}
	for _, pin := range pins {
		for _, key := range []string{expectedPinField, targetPinField, expectedContainerVersionField} {
			if err := requiredNullable(pin, key); err != nil {
				return err
			}
		}
		var version int64
		if err := json.Unmarshal(pin[expectedContainerVersionField], &version); err != nil {
			return storage.ErrPlanInvalid
		}
		if version < 1 {
			return storage.ErrPlanPrecondition
		}
	}
	return nil
}

// Empty optional collections are equivalent to an empty proposal list.
func rawOrArray(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage("[]")
	}
	return raw
}

// Adoption retains resolver error codes alongside durable-plan status mapping.
func adoptionError(c echo.Context, err error) error {
	// Governance resolver errors retain their distinct public codes.
	if errors.Is(err, storage.ErrGovernanceCycle) ||
		errors.Is(err, storage.ErrAmbiguousGovernance) ||
		errors.Is(err, storage.ErrGovernanceReconciliation) ||
		errors.Is(err, storage.ErrIssueNotFound) ||
		errors.Is(err, storage.ErrExecutionPrecondition) {
		return issueMutationError(c, err)
	}
	return planError(c, err)
}

// listPlanAdoptions exposes immutable requests, compatibility reasons and
// before/after snapshots with the same bounded pagination as plan history.
func (s *Server) listPlanAdoptions(c echo.Context) error {
	limit, offset, err := planPage(c)
	if err != nil {
		return planError(c, err)
	}
	result, err := s.store.ListPlanAdoptions(
		c.Request().Context(),
		projectID(c),
		c.Param("id"),
		limit,
		offset,
	)
	if err != nil {
		return adoptionError(c, err)
	}
	return successJSON(c, result)
}
