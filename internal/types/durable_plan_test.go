package types //nolint:testpackage // contract assertions pin the package declarations

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// Contract assertions in internal/types/plan_test.go.
var (
	_ string        = Plan{}.ProjectID
	_ int64         = Plan{}.HeadRevision
	_ int64         = PlanRevision{}.ReviewVersion
	_ PlanReference = GoverningPlan{}.Reference
	_ string        = PlanRevisionWithContent{}.Content
)

// Additional contract assertions.
var (
	_ int64                    = Plan{}.FeedbackVersion
	_ int64                    = Issue{}.ContractVersion
	_ *int64                   = PlanComment{}.Revision
	_ int64                    = PlanComment{}.Version
	_ *GoverningPlan           = ExpectedGovernance{}.Governing
	_ *ExpectedGovernance      = ExecutionEvidenceRequest{}.Expected
	_ []ReconciledTask         = PlanAdoptionRequest{}.Tasks
	_ []ReconciliationEdge     = PlanAdoptionRequest{}.Edges
	_ []ReconciledContainerPin = PlanAdoptionRequest{}.ContainerPins
	_ []GoverningPlanContext   = GoverningPlan{}.Context
)

func TestExpectedGovernancePreservesUnlinkedAndLayeredContext(t *testing.T) {
	var missing ExecutionEvidenceRequest
	require.NoError(t, json.Unmarshal([]byte(`{"phase":"build","evidence":"test"}`), &missing))
	require.Nil(t, missing.Expected)
	var unlinked ExecutionEvidenceRequest
	require.NoError(t, json.Unmarshal([]byte(`{
        "expected":{"governing":null,"contract_version":4},"phase":"build","evidence":"test"
    }`), &unlinked))
	require.NotNil(t, unlinked.Expected)
	require.Nil(t, unlinked.Expected.Governing)
	require.Equal(t, int64(4), unlinked.Expected.ContractVersion)
	source := GoverningPlan{
		ContainerID: "epic", ContainerType: TypeEpic,
		Reference: PlanReference{PlanID: "tactical", Revision: 5},
		Context: []GoverningPlanContext{{
			ContainerID: "milestone", ContainerType: TypeMilestone,
			Reference: PlanReference{PlanID: "architecture", Revision: 2},
		}},
	}
	encoded, err := json.Marshal(source)
	require.NoError(t, err)
	var decoded GoverningPlan
	require.NoError(t, json.Unmarshal(encoded, &decoded))
	require.Equal(t, source, decoded)
}

func TestDurableDomainJSONFields(t *testing.T) {
	revision := int64(2)
	pin := &PlanReference{PlanID: "plan", Revision: revision}
	governance := &GoverningPlan{ContainerID: "epic", ContainerType: TypeEpic, Reference: *pin}
	source := IssueDetails{Issue: Issue{ContractVersion: 4, GoverningPlan: pin}, ResolvedGovernance: governance}
	encoded, err := json.Marshal(source)
	require.NoError(t, err)
	var decoded IssueDetails
	require.NoError(t, json.Unmarshal(encoded, &decoded))
	require.Equal(t, source, decoded)
	require.Contains(t, string(encoded), `"contract_version":4`)
	require.Contains(t, string(encoded), `"governing_plan":{"plan_id":"plan","revision":2}`)
	require.Contains(t, string(encoded), `"resolved_governance":`)

	encoded, err = json.Marshal(Project{GovernanceGeneration: 3})
	require.NoError(t, err)
	require.Contains(t, string(encoded), `"governance_generation":3`)
	var project Project
	require.NoError(t, json.Unmarshal(encoded, &project))
	require.Equal(t, int64(3), project.GovernanceGeneration)

	comment := PlanComment{Revision: &revision, Version: 5, DeletedAt: &project.CreatedAt}
	encoded, err = json.Marshal(comment)
	require.NoError(t, err)
	require.Contains(t, string(encoded), `"revision":2`)
	require.Contains(t, string(encoded), `"version":5`)
	require.Contains(t, string(encoded), `"deleted_at":`)
	var decodedComment PlanComment
	require.NoError(t, json.Unmarshal(encoded, &decodedComment))
	require.Equal(t, comment, decodedComment)
	encoded, err = json.Marshal(PlanComment{})
	require.NoError(t, err)
	require.Contains(t, string(encoded), `"revision":null`)
	require.NotContains(t, string(encoded), `"deleted_at"`)
}

func TestNullExecutionExpectationIsMissing(t *testing.T) {
	var request ExecutionEvidenceRequest
	require.NoError(t, json.Unmarshal([]byte(`{"expected":null,"phase":"build","evidence":"test"}`), &request))
	require.Nil(t, request.Expected)
}
