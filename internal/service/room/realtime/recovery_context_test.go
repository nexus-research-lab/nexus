package realtime

import (
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
)

func TestSlotExecutionRecoveryContextOnlyForExplicitUserTrigger(t *testing.T) {
	history := []protocol.Message{{
		"role":        "assistant",
		"agent_id":    "agent-a",
		"is_complete": true,
		"result_summary": map[string]any{
			"subtype":         "error",
			"is_error":        true,
			"terminal_reason": protocol.ProviderFailureContentFiltered,
		},
	}}
	execution := &slotExecution{
		round:   &activeRoomRound{},
		slot:    &activeRoomSlot{AgentID: "agent-a", Trigger: roomTrigger{TriggerType: "public_chat"}},
		history: history,
	}
	inputs := execution.contextualInputs()
	if len(inputs) != 1 || inputs[0].Name != runtimectx.ContextualInputNameRoundRecovery {
		t.Fatalf("真实用户触发应带上目标 Agent 的失败恢复上下文: %+v", inputs)
	}

	execution.slot.Trigger.TriggerType = "public_mention"
	if got := execution.contextualInputs(); len(got) != 0 {
		t.Fatalf("Agent 接力唤醒不应消费用户轮失败恢复语义: %+v", got)
	}

	execution.slot.Trigger.TriggerType = "public_chat"
	execution.round.Internal = true
	if got := execution.contextualInputs(); len(got) != 0 {
		t.Fatalf("内部轮次不应注入用户轮失败恢复语义: %+v", got)
	}
}
