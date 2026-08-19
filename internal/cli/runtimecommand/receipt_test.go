package runtimecommand

import (
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestReceiptGoalProgressRequiresAppliedTypedMutationAndExactBinding(t *testing.T) {
	tests := []struct {
		name    string
		receipt Receipt
		want    bool
	}{
		{name: "Goal applied", receipt: Receipt{Domain: DomainGoal, Operation: GoalOperationRetarget, Outcome: string(protocol.MutationResultApplied)}, want: true},
		{name: "Goal missing outcome", receipt: Receipt{Domain: DomainGoal, Operation: GoalOperationRetarget}},
		{name: "Goal rejected", receipt: Receipt{Domain: DomainGoal, Operation: GoalOperationUpdate, Outcome: string(protocol.MutationResultRejected)}},
		{name: "Execution applied and bound", receipt: Receipt{Domain: DomainExecution, Operation: "submit_work", Outcome: string(protocol.MutationResultApplied), GoalBound: true}, want: true},
		{name: "Execution applied but unbound", receipt: Receipt{Domain: DomainExecution, Operation: "submit_work", Outcome: string(protocol.MutationResultApplied)}},
		{name: "Execution prepare is read only", receipt: Receipt{Domain: DomainExecution, Operation: "prepare_plan_execution", Outcome: string(protocol.MutationResultApplied), GoalBound: true}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.receipt.CountsAsGoalProgress(); got != test.want {
				t.Fatalf("CountsAsGoalProgress() = %t, want %t", got, test.want)
			}
		})
	}
}

func TestSuccessfulGoalCompletionIDUsesReceiptIdentityAndFailsClosedOnConflict(t *testing.T) {
	receipts := []Receipt{{
		Domain: DomainGoal, Operation: GoalOperationUpdate,
		Outcome: string(protocol.MutationResultApplied),
		GoalID:  "goal-receipt", GoalStatus: string(protocol.GoalStatusComplete),
	}}
	if got := SuccessfulGoalCompletionID(receipts, ""); got != "goal-receipt" {
		t.Fatalf("completion ID = %q", got)
	}
	if got := SuccessfulGoalCompletionID(receipts, "goal-bound"); got != "" {
		t.Fatalf("conflicting completion ID = %q, want empty", got)
	}
	receipts[0].GoalStatus = string(protocol.GoalStatusBlocked)
	if got := SuccessfulGoalCompletionID(receipts, "goal-receipt"); got != "" {
		t.Fatalf("blocked completion ID = %q, want empty", got)
	}
}

func TestReceiptStateIsMonotonic(t *testing.T) {
	state := NewReceiptState()
	first := state.Record(Receipt{Domain: DomainGoal, Operation: GoalOperationCreate})
	second := state.Record(Receipt{Domain: DomainExecution, Operation: ExecutionOperationPlan})
	if first.Sequence != 1 || second.Sequence != 2 {
		t.Fatalf("sequences = %d, %d", first.Sequence, second.Sequence)
	}
	receipts, sequence := state.Since(1)
	if sequence != 2 || len(receipts) != 1 || receipts[0].Sequence != 2 {
		t.Fatalf("Since(1) = %+v sequence=%d", receipts, sequence)
	}
	if !HasDomain(receipts, DomainExecution) || HasDomain(receipts, DomainGoal) {
		t.Fatalf("domain classification failed for %+v", receipts)
	}
}
