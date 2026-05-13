package room

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	sdkhook "github.com/nexus-research-lab/nexus-agent-sdk-go/hook"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
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
	})(context.Background(), sdkhook.Input{
		EventName: sdkhook.EventPostToolUse,
	}, "tool-1")
	if err != nil {
		t.Fatalf("执行 Room PostToolUse 引导 hook 失败: %v", err)
	}
	additionalContext := ""
	if output.SpecificOutput != nil {
		additionalContext = output.SpecificOutput.AdditionalContext
	}
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
	output, err := service.roomSlotGuidanceHook(nil, slot, location)(context.Background(), sdkhook.Input{
		EventName: sdkhook.EventPostToolUse,
	}, "tool-1")
	if err != nil {
		t.Fatalf("执行 Room 队列引导 hook 失败: %v", err)
	}
	additionalContext := ""
	if output.SpecificOutput != nil {
		additionalContext = output.SpecificOutput.AdditionalContext
	}
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

func TestRoomSlotGuidanceHookKeepsUnanchoredQueueItemWithPublicDelta(t *testing.T) {
	storeRoot := t.TempDir()
	t.Setenv("NEXUS_CONFIG_DIR", filepath.Join(storeRoot, ".nexus"))
	store := workspacestore.NewInputQueueStore(storeRoot)
	roomHistory := workspacestore.NewRoomHistoryStore(storeRoot)
	conversationID := "4b114cfed67a"
	agentID := "agent-1"
	location := workspacestore.InputQueueLocation{
		Scope:          protocol.InputQueueScopeRoom,
		WorkspacePath:  storeRoot,
		SessionKey:     protocol.BuildRoomAgentSessionKey(conversationID, agentID, protocol.RoomTypeGroup),
		RoomID:         "room-1",
		ConversationID: conversationID,
	}
	if err := roomHistory.AppendInlineMessage(conversationID, protocol.Message{
		"message_id":      "public-1",
		"room_id":         "room-1",
		"conversation_id": conversationID,
		"role":            "user",
		"content":         "@Amy 已有公区消息",
		"timestamp":       int64(1),
	}); err != nil {
		t.Fatalf("写入 Room 公区历史失败: %v", err)
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

	service := &RealtimeService{
		permission:  permissionctx.NewContext(),
		inputQueue:  store,
		roomHistory: roomHistory,
	}
	slot := &activeRoomSlot{
		AgentID:           agentID,
		AgentRoundID:      "room-round-running",
		RuntimeSessionKey: location.SessionKey,
		WorkspacePath:     storeRoot,
	}
	roundValue := &activeRoomRound{
		SessionKey:     protocol.BuildRoomSharedSessionKey(conversationID),
		RoomID:         "room-1",
		ConversationID: conversationID,
		Context: &protocol.ConversationContextAggregate{
			Room: protocol.RoomRecord{
				ID:       "room-1",
				RoomType: protocol.RoomTypeGroup,
			},
			Conversation: protocol.ConversationRecord{
				ID:     conversationID,
				RoomID: "room-1",
			},
			Members: []protocol.MemberRecord{{
				RoomID:        "room-1",
				MemberType:    protocol.MemberTypeAgent,
				MemberAgentID: agentID,
			}},
			MemberAgents: []protocol.Agent{{
				AgentID:       agentID,
				Name:          "Amy",
				WorkspacePath: storeRoot,
			}},
		},
	}
	output, err := service.roomSlotGuidanceHook(roundValue, slot, location)(context.Background(), sdkhook.Input{
		EventName: sdkhook.EventPostToolUse,
	}, "tool-1")
	if err != nil {
		t.Fatalf("执行 Room 队列引导 hook 失败: %v", err)
	}
	additionalContext := ""
	if output.SpecificOutput != nil {
		additionalContext = output.SpecificOutput.AdditionalContext
	}
	if !strings.Contains(additionalContext, "已有公区消息") ||
		!strings.Contains(additionalContext, "@Amy 路径发给我吧") ||
		!strings.Contains(additionalContext, "queue_room-guide-item") {
		t.Fatalf("additionalContext 应同时保留公区增量和未入公区的队列引导内容: %q", additionalContext)
	}
}
