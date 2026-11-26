package conversation

import (
	"strings"
	"testing"

	messagepkg "github.com/nexus-research-lab/nexus/internal/message"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestRecoveryContextDetectsIncompleteHistoricalTurn(t *testing.T) {
	for _, role := range []string{"user", "assistant"} {
		t.Run(role, func(t *testing.T) {
			history := []protocol.Message{
				{"role": "result", "round_id": "older", "subtype": "success"},
				{"role": role, "round_id": "interrupted", "agent_id": "a", "content": "partial"},
				{"role": "user", "round_id": "current", "agent_id": "a"},
			}
			blocks := RoundRecoveryContextualInputs(history, "a", "current")
			if len(blocks) != 1 {
				t.Fatalf("recovery blocks = %#v", blocks)
			}
			outcome, ok := latestTerminalOutcome(history, "a", []string{"current"})
			if !ok || outcome.reason != recoveryInterruptedByHost {
				t.Fatalf("outcome = %#v", outcome)
			}
			if !strings.Contains(recoveryInstruction(outcome.reason), "do not automatically replay side effects") {
				t.Fatal("missing reconciliation instruction")
			}
		})
	}
}

func TestRecoveryContextTerminalAndAgentBoundaries(t *testing.T) {
	for _, subtype := range []string{"success", "interrupted"} {
		history := []protocol.Message{
			{"role": "assistant", "round_id": "old", "agent_id": "a"},
			{"role": "result", "round_id": "old", "agent_id": "a", "subtype": subtype},
			{"role": "assistant", "round_id": "other", "agent_id": "b"},
			{"role": "user", "round_id": "current", "agent_id": "a"},
		}
		if blocks := RoundRecoveryContextualInputs(history, "a", "current"); len(blocks) != 0 {
			t.Fatalf("%s: %#v", subtype, blocks)
		}
	}
}

func TestRecoveryContextRecognizesMaterializedIncompleteRound(t *testing.T) {
	history := []protocol.Message{{"role": "assistant", "is_complete": true,
		"result_summary": map[string]any{"subtype": "interrupted", "is_error": false, "terminal_reason": "interrupted_by_host"}}}
	if blocks := RoundRecoveryContextualInputs(history, ""); len(blocks) != 1 {
		t.Fatalf("blocks = %#v", blocks)
	}
}

func TestRecoveryContextReadsHostRunOutcome(t *testing.T) {
	for outcome, wantBlocks := range map[string]int{"quiet": 0, "stopped": 0, "failed": 1} {
		history := []protocol.Message{
			{"role": "user", "round_id": "host", "agent_id": "a", "content": "<scheduled-task>"},
			{"role": "system", "round_id": "host", "agent_id": "a", "content": outcome, "metadata": map[string]any{
				"subtype": messagepkg.SystemMessageSubtypeHostRunOutcome, "outcome": outcome,
			}},
			{"role": "user", "round_id": "current", "agent_id": "a"},
		}
		if blocks := RoundRecoveryContextualInputs(history, "a", "current"); len(blocks) != wantBlocks {
			t.Fatalf("%s: recovery blocks = %#v", outcome, blocks)
		}
	}
}
