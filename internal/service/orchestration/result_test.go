package orchestration

import (
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestMutationResultUsesSnapshotRevisionAndStableReasonCode(t *testing.T) {
	snapshot := &protocol.ExecutionSnapshot{Execution: protocol.Execution{
		ID:      "execution-1",
		Version: 9,
	}}
	result := RejectedResult(
		snapshot,
		&DomainError{
			Code:    ErrorCodeDependencyNotAccepted,
			Message: "accept W1 first",
		},
		[]NextAction{{
			Tool:       "review_work",
			LogicalKey: "W1",
			Reason:     "upstream submission is pending review",
		}},
	)
	if result.Outcome != MutationRejected ||
		result.ReasonCode != ErrorCodeDependencyNotAccepted ||
		result.ExecutionID != "execution-1" ||
		result.SnapshotRevision != 9 ||
		len(result.NextActions) != 1 ||
		result.NextActions[0].Tool != "review_work" {
		t.Fatalf("unexpected mutation result: %+v", result)
	}
}

func TestAppliedResultDeduplicatesChangedIDsWithoutReorderingNextActions(t *testing.T) {
	result := AppliedResult(
		nil,
		[]string{"work-2", "work-1", "work-2", ""},
		[]NextAction{
			{Tool: "submit_work", WorkItemID: "work-2", Reason: "finish assigned work"},
			{Tool: "review_work", WorkItemID: "work-1", Reason: "review upstream first"},
		},
	)
	if len(result.Changed) != 2 || result.Changed[0] != "work-1" || result.Changed[1] != "work-2" {
		t.Fatalf("changed IDs should be deterministic: %+v", result.Changed)
	}
	if len(result.NextActions) != 2 ||
		result.NextActions[0].Tool != "submit_work" ||
		result.NextActions[1].Tool != "review_work" {
		t.Fatalf("next actions must keep service priority: %+v", result.NextActions)
	}
}
