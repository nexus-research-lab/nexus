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
	if !ok || first.InputTokens != 10 || first.OutputTokens != 2 || first.Total() != 12 || first.RuntimeSeconds != 5 {
		t.Fatalf("first delta = %#v, ok = %v, want 10/2 usage with 5s", first, ok)
	}

	second, ok := accumulator.Delta(RuntimeUsageSnapshot{
		Usage:          protocol.GoalUsage{InputTokens: 15, OutputTokens: 4},
		ElapsedSeconds: 7,
	})
	if !ok || second.InputTokens != 5 || second.OutputTokens != 2 || second.Total() != 7 || second.RuntimeSeconds != 2 {
		t.Fatalf("second delta = %#v, ok = %v, want 5/2 usage with 2s", second, ok)
	}

	accumulator.Reset(RuntimeUsageSnapshot{
		Usage:          protocol.GoalUsage{InputTokens: 20, OutputTokens: 5},
		ElapsedSeconds: 10,
	})
	afterReset, ok := accumulator.Delta(RuntimeUsageSnapshot{
		Usage:          protocol.GoalUsage{InputTokens: 25, OutputTokens: 8},
		ElapsedSeconds: 15,
	})
	if !ok || afterReset.InputTokens != 5 || afterReset.OutputTokens != 3 || afterReset.Total() != 8 || afterReset.RuntimeSeconds != 5 {
		t.Fatalf("after reset delta = %#v, ok = %v, want 5/3 usage with 5s", afterReset, ok)
	}

	accumulator.Close()
	if delta, ok := accumulator.Delta(RuntimeUsageSnapshot{
		Usage:          protocol.GoalUsage{InputTokens: 40, OutputTokens: 10},
		ElapsedSeconds: 30,
	}); ok {
		t.Fatalf("closed delta = %#v, ok = true, want no delta", delta)
	}
}
