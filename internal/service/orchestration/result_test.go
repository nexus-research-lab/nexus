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
			Domain: "execution", Operation: "review_work",
			LogicalKey: "W1",
			Reason:     "upstream submission is pending review",
		}},
	)
	if result.Outcome != MutationRejected ||
		result.ReasonCode != ErrorCodeDependencyNotAccepted ||
		result.ExecutionID != "execution-1" ||
		result.SnapshotRevision != 9 ||
		len(result.NextActions) != 1 ||
		result.NextActions[0].Operation != "review_work" {
		t.Fatalf("unexpected mutation result: %+v", result)
	}
}

func TestAppliedResultDeduplicatesChangedIDsWithoutReorderingNextActions(t *testing.T) {
	result := AppliedResult(
		nil,
		[]string{"work-2", "work-1", "work-2", ""},
		[]NextAction{
			{Domain: "execution", Operation: "submit_work", WorkItemID: "work-2", Reason: "finish assigned work"},
			{Domain: "execution", Operation: "review_work", WorkItemID: "work-1", Reason: "review upstream first"},
		},
	)
	if len(result.Changed) != 2 || result.Changed[0] != "work-1" || result.Changed[1] != "work-2" {
		t.Fatalf("changed IDs should be deterministic: %+v", result.Changed)
	}
	if len(result.NextActions) != 2 ||
		result.NextActions[0].Operation != "submit_work" ||
		result.NextActions[1].Operation != "review_work" {
		t.Fatalf("next actions must keep service priority: %+v", result.NextActions)
	}
}

func TestSupersededResultKeepsTerminalSnapshotIdentity(t *testing.T) {
	snapshot := &protocol.ExecutionSnapshot{Execution: protocol.Execution{
		ID:      "execution-old",
		Version: 12,
	}}
	result := SupersededResult(snapshot, &DomainError{
		Code:    ErrorCodeExecutionTerminal,
		Message: "the predecessor was replaced",
	})
	if result.Outcome != MutationSuperseded ||
		result.ReasonCode != ErrorCodeExecutionTerminal ||
		result.ExecutionID != "execution-old" ||
		result.SnapshotRevision != 12 {
		t.Fatalf("unexpected superseded result: %+v", result)
	}
}
