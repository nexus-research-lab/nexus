package goal

import (
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestRuntimeUsageAccumulatorAddsDistinctTurnsAndReconcilesFinal(t *testing.T) {
	accumulator := NewRuntimeUsageAccumulator(true)

	first, ok := accumulator.Delta(runtimeTurnSnapshot("turn-a", 90, 10))
	if ok {
		t.Fatalf("first delta = %#v, ok = true, want all token fields deferred", first)
	}
	if duplicate, ok := accumulator.Delta(runtimeTurnSnapshot("turn-a", 90, 10)); ok {
		t.Fatalf("duplicate turn delta = %#v, want none", duplicate)
	}
	second, ok := accumulator.Delta(runtimeTurnSnapshot("turn-b", 70, 10))
	if ok {
		t.Fatalf("second delta = %#v, ok = true, want all token fields deferred", second)
	}
	final, ok := accumulator.Delta(RuntimeUsageSnapshot{
		Usage:      protocol.GoalUsage{InputTokens: 160, OutputTokens: 20, ActualTotalTokens: 180},
		Cumulative: true,
		Terminal:   true,
	})
	if !ok || final.ActualTokens() != 180 || final.BudgetTokens() != 180 {
		t.Fatalf("reconciled final delta = %#v, ok = %v, want complete terminal 180", final, ok)
	}
}

func TestRuntimeUsageAccumulatorModelAndExternalActivationUseDifferentBaselines(t *testing.T) {
	modelCreated := NewRuntimeUsageAccumulator(false)
	if delta, ok := modelCreated.Delta(runtimeTurnSnapshot("turn-a", 90, 10)); ok {
		t.Fatalf("inactive delta = %#v, want none", delta)
	}
	backlog, ok := modelCreated.ActivateFromRoundStart()
	if ok {
		t.Fatalf("model-created backlog = %#v, ok = true, want tokens deferred", backlog)
	}
	next, ok := modelCreated.Delta(runtimeTurnSnapshot("turn-b", 70, 10))
	if ok {
		t.Fatalf("model-created next delta = %#v, ok = true, want tokens deferred", next)
	}

	externalCreated := NewRuntimeUsageAccumulator(false)
	if delta, ok := externalCreated.Delta(runtimeTurnSnapshot("turn-a", 90, 10)); ok {
		t.Fatalf("inactive delta = %#v, want none", delta)
	}
	externalCreated.Reset(runtimeTurnSnapshot("turn-a", 90, 10))
	next, ok = externalCreated.Delta(runtimeTurnSnapshot("turn-b", 70, 10))
	if ok {
		t.Fatalf("external-created delta = %#v, ok = true, want tokens deferred", next)
	}
}

func TestRuntimeUsageAccumulatorUnboundTerminalEligibilityIsPristineOnly(t *testing.T) {
	pristine := NewRuntimeUsageAccumulator(false)
	pristine.PrepareDelta(RuntimeUsageSnapshot{TokenUsageObserved: true})
	if !pristine.EligibleForUnboundTerminal() {
		t.Fatal("observed but never-bound accumulator was not eligible for unbound terminal")
	}

	activated := NewRuntimeUsageAccumulator(false)
	activated.PrepareActivationFromRoundStart()
	if activated.EligibleForUnboundTerminal() {
		t.Fatal("activated accumulator remained eligible for unbound terminal")
	}

	reset := NewRuntimeUsageAccumulator(false)
	reset.Reset(RuntimeUsageSnapshot{})
	if reset.EligibleForUnboundTerminal() {
		t.Fatal("Reset accumulator remained eligible for unbound terminal")
	}

	closed := NewRuntimeUsageAccumulator(false)
	closed.Close()
	if closed.EligibleForUnboundTerminal() {
		t.Fatal("closed accumulator remained eligible for unbound terminal")
	}
	if delta, ok := closed.PrepareActivationFromRoundStart(); ok || closed.Active() {
		t.Fatalf("closed accumulator reactivated with delta %#v", delta)
	}

	reset.Close()
	if delta, ok := reset.PrepareActivationFromRoundStart(); ok || reset.Active() {
		t.Fatalf("previously activated accumulator reactivated with delta %#v", delta)
	}
}

func runtimeTurnSnapshot(turnID string, inputTokens int64, outputTokens int64) RuntimeUsageSnapshot {
	return RuntimeUsageSnapshot{
		TurnID: turnID,
		Usage: protocol.GoalUsage{
			InputTokens:       inputTokens,
			OutputTokens:      outputTokens,
			ActualTotalTokens: inputTokens + outputTokens,
		},
	}
}
