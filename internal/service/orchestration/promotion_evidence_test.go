package orchestration

import (
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestAdaptiveEvidenceDoesNotTreatSelfRoomDAGAsDurableDependency(t *testing.T) {
	snapshot := roomPromotionSnapshot()
	snapshot.Dependencies = []protocol.ExecutionPlanDependency{{
		PlanID:              "plan-1",
		ExecutionID:         snapshot.Execution.ID,
		WorkItemID:          "work-verify",
		DependsOnWorkItemID: "work-produce",
		Kind:                protocol.WorkDependencyHard,
	}}
	snapshot.Assignments = []protocol.WorkAssignment{{
		ID:           "assignment-self",
		PlanID:       "plan-1",
		WorkItemID:   "work-produce",
		SpecID:       "spec-produce",
		OwnerAgentID: "agent-lead",
		Status:       protocol.WorkAssignmentStatusAssigned,
	}}

	evidence := adaptiveEvidenceFromSnapshot(snapshot, ActorContext{AgentID: "agent-lead"})
	if evidence.BoundRoomDependency {
		t.Fatalf("self-only Room DAG became durable cross-Agent evidence: %#v", evidence)
	}
}

func TestAdaptiveEvidenceRequiresBoundOtherAgentForRoomSignal(t *testing.T) {
	snapshot := roomPromotionSnapshot()
	snapshot.Assignments = []protocol.WorkAssignment{{
		ID:           "assignment-worker",
		PlanID:       "plan-1",
		WorkItemID:   "work-produce",
		SpecID:       "spec-produce",
		OwnerAgentID: "agent-worker",
		Status:       protocol.WorkAssignmentStatusAssigned,
	}}

	evidence := adaptiveEvidenceFromSnapshot(snapshot, ActorContext{AgentID: "agent-lead"})
	if !evidence.BoundRoomDependency {
		t.Fatalf("bound other-Agent required work did not become durable evidence: %#v", evidence)
	}
}

func TestAdaptiveEvidenceIgnoresOptionalWaitingWork(t *testing.T) {
	snapshot := roomPromotionSnapshot()
	snapshot.WorkItemStates = []protocol.WorkItemState{{
		WorkItemID:    "work-optional",
		CurrentSpecID: "spec-optional",
		Status:        protocol.WorkItemStatusWaitingInput,
	}}

	evidence := adaptiveEvidenceFromSnapshot(snapshot, ActorContext{AgentID: "agent-lead"})
	if evidence.RequiredExternalWait {
		t.Fatalf("optional waiting work became required external wait: %#v", evidence)
	}

	snapshot.WorkItemStates = append(snapshot.WorkItemStates, protocol.WorkItemState{
		WorkItemID:    "work-produce",
		CurrentSpecID: "spec-produce",
		Status:        protocol.WorkItemStatusWaitingInput,
	})
	evidence = adaptiveEvidenceFromSnapshot(snapshot, ActorContext{AgentID: "agent-lead"})
	if !evidence.RequiredExternalWait {
		t.Fatalf("required waiting work did not become durable external wait: %#v", evidence)
	}
}

func roomPromotionSnapshot() *protocol.ExecutionSnapshot {
	return &protocol.ExecutionSnapshot{
		Execution: protocol.Execution{
			ID:                 "execution-1",
			ScopeKind:          protocol.ExecutionScopeRoom,
			CoordinatorAgentID: "agent-lead",
			Objective:          "deliver verified result",
			CompletionCriteria: []string{"verified"},
			Status:             protocol.ExecutionStatusActive,
		},
		Plan: &protocol.ExecutionPlanRevision{
			ID:     "plan-1",
			Status: protocol.PlanRevisionStatusActive,
		},
		PlanItems: []protocol.ExecutionPlanItem{
			{
				PlanID:     "plan-1",
				WorkItemID: "work-produce",
				SpecID:     "spec-produce",
				Required:   true,
			},
			{
				PlanID:     "plan-1",
				WorkItemID: "work-verify",
				SpecID:     "spec-verify",
				Required:   true,
				Terminal:   true,
			},
			{
				PlanID:     "plan-1",
				WorkItemID: "work-optional",
				SpecID:     "spec-optional",
			},
		},
	}
}
