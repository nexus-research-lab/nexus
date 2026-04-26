package room

import (
	"context"
	"strings"
	"testing"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-go/protocol"
)

func TestRoomSlotGuidanceHookInjectsQueuedInput(t *testing.T) {
	slot := &activeRoomSlot{}
	slot.enqueueGuidedInput("room-round-guide", "下一步先看工具输出里的错误")

	output, err := roomSlotGuidanceHook(slot)(context.Background(), sdkprotocol.HookInput{
		EventName: sdkprotocol.HookEventPostToolUse,
	}, "tool-1")
	if err != nil {
		t.Fatalf("执行 Room PostToolUse 引导 hook 失败: %v", err)
	}
	additionalContext, _ := output.HookSpecificOutput["additionalContext"].(string)
	if !strings.Contains(additionalContext, "下一步先看工具输出里的错误") ||
		!strings.Contains(additionalContext, "room-round-guide") {
		t.Fatalf("additionalContext 未包含 Room 引导内容: %q", additionalContext)
	}
	if count := len(slot.drainGuidedInputs()); count != 0 {
		t.Fatalf("Room 引导队列未被消费: count=%d", count)
	}
}
