// INPUT: equal proposal documents under different authority and target fences.
// OUTPUT: stable immutable digests that change for every security-relevant fence.
// POS: protocol-level commit identity regression tests for sealed ExecutionPlanProposal.
package protocol

import "testing"

func TestDigestExecutionPlanProposalImmutableCoversAuthorityTargetAndGoalFence(t *testing.T) {
	base := ExecutionPlanProposal{
		OwnerUserID:            "owner-1",
		SessionKey:             "session-1",
		ScopeKind:              ExecutionScopeDM,
		CoordinatorAgentID:     "agent-lead",
		RootRoundID:            "root-round-1",
		RuntimeRoundID:         "runtime-round-1",
		AgentRoundID:           "agent-round-1",
		TargetExecutionID:      "execution-1",
		TargetExecutionVersion: 4,
		BasePlanID:             "plan-3",
		GoalID:                 "goal-1",
		GoalObjectiveRevision:  2,
		GoalActivationOrigin:   GoalActivationOriginUserExplicit,
		GoalActivationReason:   GoalActivationReasonPersistenceRequested,
		ReplacesExecutionID:    "execution-0",
		Document: ExecutionPlanProposalDocument{
			Version:            ExecutionPlanProposalDocumentVersion,
			Operation:          ExecutionPlanProposalReplan,
			Objective:          "deliver",
			CompletionCriteria: []string{"verified"},
			RevisionReason:     "new evidence",
			Items: []ExecutionPlanProposalItem{{
				LogicalKey:         "deliver",
				Kind:               WorkItemKindProduce,
				Subject:            "Deliver",
				Objective:          "Produce result",
				Deliverable:        "result",
				AcceptanceCriteria: []string{"verified"},
				Required:           true,
				Terminal:           true,
			}},
		},
	}
	want, err := DigestExecutionPlanProposalImmutable(base)
	if err != nil {
		t.Fatal(err)
	}
	mutations := []struct {
		name   string
		mutate func(*ExecutionPlanProposal)
	}{
		{name: "owner", mutate: func(item *ExecutionPlanProposal) { item.OwnerUserID = "owner-2" }},
		{name: "session", mutate: func(item *ExecutionPlanProposal) { item.SessionKey = "session-2" }},
		{name: "scope", mutate: func(item *ExecutionPlanProposal) {
			item.ScopeKind = ExecutionScopeRoom
			item.RoomID = "room-1"
			item.ConversationID = "conversation-1"
		}},
		{name: "coordinator", mutate: func(item *ExecutionPlanProposal) { item.CoordinatorAgentID = "agent-other" }},
		{name: "root round", mutate: func(item *ExecutionPlanProposal) { item.RootRoundID = "root-round-2" }},
		{name: "runtime round", mutate: func(item *ExecutionPlanProposal) { item.RuntimeRoundID = "runtime-round-2" }},
		{name: "agent round", mutate: func(item *ExecutionPlanProposal) { item.AgentRoundID = "agent-round-2" }},
		{name: "target", mutate: func(item *ExecutionPlanProposal) { item.TargetExecutionID = "execution-2" }},
		{name: "target version", mutate: func(item *ExecutionPlanProposal) { item.TargetExecutionVersion++ }},
		{name: "base plan", mutate: func(item *ExecutionPlanProposal) { item.BasePlanID = "plan-4" }},
		{name: "goal", mutate: func(item *ExecutionPlanProposal) { item.GoalID = "goal-2" }},
		{name: "goal revision", mutate: func(item *ExecutionPlanProposal) { item.GoalObjectiveRevision++ }},
		{name: "goal activation origin", mutate: func(item *ExecutionPlanProposal) {
			item.GoalActivationOrigin = GoalActivationOriginAdaptivePromoted
		}},
		{name: "goal activation reason", mutate: func(item *ExecutionPlanProposal) {
			item.GoalActivationReason = GoalActivationReasonObservedBoundary
		}},
		{name: "goal reserved execution", mutate: func(item *ExecutionPlanProposal) {
			item.GoalReservedExecutionID = "execution-goal-reserved"
		}},
		{name: "replaces execution", mutate: func(item *ExecutionPlanProposal) {
			item.ReplacesExecutionID = "execution-other"
		}},
		{name: "document", mutate: func(item *ExecutionPlanProposal) { item.Document.Objective = "different" }},
	}
	for _, test := range mutations {
		t.Run(test.name, func(t *testing.T) {
			item := base
			test.mutate(&item)
			got, digestErr := DigestExecutionPlanProposalImmutable(item)
			if digestErr != nil {
				t.Fatal(digestErr)
			}
			if got == want {
				t.Fatalf("immutable digest did not change for %s", test.name)
			}
		})
	}

	mutable := base
	mutable.ContentDigest = "ignored"
	mutable.Status = ExecutionPlanProposalStatusMaterialized
	mutable.Version = 99
	mutable.ReservedExecutionID = "execution-1"
	mutable.MaterializedPlanID = "plan-new"
	got, err := DigestExecutionPlanProposalImmutable(mutable)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatal("lifecycle and receipt fields changed immutable digest")
	}
}
