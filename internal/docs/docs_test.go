package docs_test

import (
	"os"
	"strings"
	"testing"
)

func TestReadmeDurablePlanContract(t *testing.T) {
	data, err := os.ReadFile("../../README.md")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, obsolete := range []string{
		"linkable to multiple issues",
		"`PUT /api/v1/projects/:id/plans/:pid`",
		"`DELETE /api/v1/projects/:id/plans/:pid`",
		"`POST /api/v1/projects/:id/plans/:pid/link`",
		"`DELETE /api/v1/projects/:id/plans/:pid/link/:iid`",
	} {
		if strings.Contains(text, obsolete) {
			t.Errorf("README advertises obsolete plan contract: %s", obsolete)
		}
	}
	for _, required := range []string{
		"Immutable revisions", "epics and milestones", "Archive retains",
		"`POST /api/v1/projects/:id/plans/:pid/revisions`",
		"`PATCH /api/v1/projects/:id/plans/:pid`",
		"`POST /api/v1/projects/:id/issues/:iid/plan-adoption`",
		"`GET /api/v1/projects/:id/issues/:iid/governing-plan`",
		// Inline plans are a separate existing feature, not obsolete durable links.
		"`POST /api/v1/projects/:id/issues/:iid/plan`",
		"`GET /api/v1/projects/:id/issues/:iid/plan/history`",
	} {
		if !strings.Contains(text, required) {
			t.Errorf("README missing plan contract: %s", required)
		}
	}
}
