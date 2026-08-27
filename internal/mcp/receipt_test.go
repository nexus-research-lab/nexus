package mcp

import (
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestReceiptGoalProgressRequiresAppliedTypedMutationAndExactBinding(t *testing.T) {
	tests := []struct {
		name    string
		receipt CommandReceipt
		want    bool
	}{
		{name: "Goal applied", receipt: CommandReceipt{Domain: CommandDomainGoal, Operation: GoalOperationRetarget, Outcome: string(protocol.MutationResultApplied)}, want: true},
		{name: "Goal missing outcome", receipt: CommandReceipt{Domain: CommandDomainGoal, Operation: GoalOperationRetarget}},
		{name: "Goal rejected", receipt: CommandReceipt{Domain: CommandDomainGoal, Operation: GoalOperationUpdate, Outcome: string(protocol.MutationResultRejected)}},
		{name: "Execution applied and bound", receipt: CommandReceipt{Domain: CommandDomainExecution, Operation: "submit_work", Outcome: string(protocol.MutationResultApplied), GoalBound: true}, want: true},
		{name: "Execution applied but unbound", receipt: CommandReceipt{Domain: CommandDomainExecution, Operation: "submit_work", Outcome: string(protocol.MutationResultApplied)}},
		{name: "Execution prepare is read only", receipt: CommandReceipt{Domain: CommandDomainExecution, Operation: "prepare_plan_execution", Outcome: string(protocol.MutationResultApplied), GoalBound: true}},
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
	receipts := []CommandReceipt{{
		Domain: CommandDomainGoal, Operation: GoalOperationUpdate,
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
	state := NewCommandReceiptState()
	first := state.Record(CommandReceipt{Domain: CommandDomainGoal, Operation: GoalOperationCreate})
	second := state.Record(CommandReceipt{Domain: CommandDomainExecution, Operation: ExecutionOperationPlan})
	if first.Sequence != 1 || second.Sequence != 2 {
		t.Fatalf("sequences = %d, %d", first.Sequence, second.Sequence)
	}
	receipts, sequence := state.Since(1)
	if sequence != 2 || len(receipts) != 1 || receipts[0].Sequence != 2 {
		t.Fatalf("Since(1) = %+v sequence=%d", receipts, sequence)
	}
	if !HasDomain(receipts, CommandDomainExecution) || HasDomain(receipts, CommandDomainGoal) {
		t.Fatalf("domain classification failed for %+v", receipts)
	}
}
