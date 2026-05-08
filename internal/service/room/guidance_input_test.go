package room

import (
	"context"
	"strings"
	"testing"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-go/protocol"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

func TestRoomSlotGuidanceHookInjectsQueuedInput(t *testing.T) {
	slot := &activeRoomSlot{}
	slot.enqueueGuidedInput("room-round-guide", "下一步先看工具输出里的错误")
	storeRoot := t.TempDir()
	service := &RealtimeService{inputQueue: workspacestore.NewInputQueueStore(storeRoot)}

	output, err := service.roomSlotGuidanceHook(nil, slot, workspacestore.InputQueueLocation{
		Scope:         protocol.InputQueueScopeRoom,
		WorkspacePath: storeRoot,
		SessionKey:    protocol.BuildRoomAgentSessionKey("conversation-1", "agent-1", protocol.RoomTypeGroup),
	})(context.Background(), sdkprotocol.HookInput{
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

func TestRoomSlotGuidanceHookConsumesInputQueueGuidance(t *testing.T) {
	storeRoot := t.TempDir()
	store := workspacestore.NewInputQueueStore(storeRoot)
	location := workspacestore.InputQueueLocation{
		Scope:          protocol.InputQueueScopeRoom,
		WorkspacePath:  storeRoot,
		SessionKey:     protocol.BuildRoomAgentSessionKey("conversation-1", "agent-1", protocol.RoomTypeGroup),
		RoomID:         "room-1",
		ConversationID: "conversation-1",
	}
	if _, err := store.Enqueue(location, protocol.InputQueueItem{
		ID:             "room-guide-item",
		Content:        "@Amy 路径发给我吧",
		DeliveryPolicy: protocol.ChatDeliveryPolicyGuide,
		RootRoundID:    "room-round-running",
		Source:         protocol.InputQueueSourceUser,
	}); err != nil {
		t.Fatalf("写入 Room 引导队列失败: %v", err)
	}

	service := &RealtimeService{inputQueue: store}
	slot := &activeRoomSlot{
		AgentID:           "agent-1",
		AgentRoundID:      "room-round-running",
		RuntimeSessionKey: location.SessionKey,
	}
	output, err := service.roomSlotGuidanceHook(nil, slot, location)(context.Background(), sdkprotocol.HookInput{
		EventName: sdkprotocol.HookEventPostToolUse,
	}, "tool-1")
	if err != nil {
		t.Fatalf("执行 Room 队列引导 hook 失败: %v", err)
	}
	additionalContext, _ := output.HookSpecificOutput["additionalContext"].(string)
	if !strings.Contains(additionalContext, "@Amy 路径发给我吧") ||
		!strings.Contains(additionalContext, "queue_room-guide-item") {
		t.Fatalf("additionalContext 未包含 Room 队列引导内容: %q", additionalContext)
	}

	items, err := store.Snapshot(location)
	if err != nil {
		t.Fatalf("读取 Room 引导队列失败: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("Room 队列引导只应在 hook 注入后被消费: %+v", items)
	}
}
