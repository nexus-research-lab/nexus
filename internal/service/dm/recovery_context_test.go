package dm

import (
	"testing"

	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
)

func TestRoundRunnerContextualInputsCombinesGoalAndRecovery(t *testing.T) {
	recovery := runtimectx.NewContextualInputBlock(
		runtimectx.ContextualInputNameRoundRecovery,
		"Recorded terminal reason: content_filtered.",
		0,
		nil,
	)
	runner := &roundRunner{
		sessionKey:     "dm:agent-a:session-a",
		goalContext:    "goal context",
		goalIDForUsage: "goal-a",
		recoveryContext: []runtimectx.ContextualInputBlock{
			recovery,
		},
	}
	inputs := runner.contextualInputs()
	if len(inputs) != 2 || inputs[0].Name != goalContextualInputName ||
		inputs[1].Name != runtimectx.ContextualInputNameRoundRecovery {
		t.Fatalf("DM contextual inputs 未同时保留 Goal 与失败恢复上下文: %+v", inputs)
	}
}
