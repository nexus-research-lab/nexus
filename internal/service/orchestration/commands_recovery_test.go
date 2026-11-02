package orchestration

import (
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestNextActionsSuggestTakeoverOnlyForCurrentFailedRoot(t *testing.T) {
	for _, status := range []protocol.WorkAttemptStatus{protocol.WorkAttemptStatusFailed, protocol.WorkAttemptStatusInterrupted, protocol.WorkAttemptStatusRunning, protocol.WorkAttemptStatusSucceeded} {
		snapshot := &protocol.ExecutionSnapshot{
			Execution:      protocol.Execution{CoordinatorAgentID: "lead", Status: protocol.ExecutionStatusActive},
			WorkItems:      []protocol.WorkItem{{ID: "work", LogicalKey: "work"}},
			WorkItemStates: []protocol.WorkItemState{{WorkItemID: "work", CurrentSpecID: "spec", Status: protocol.WorkItemStatusOpen}},
			Assignments:    []protocol.WorkAssignment{{ID: "assignment", WorkItemID: "work", SpecID: "spec", OwnerAgentID: "member", Status: protocol.WorkAssignmentStatusActive}},
			Attempts:       []protocol.WorkAttempt{{ID: "root", AssignmentID: "assignment", Status: status, CreatedAt: time.Unix(1, 0)}},
		}
		hasTakeover := func(actor string) bool {
			for _, action := range nextActions(snapshot, ActorContext{AgentID: actor}) {
				if action.Operation == "take_over_work" {
					return true
				}
			}
			return false
		}
		want := status == protocol.WorkAttemptStatusFailed || status == protocol.WorkAttemptStatusInterrupted
		if got := hasTakeover("lead"); got != want {
			t.Fatalf("status %s: takeover=%v", status, got)
		}
		if hasTakeover("member") {
			t.Fatal("member received coordinator recovery action")
		}
		snapshot.Attempts = append(snapshot.Attempts, protocol.WorkAttempt{AssignmentID: "assignment", Status: protocol.WorkAttemptStatusRunning, CreatedAt: time.Unix(2, 0)})
		if hasTakeover("lead") {
			t.Fatal("old failed Attempt overrides live successor")
		}
	}
}
