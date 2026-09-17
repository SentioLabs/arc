package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/sentiolabs/arc/internal/storage"
	"github.com/sentiolabs/arc/internal/types"
)

// Captured contexts are never filled from a completion-time server read.
// decodeExecutionExpected preserves missing/null versus explicit unlinked state.
func decodeExecutionExpected(raw json.RawMessage) (*types.ExpectedGovernance, error) {
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil, nil //nolint:nilnil // Storage distinguishes legacy omission from governed work.
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil || fields == nil {
		return nil, storage.ErrPlanInvalid
	}
	if _, ok := fields["governing"]; !ok {
		return nil, storage.ErrExecutionPrecondition
	}
	version, ok := fields["contract_version"]
	if !ok || bytes.Equal(bytes.TrimSpace(version), []byte("null")) {
		return nil, storage.ErrExecutionPrecondition
	}
	var expected types.ExpectedGovernance
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&expected); err != nil {
		return nil, storage.ErrPlanInvalid
	}
	if expected.ContractVersion < 1 {
		return nil, storage.ErrExecutionPrecondition
	}
	return &expected, nil
}

// recordExecutionEvidence keeps captured expectations separate from fresh issue reads.
// The storage transaction checks both the complete chain and contract version.
// Successful phase evidence does not itself change the execution contract.
func (s *Server) recordExecutionEvidence(c echo.Context) error {
	var body struct {
		Expected json.RawMessage `json:"expected"`
		Phase    string          `json:"phase"`
		Evidence string          `json:"evidence"`
	}
	if err := decodePlanBody(c, &body, []string{"phase", evidenceField}, nil); err != nil {
		return planError(c, err)
	}
	expected, err := decodeExecutionExpected(body.Expected)
	if err != nil {
		return issueMutationError(c, err)
	}
	result, err := s.store.RecordExecutionEvidence(
		c.Request().Context(),
		projectID(c),
		c.Param("id"),
		types.ExecutionEvidenceRequest{
			Expected: expected,
			Phase:    body.Phase,
			Evidence: body.Evidence,
		},
	)
	if err != nil {
		return issueMutationError(c, err)
	}
	return c.JSON(http.StatusCreated, result)
}

// listExecutionEvidence returns bounded immutable capture history.
// Adoption changes inheritance but never rewrites earlier phase provenance.
func (s *Server) listExecutionEvidence(c echo.Context) error {
	limit, offset, err := planPage(c)
	if err != nil {
		return planError(c, err)
	}
	result, err := s.store.ListExecutionEvidence(
		c.Request().Context(),
		projectID(c),
		c.Param("id"),
		limit,
		offset,
	)
	if err != nil {
		return issueMutationError(c, err)
	}
	return successJSON(c, result)
}

// requiredNullable distinguishes a missing pin key from an explicit detach.
// Both attachment and detachment must state the expected and proposed pin.
func requiredNullable(fields map[string]json.RawMessage, key string) error {
	if _, ok := fields[key]; !ok {
		return fmt.Errorf("%w: %s", storage.ErrPlanPrecondition, key)
	}
	return nil
}
