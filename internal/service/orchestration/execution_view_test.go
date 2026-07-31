package orchestration

import (
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestProjectExecutionViewPreservesResponsibilityAndAcceptanceFlow(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 31, 10, 0, 0, 0, time.UTC)
	started := now.Add(time.Minute)
	snapshot := &protocol.ExecutionSnapshot{
		Execution: protocol.Execution{
			ID:                 "execution-1",
			OwnerUserID:        "owner-1",
			SessionKey:         "session-1",
			ScopeKind:          protocol.ExecutionScopeRoom,
			CoordinatorAgentID: "lead",
			Objective:          "Ship the WorkGraph UI",
			CompletionCriteria: []string{"All required work is accepted"},
			Status:             protocol.ExecutionStatusActive,
			Version:            7,
			CreatedAt:          now,
			UpdatedAt:          now.Add(2 * time.Minute),
		},
		Plan: &protocol.ExecutionPlanRevision{
			ID:        "plan-1",
			Revision:  2,
			Status:    protocol.PlanRevisionStatusActive,
			CreatedAt: now,
		},
		WorkItems: []protocol.WorkItem{
			{ID: "work-a", ExecutionID: "execution-1", LogicalKey: "research", Kind: protocol.WorkItemKindProduce},
			{ID: "work-b", ExecutionID: "execution-1", LogicalKey: "build", Kind: protocol.WorkItemKindProduce},
			{ID: "work-c", ExecutionID: "execution-1", LogicalKey: "integrate", Kind: protocol.WorkItemKindIntegrate},
		},
		WorkItemStates: []protocol.WorkItemState{
			{WorkItemID: "work-a", ExecutionID: "execution-1", CurrentSpecID: "spec-a", Status: protocol.WorkItemStatusOpen, UpdatedAt: now},
			{WorkItemID: "work-b", ExecutionID: "execution-1", CurrentSpecID: "spec-b", Status: protocol.WorkItemStatusOpen, UpdatedAt: now},
			{WorkItemID: "work-c", ExecutionID: "execution-1", CurrentSpecID: "spec-c", Status: protocol.WorkItemStatusOpen, UpdatedAt: now},
		},
		WorkItemSpecs: []protocol.WorkItemSpec{
			{ID: "spec-a", WorkItemID: "work-a", ExecutionID: "execution-1", Subject: "Research", Objective: "Collect facts", Deliverable: "Evidence list", AcceptanceCriteria: []string{"Sources included"}},
			{ID: "spec-b", WorkItemID: "work-b", ExecutionID: "execution-1", Subject: "Build", Objective: "Implement UI", Deliverable: "Working panel", AcceptanceCriteria: []string{"Typecheck passes"}},
			{ID: "spec-c", WorkItemID: "work-c", ExecutionID: "execution-1", Subject: "Integrate", Objective: "Close the flow", Deliverable: "Accepted release", AcceptanceCriteria: []string{"All dependencies accepted"}},
		},
		PlanItems: []protocol.ExecutionPlanItem{
			{PlanID: "plan-1", ExecutionID: "execution-1", WorkItemID: "work-a", SpecID: "spec-a", Required: true, Position: 0},
			{PlanID: "plan-1", ExecutionID: "execution-1", WorkItemID: "work-b", SpecID: "spec-b", Required: true, Position: 1},
			{PlanID: "plan-1", ExecutionID: "execution-1", WorkItemID: "work-c", SpecID: "spec-c", Required: true, Terminal: true, Position: 2},
		},
		Dependencies: []protocol.ExecutionPlanDependency{
			{PlanID: "plan-1", ExecutionID: "execution-1", WorkItemID: "work-b", DependsOnWorkItemID: "work-a", Kind: protocol.WorkDependencyHard},
			{PlanID: "plan-1", ExecutionID: "execution-1", WorkItemID: "work-c", DependsOnWorkItemID: "work-b", Kind: protocol.WorkDependencyHard},
		},
		OutputClaims: []protocol.ExecutionPlanOutputClaim{
			{PlanID: "plan-1", ExecutionID: "execution-1", WorkItemID: "work-b", SpecID: "spec-b", Scope: "dir:web/src/features/execution", Mode: protocol.WorkOutputScopeExclusive},
		},
		Assignments: []protocol.WorkAssignment{
			{ID: "assignment-a", ExecutionID: "execution-1", PlanID: "plan-1", WorkItemID: "work-a", SpecID: "spec-a", OwnerAgentID: "researcher", Status: protocol.WorkAssignmentStatusCompleted},
			{ID: "assignment-b", ExecutionID: "execution-1", PlanID: "plan-1", WorkItemID: "work-b", SpecID: "spec-b", OwnerAgentID: "builder", Status: protocol.WorkAssignmentStatusActive, Strategy: protocol.AssignmentStrategyRoomMember},
		},
		Attempts: []protocol.WorkAttempt{
			{ID: "attempt-root", AssignmentID: "assignment-b", ExecutionID: "execution-1", PlanID: "plan-1", WorkItemID: "work-b", SpecID: "spec-b", ExecutorKind: protocol.AttemptExecutorAgent, ExecutorAgentID: "builder", Status: protocol.WorkAttemptStatusRunning, CreatedAt: now, StartedAt: &started},
			{ID: "attempt-child", AssignmentID: "assignment-b", ExecutionID: "execution-1", PlanID: "plan-1", WorkItemID: "work-b", SpecID: "spec-b", ParentAttemptID: "attempt-root", ExecutorKind: protocol.AttemptExecutorSubagent, ParentAgentID: "builder", Status: protocol.WorkAttemptStatusRunning, CreatedAt: started},
		},
		Submissions: []protocol.WorkSubmission{
			{ID: "submission-a", ExecutionID: "execution-1", PlanID: "plan-1", WorkItemID: "work-a", SpecID: "spec-a", AssignmentID: "assignment-a", Sequence: 1, SubmitterAgentID: "researcher", ResultSummary: "Evidence collected", Evidence: []string{"report.md"}, CreatedAt: now},
		},
		Acceptances: []protocol.WorkAcceptance{
			{ID: "acceptance-a", ExecutionID: "execution-1", PlanID: "plan-1", WorkItemID: "work-a", SpecID: "spec-a", AssignmentID: "assignment-a", SubmissionID: "submission-a", Decision: protocol.WorkAcceptanceAccepted, ReviewerKind: protocol.WorkReviewerAgent, ReviewerID: "lead", CreatedAt: now},
		},
	}

	view := ProjectExecutionView(snapshot)
	if view == nil {
		t.Fatal("expected Execution view")
	}
	if view.Progress.Total != 3 ||
		view.Progress.Accepted != 1 ||
		view.Progress.Running != 1 ||
		view.Progress.Waiting != 1 {
		t.Fatalf("unexpected progress: %+v", view.Progress)
	}
	if len(view.WorkItems) != 3 {
		t.Fatalf("work item count = %d, want 3", len(view.WorkItems))
	}
	if view.WorkItems[0].Status != protocol.ExecutionWorkItemViewAccepted ||
		view.WorkItems[0].Acceptance == nil {
		t.Fatalf("accepted work projection is incomplete: %+v", view.WorkItems[0])
	}
	running := view.WorkItems[1]
	if running.Status != protocol.ExecutionWorkItemViewRunning ||
		running.OwnerAgentID != "builder" ||
		len(running.Attempts) != 2 ||
		len(running.OutputScopes) != 1 {
		t.Fatalf("running work projection is incomplete: %+v", running)
	}
	waiting := view.WorkItems[2]
	if waiting.Status != protocol.ExecutionWorkItemViewWaiting ||
		len(waiting.DependencyIDs) != 1 ||
		waiting.DependencyIDs[0] != "work-b" {
		t.Fatalf("dependency projection is incomplete: %+v", waiting)
	}
}
