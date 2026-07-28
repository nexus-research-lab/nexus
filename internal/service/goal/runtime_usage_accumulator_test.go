package goal

import (
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestRuntimeUsageAccumulatorDeltasResetAndClose(t *testing.T) {
	accumulator := NewRuntimeUsageAccumulator(true)

	first, ok := accumulator.Delta(RuntimeUsageSnapshot{
		Usage:          protocol.GoalUsage{InputTokens: 10, OutputTokens: 2},
		ElapsedSeconds: 5,
	})
	if !ok || first.BudgetTokens() != 0 || first.ActualTokens() != 0 || first.RuntimeSeconds != 5 {
		t.Fatalf("first delta = %#v, ok = %v, want elapsed-only 5s", first, ok)
	}

	second, ok := accumulator.Delta(RuntimeUsageSnapshot{
		Usage:          protocol.GoalUsage{InputTokens: 15, OutputTokens: 4},
		ElapsedSeconds: 7,
	})
	if !ok || second.BudgetTokens() != 0 || second.ActualTokens() != 0 || second.RuntimeSeconds != 2 {
		t.Fatalf("second delta = %#v, ok = %v, want elapsed-only 2s", second, ok)
	}

	accumulator.Reset(RuntimeUsageSnapshot{
		Usage:          protocol.GoalUsage{InputTokens: 20, OutputTokens: 5},
		ElapsedSeconds: 10,
	})
	afterReset, ok := accumulator.Delta(RuntimeUsageSnapshot{
		Usage:          protocol.GoalUsage{InputTokens: 25, OutputTokens: 8},
		ElapsedSeconds: 15,
	})
	if !ok || afterReset.BudgetTokens() != 0 || afterReset.ActualTokens() != 0 || afterReset.RuntimeSeconds != 5 {
		t.Fatalf("after reset delta = %#v, ok = %v, want elapsed-only 5s", afterReset, ok)
	}

	accumulator.Close()
	if delta, ok := accumulator.Delta(RuntimeUsageSnapshot{
		Usage:          protocol.GoalUsage{InputTokens: 40, OutputTokens: 10},
		ElapsedSeconds: 30,
	}); ok {
		t.Fatalf("closed delta = %#v, ok = true, want no delta", delta)
	}
}

func TestRuntimeUsageAccumulatorSeparatesActualAndBudgetDeltas(t *testing.T) {
	accumulator := NewRuntimeUsageAccumulator(true)

	first, ok := accumulator.Delta(RuntimeUsageSnapshot{Usage: protocol.GoalUsage{
		InputTokens:          10,
		OutputTokens:         2,
		CacheReadInputTokens: 100,
		ActualTotalTokens:    112,
	}})
	if ok {
		t.Fatalf("first delta = %#v, ok = true, want all token fields deferred", first)
	}

	second, ok := accumulator.Delta(RuntimeUsageSnapshot{Usage: protocol.GoalUsage{
		InputTokens:          15,
		OutputTokens:         4,
		CacheReadInputTokens: 150,
		ActualTotalTokens:    169,
	}})
	if ok {
		t.Fatalf("second delta = %#v, ok = true, want all token fields deferred", second)
	}

	final, ok := accumulator.Delta(RuntimeUsageSnapshot{
		Usage: protocol.GoalUsage{
			InputTokens:          15,
			OutputTokens:         4,
			CacheReadInputTokens: 150,
			ActualTotalTokens:    169,
		},
		Cumulative: true,
		Terminal:   true,
	})
	if !ok || final.BudgetTokens() != 19 || final.ActualTokens() != 169 ||
		final.InputTokens != 15 || final.OutputTokens != 4 ||
		final.CacheReadInputTokens != 150 {
		t.Fatalf("final delta = %#v, ok = %v, want full terminal token snapshot", final, ok)
	}
}

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

func TestRuntimeUsageAccumulatorCorrectsSameTurnAndStopsAtClose(t *testing.T) {
	accumulator := NewRuntimeUsageAccumulator(true)
	first, ok := accumulator.Delta(runtimeTurnSnapshot("turn-a", 90, 10))
	if ok {
		t.Fatalf("first delta = %#v, ok = true, want tokens deferred", first)
	}
	correction, ok := accumulator.Delta(runtimeTurnSnapshot("turn-a", 95, 15))
	if ok {
		t.Fatalf("correction delta = %#v, ok = true, want tokens deferred", correction)
	}
	accumulator.Close()
	if delta, ok := accumulator.Delta(RuntimeUsageSnapshot{
		Usage:      protocol.GoalUsage{InputTokens: 180, OutputTokens: 20, ActualTotalTokens: 200},
		Cumulative: true,
	}); ok {
		t.Fatalf("closed final delta = %#v, want none", delta)
	}
}

func TestRuntimeUsageAccumulatorPreservesAuthoritativeZeroTotalDelta(t *testing.T) {
	accumulator := NewRuntimeUsageAccumulator(true)
	if delta, ok := accumulator.Delta(RuntimeUsageSnapshot{
		TurnID: "turn-a",
		Usage: protocol.GoalUsage{
			InputTokens:       90,
			OutputTokens:      10,
			ActualTotalTokens: 100,
		},
	}); ok {
		t.Fatalf("first delta = %#v, want tokens deferred", delta)
	}

	delta, ok := accumulator.Delta(RuntimeUsageSnapshot{
		TurnID: "turn-a",
		Usage: protocol.GoalUsage{
			InputTokens:       95,
			OutputTokens:      15,
			ActualTotalTokens: 100,
		},
	})
	if ok {
		t.Fatalf("redistribution delta = %#v, ok = true, want tokens deferred", delta)
	}

	delta, ok = accumulator.Delta(RuntimeUsageSnapshot{
		TurnID:             "turn-a",
		Usage:              protocol.GoalUsage{InputTokens: 95, OutputTokens: 15, ActualTotalTokens: 100},
		SettlementBoundary: true,
	})
	if !ok || delta.ActualTokens() != 100 || delta.BudgetTokens() != 110 {
		t.Fatalf("settlement delta = %#v, ok = %v, want authoritative actual 100 and budget 110", delta, ok)
	}
}

func TestRuntimeUsageAccumulatorReplacesIntermediateEstimateWithTerminalExact(t *testing.T) {
	for _, test := range []struct {
		name      string
		exact     int64
		wantDelta int64
	}{
		{name: "same total upgrades provenance", exact: 200, wantDelta: 200},
		{name: "lower exact replaces estimate", exact: 180, wantDelta: 180},
	} {
		t.Run(test.name, func(t *testing.T) {
			accumulator := NewRuntimeUsageAccumulator(true)
			estimated, ok := accumulator.PrepareDelta(RuntimeUsageSnapshot{
				TurnID: "turn-estimated",
				Usage: protocol.GoalUsage{
					InputTokens:           150,
					OutputTokens:          50,
					ActualTokensEstimated: true,
				},
			})
			if ok {
				t.Fatalf("intermediate delta = %#v, ok = true, want all tokens deferred", estimated)
			}

			exact, ok := accumulator.PrepareDelta(RuntimeUsageSnapshot{
				Usage: protocol.GoalUsage{
					InputTokens:       140,
					OutputTokens:      40,
					ActualTotalTokens: test.exact,
					ActualTotalKnown:  true,
				},
				Cumulative: true,
				Terminal:   true,
			})
			if !ok || exact.ActualTokens() != test.wantDelta || exact.ActualTokensAreEstimated() ||
				exact.BudgetTokens() != 180 {
				t.Fatalf("terminal delta = %#v, ok = %v, want exact actual %d and budget 180", exact, ok, test.wantDelta)
			}
		})
	}
}

func TestRuntimeUsageAccumulatorRetriesUncommittedDeltaAtTerminal(t *testing.T) {
	accumulator := NewRuntimeUsageAccumulator(true)
	first, ok := accumulator.PrepareDelta(RuntimeUsageSnapshot{
		TurnID: "turn-a",
		Usage: protocol.GoalUsage{
			InputTokens:       90,
			OutputTokens:      10,
			ActualTotalTokens: 100,
			ActualTotalKnown:  true,
		},
		ElapsedSeconds: 5,
	})
	if !ok || first.ActualTokens() != 0 || first.BudgetTokens() != 0 || first.RuntimeSeconds != 5 {
		t.Fatalf("first prepared delta = %#v, ok = %v, want elapsed-only retry delta", first, ok)
	}
	// 模拟持久化失败：不 CommitDelta。
	final, ok := accumulator.PrepareDelta(RuntimeUsageSnapshot{
		Usage: protocol.GoalUsage{
			InputTokens:       140,
			OutputTokens:      10,
			ActualTotalTokens: 150,
			ActualTotalKnown:  true,
		},
		Cumulative: true,
		Terminal:   true,
	})
	if !ok || final.ActualTokens() != 150 || final.BudgetTokens() != 150 || final.RuntimeSeconds != 5 {
		t.Fatalf("terminal retry delta = %#v, ok = %v, want complete 150 plus uncommitted elapsed", final, ok)
	}
}

func TestRuntimeUsageAccumulatorTerminalExactCanReduceIntermediateExact(t *testing.T) {
	accumulator := NewRuntimeUsageAccumulator(true)
	intermediate, ok := accumulator.PrepareDelta(RuntimeUsageSnapshot{
		TurnID: "turn-a",
		Usage: protocol.GoalUsage{
			InputTokens:       190,
			OutputTokens:      10,
			ActualTotalTokens: 200,
			ActualTotalKnown:  true,
		},
	})
	if ok {
		t.Fatalf("intermediate delta = %#v, ok = true, want all tokens deferred", intermediate)
	}

	terminal, ok := accumulator.PrepareDelta(RuntimeUsageSnapshot{
		Usage: protocol.GoalUsage{
			InputTokens:       170,
			OutputTokens:      10,
			ActualTotalTokens: 180,
			ActualTotalKnown:  true,
		},
		Cumulative: true,
		Terminal:   true,
	})
	if !ok || terminal.ActualTokens() != 180 || terminal.ActualTokensAreEstimated() ||
		terminal.BudgetTokens() != 180 ||
		terminal.InputTokens != 170 || terminal.OutputTokens != 10 {
		t.Fatalf("terminal delta = %#v, ok = %v, want authoritative exact 180 with corrected components", terminal, ok)
	}
}

func TestRuntimeUsageAccumulatorTotalOnlyTerminalKeepsObservedBreakdown(t *testing.T) {
	accumulator := NewRuntimeUsageAccumulator(true)
	if delta, ok := accumulator.PrepareDelta(RuntimeUsageSnapshot{
		TurnID:             "turn-a",
		TokenUsageObserved: true,
		Usage: protocol.GoalUsage{
			InputTokens:       90,
			OutputTokens:      10,
			ActualTotalTokens: 100,
			ActualTotalKnown:  true,
		},
	}); ok {
		t.Fatalf("intermediate delta = %#v, want tokens deferred", delta)
	}

	terminal, ok := accumulator.PrepareDelta(RuntimeUsageSnapshot{
		TokenUsageObserved: true,
		Usage: protocol.GoalUsage{
			ActualTotalTokens: 180,
			ActualTotalKnown:  true,
		},
		Cumulative: true,
		Terminal:   true,
	})
	if !ok || terminal.InputTokens != 90 || terminal.OutputTokens != 10 ||
		terminal.BudgetTokens() != 100 || terminal.ActualTokens() != 180 ||
		terminal.ActualTokensAreEstimated() {
		t.Fatalf("total-only terminal delta = %#v, ok = %v, want prior budget 100 plus exact actual 180", terminal, ok)
	}
}

func TestRuntimeUsageAccumulatorExplicitZeroTotalKeepsObservedBreakdownAndPresence(t *testing.T) {
	accumulator := NewRuntimeUsageAccumulator(true)
	accumulator.PrepareDelta(RuntimeUsageSnapshot{
		TurnID:             "turn-a",
		TokenUsageObserved: true,
		Usage: protocol.GoalUsage{
			InputTokens:       90,
			OutputTokens:      10,
			ActualTotalTokens: 100,
			ActualTotalKnown:  true,
		},
	})

	terminal, ok := accumulator.PrepareDelta(RuntimeUsageSnapshot{
		TokenUsageObserved: true,
		Usage: protocol.GoalUsage{
			ActualTotalKnown: true,
		},
		Cumulative: true,
		Terminal:   true,
	})
	if !ok || terminal.BudgetTokens() != 100 || terminal.ActualTokens() != 0 {
		t.Fatalf("explicit-zero terminal delta = %#v, ok = %v, want budget 100 and exact actual zero", terminal, ok)
	}
	if !accumulator.TokenUsageObserved() {
		t.Fatal("explicit-zero terminal usage was treated as missing")
	}
}

func TestRuntimeUsageAccumulatorTokenPresenceRespectsActivationBaseline(t *testing.T) {
	external := NewRuntimeUsageAccumulator(false)
	external.Reset(RuntimeUsageSnapshot{TokenUsageObserved: true})
	if external.TokenUsageObserved() {
		t.Fatal("Reset included pre-activation token presence")
	}
	external.PrepareDelta(RuntimeUsageSnapshot{TokenUsageObserved: true})
	if !external.TokenUsageObserved() {
		t.Fatal("post-Reset explicit token presence was not retained")
	}

	model := NewRuntimeUsageAccumulator(false)
	model.PrepareDelta(RuntimeUsageSnapshot{TokenUsageObserved: true})
	model.PrepareActivationFromRoundStart()
	if !model.TokenUsageObserved() {
		t.Fatal("round-start activation lost pre-create token presence")
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

func TestRuntimeUsageAccumulatorTerminalExplicitZeroOverridesIntermediateExact(t *testing.T) {
	accumulator := NewRuntimeUsageAccumulator(true)
	intermediate, ok := accumulator.PrepareDelta(RuntimeUsageSnapshot{
		TurnID: "turn-a",
		Usage: protocol.GoalUsage{
			InputTokens:       190,
			OutputTokens:      10,
			ActualTotalTokens: 200,
			ActualTotalKnown:  true,
		},
	})
	if ok {
		t.Fatalf("intermediate delta = %#v, ok = true, want all tokens deferred", intermediate)
	}

	terminal, ok := accumulator.PrepareDelta(RuntimeUsageSnapshot{
		Usage: protocol.GoalUsage{
			InputTokens:       190,
			OutputTokens:      10,
			ActualTotalTokens: 0,
			ActualTotalKnown:  true,
		},
		Cumulative: true,
		Terminal:   true,
	})
	if !ok || terminal.ActualTokens() != 0 || terminal.BudgetTokens() != 200 ||
		terminal.InputTokens != 190 || terminal.OutputTokens != 10 {
		t.Fatalf("terminal delta = %#v, ok = %v, want authoritative zero plus terminal breakdown", terminal, ok)
	}
}

func TestRuntimeUsageAccumulatorExternalSettlementBoundaryFlushesDeferredActual(t *testing.T) {
	for _, test := range []struct {
		name      string
		estimated bool
	}{
		{name: "provider actual"},
		{name: "estimated fallback", estimated: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			accumulator := NewRuntimeUsageAccumulator(true)
			intermediate, ok := accumulator.PrepareDelta(RuntimeUsageSnapshot{
				TurnID: "turn-a",
				Usage: protocol.GoalUsage{
					InputTokens:           90,
					OutputTokens:          10,
					ActualTotalTokens:     100,
					ActualTotalKnown:      true,
					ActualTokensEstimated: test.estimated,
				},
			})
			if ok {
				t.Fatalf("intermediate delta = %#v, ok = true, want all tokens deferred", intermediate)
			}

			boundary, ok := accumulator.PrepareDelta(RuntimeUsageSnapshot{
				TurnID:             "turn-a",
				Usage:              intermediateBoundaryUsage(test.estimated),
				SettlementBoundary: true,
			})
			if !ok || boundary.ActualTokens() != 100 ||
				boundary.ActualTokensAreEstimated() != test.estimated ||
				boundary.BudgetTokens() != 100 {
				t.Fatalf("boundary delta = %#v, ok = %v, want full tokens 100 with estimated=%v", boundary, ok, test.estimated)
			}
		})
	}
}

func intermediateBoundaryUsage(estimated bool) protocol.GoalUsage {
	return protocol.GoalUsage{
		InputTokens:           90,
		OutputTokens:          10,
		ActualTotalTokens:     100,
		ActualTotalKnown:      true,
		ActualTokensEstimated: estimated,
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
