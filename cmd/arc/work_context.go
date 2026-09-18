// Work artifacts contain complete expectations, including explicit unlinked state.
// Presence checks precede typed JSON decoding so omission cannot become a valid
// zero value. Callers retain this artifact for every later execution phase.
// A missing object must never be confused with governing:null.
// Writes refuse to replace an existing capture with a different contract.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/sentiolabs/arc/internal/client"
	"github.com/sentiolabs/arc/internal/types"
)

// requiredJSONFields rejects missing/null fields before Go's zero-value decoding.
func requiredJSONFields(b []byte, keys ...string) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	if raw == nil {
		return errors.New("expected a JSON object")
	}
	for _, key := range keys {
		value, ok := raw[key]
		if !ok || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return fmt.Errorf("context requires %s", key)
		}
	}
	return nil
}

// readWorkContext loads the original artifact supplied by the worker.
// Completion never falls back to a server read if this file is unavailable.
func readWorkContext(path string) (*types.ExpectedGovernance, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return decodeWorkContext(b)
}

// decodeWorkContext preserves explicit unlinked state and the full layered chain.
// Unknown fields and malformed source identities fail before a mutation request.
func decodeWorkContext(b []byte) (*types.ExpectedGovernance, error) {
	if err := requiredJSONFields(b, "contract_version"); err != nil {
		return nil, err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, err
	}
	governing, ok := raw["governing"]
	if !ok {
		return nil, errors.New("context requires governing (explicit null means unlinked)")
	}
	if !bytes.Equal(bytes.TrimSpace(governing), []byte("null")) {
		if err := validateGoverningJSON(governing, true); err != nil {
			return nil, err
		}
	}
	var expected types.ExpectedGovernance
	decoder := json.NewDecoder(bytes.NewReader(b))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&expected); err != nil {
		return nil, err
	}
	if expected.ContractVersion < 1 {
		return nil, errors.New("context requires a positive contract_version")
	}
	return &expected, nil
}

// validateGoverningJSON checks every required source and reference field.
// Even an empty higher-level chain must be explicitly captured in context.
func validateGoverningJSON(b []byte, primary bool) error {
	if err := requiredJSONFields(b, "container_id", "container_type", "reference"); err != nil {
		return err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	if err := requiredJSONFields(raw["reference"], "plan_id", "revision"); err != nil {
		return err
	}
	if primary {
		// Empty context may be null, but the field must explicitly name the full chain.
		context, ok := raw["context"]
		if !ok {
			return errors.New("governing context field is required")
		}
		var entries []json.RawMessage
		if err := json.Unmarshal(context, &entries); err != nil {
			return err
		}
		for _, entry := range entries {
			if err := validateGoverningJSON(entry, false); err != nil {
				return err
			}
		}
	}
	var source types.GoverningPlanContext
	if err := json.Unmarshal(b, &source); err != nil {
		return err
	}
	if source.ContainerID == "" || source.Reference.PlanID == "" || source.Reference.Revision < 1 ||
		(source.ContainerType != types.TypeEpic && source.ContainerType != types.TypeMilestone) {
		return errors.New("invalid governing source identity")
	}
	return nil
}

// writeContextJSON publishes a complete artifact with atomic no-clobber semantics.
// An existing capture is never overwritten with a newer execution contract.
func writeContextJSON(path string, value any) error {
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return writePlanExport(path, append(b, '\n'), false)
}

// Captures use only the atomic update response; a later GET cannot replace it.
func captureWorkContext(
	details *types.IssueDetails,
	path string,
) (types.ExpectedGovernance, error) {
	expected := types.ExpectedGovernance{
		ContractVersion: details.ContractVersion,
		Governing:       details.ResolvedGovernance,
	}
	if path != "" {
		if err := writeContextJSON(path, expected); err != nil {
			return expected, fmt.Errorf(
				"issue %s updated; captured context could not be written: %w",
				details.ID,
				err,
			)
		}
	}
	return expected, nil
}

// printGoverningSources displays outer designs before the primary tactical source.
// Titles and lifecycle come from metadata; approval comes from the pinned revision.
func printGoverningSources(
	c *client.Client,
	projectID string,
	governing *types.GoverningPlan,
) error {
	if governing == nil {
		fmt.Println("Governing plan: unlinked")
		return nil
	}
	sources := append([]types.GoverningPlanContext(nil), governing.Context...)
	sources = append(
		sources,
		types.GoverningPlanContext{
			ContainerID:   governing.ContainerID,
			ContainerType: governing.ContainerType,
			Reference:     governing.Reference,
		},
	)
	for _, source := range sources {
		plan, err := c.GetPlan(projectID, source.Reference.PlanID)
		if err != nil {
			return err
		}
		revision, err := c.ReadPlanRevision(projectID, plan.ID, source.Reference.Revision)
		if err != nil {
			return err
		}
		fmt.Print(formatPlanInfo(plan, revision, source))
	}
	return nil
}

// printWorkContext renders the captured chain without fetching current metadata.
// Historical evidence stays readable even after a different revision is adopted.
func printWorkContext(expected types.ExpectedGovernance) {
	fmt.Printf("Work contract version %d\n", expected.ContractVersion)
	if expected.Governing == nil {
		fmt.Println("Governing plan: unlinked")
		return
	}
	for _, source := range expected.Governing.Context {
		fmt.Printf(
			"Higher-level %s %s: %s revision %d\n",
			source.ContainerType,
			source.ContainerID,
			source.Reference.PlanID,
			source.Reference.Revision,
		)
	}
	source := expected.Governing
	fmt.Printf(
		"Primary %s %s: %s revision %d\n",
		source.ContainerType,
		source.ContainerID,
		source.Reference.PlanID,
		source.Reference.Revision,
	)
}

// writeDurableResult retains complete nested records in a readable, reusable form.
// These durable workflow results use indented JSON in both output modes; unrelated
// CLI commands keep their existing global formatting behavior.
func writeDurableResult(value any) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}
