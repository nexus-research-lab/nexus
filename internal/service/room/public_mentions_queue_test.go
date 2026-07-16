package room

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkhook "github.com/nexus-research-lab/nexus-agent-sdk-bridge/hook"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

func TestQueueBusyPublicMentionWakesGuidesEachBusyRootAndLeavesIdleTargetReady(t *testing.T) {
	root := t.TempDir()
	conversationID := "conversation-public-mention-mixed-roots"
	roomID := "room-public-mention-mixed-roots"
	sharedSessionKey := protocol.BuildRoomSharedSessionKey(conversationID)
	agents := []protocol.Agent{
		{AgentID: "agent-a", WorkspacePath: filepath.Join(root, "agent-a")},
		{AgentID: "agent-b", WorkspacePath: filepath.Join(root, "agent-b")},
		{AgentID: "agent-c", WorkspacePath: filepath.Join(root, "agent-c")},
	}
	members := make([]protocol.MemberRecord, 0, len(agents))
	for _, agentValue := range agents {
		members = append(members, protocol.MemberRecord{
			RoomID:        roomID,
			MemberType:    protocol.MemberTypeAgent,
			MemberAgentID: agentValue.AgentID,
		})
	}
	contextValue := &protocol.ConversationContextAggregate{
		Room:         protocol.RoomRecord{ID: roomID, RoomType: protocol.RoomTypeGroup},
		Conversation: protocol.ConversationRecord{ID: conversationID, RoomID: roomID},
		Members:      members,
		MemberAgents: agents,
	}
	newSlot := func(agent protocol.Agent, agentRoundID string) *activeRoomSlot {
		return &activeRoomSlot{
			AgentID:           agent.AgentID,
			AgentRoundID:      agentRoundID,
			RuntimeSessionKey: protocol.BuildRoomAgentSessionKey(conversationID, agent.AgentID, protocol.RoomTypeGroup),
			WorkspacePath:     agent.WorkspacePath,
			Status:            "running",
		}
	}
	slotA := newSlot(agents[0], "agent-round-a")
	slotB := newSlot(agents[1], "agent-round-b")
	store := workspacestore.NewInputQueueStore(root)
	runtimeManager := runtimectx.NewManagerWithFactory(roomGuidanceRuntimeFactory{
		client: &permissionModeTestClient{hookResponseAck: true},
	})
	for _, slot := range []*activeRoomSlot{slotA, slotB} {
		if _, err := runtimeManager.GetOrCreate(context.Background(), slot.RuntimeSessionKey, agentclient.Options{}); err != nil {
			t.Fatalf("创建 ACK runtime 失败: %v", err)
		}
	}
	service := &RealtimeService{
		inputQueue: store,
		runtime:    runtimeManager,
		permission: permissionctx.NewContext(),
		activeRounds: map[string]*activeRoomRound{
			"root-a": {
				SessionKey: sharedSessionKey, ConversationID: conversationID, RootRoundID: "root-a",
				Slots: map[string]*activeRoomSlot{slotA.AgentID: slotA},
			},
			"root-b": {
				SessionKey: sharedSessionKey, ConversationID: conversationID, RootRoundID: "root-b",
				Slots: map[string]*activeRoomSlot{slotB.AgentID: slotB},
			},
		},
	}
	parentRound := &activeRoomRound{
		SessionKey: sharedSessionKey, RoomID: roomID, ConversationID: conversationID,
		RootRoundID: "parent-root", Context: contextValue, OwnerUserID: "owner",
	}
	wakes := []publicMentionWake{
		{SourceAgentID: "source", TargetAgentID: agents[0].AgentID, MessageID: "message-a", Content: "@A"},
		{SourceAgentID: "source", TargetAgentID: agents[1].AgentID, MessageID: "message-b", Content: "@B"},
		{SourceAgentID: "source", TargetAgentID: agents[2].AgentID, MessageID: "message-c", Content: "@C"},
	}

	ready, err := service.queueBusyPublicMentionWakes(context.Background(), parentRound, sharedSessionKey, wakes)
	if err != nil {
		t.Fatal(err)
	}
	if len(ready) != 1 || ready[0].TargetAgentID != agents[2].AgentID {
		t.Fatalf("只有空闲目标应立即启动: %+v", ready)
	}
	for _, slot := range []*activeRoomSlot{slotA, slotB} {
		location := workspacestore.InputQueueLocation{
			Scope: protocol.InputQueueScopeRoom, WorkspacePath: slot.WorkspacePath,
			SessionKey: slot.RuntimeSessionKey, RoomID: roomID, ConversationID: conversationID,
		}
		items, snapshotErr := store.Snapshot(location)
		if snapshotErr != nil || len(items) != 1 || items[0].AgentID != slot.AgentID {
			t.Fatalf("不同 root 的忙碌目标都必须进入自己的队列: agent=%s items=%+v err=%v", slot.AgentID, items, snapshotErr)
		}
		if items[0].DeliveryPolicy != protocol.ChatDeliveryPolicyGuide || items[0].RootRoundID != slot.AgentRoundID {
			t.Fatalf("busy 公区 @ 必须绑定目标当前 slot 的 guide: agent=%s item=%+v", slot.AgentID, items[0])
		}
	}

	locationA := workspacestore.InputQueueLocation{
		Scope: protocol.InputQueueScopeRoom, WorkspacePath: slotA.WorkspacePath,
		SessionKey: slotA.RuntimeSessionKey, RoomID: roomID, ConversationID: conversationID,
	}
	output, err := service.roomSlotGuidanceHook(service.activeRounds["root-a"], slotA, locationA)(
		context.Background(),
		sdkhook.Input{EventName: sdkhook.EventPostToolUse},
		"tool-before-public-mention",
	)
	if err != nil {
		t.Fatalf("busy 目标消费公区 @ guide 失败: %v", err)
	}
	if output.SpecificOutput == nil || !strings.Contains(output.SpecificOutput.AdditionalContext, "@A") {
		t.Fatalf("当前 slot 未收到公区 @ additionalContext: %+v", output)
	}
	if output.OnApplied == nil {
		t.Fatal("公区 @ guide 缺少 runtime applied ACK 回调")
	}
	items, err := store.Snapshot(locationA)
	if err != nil || len(items) != 1 {
		t.Fatalf("applied ACK 前必须保留 durable 公区 @: items=%+v err=%v", items, err)
	}
	output.OnApplied(sdkhook.AppliedAck{RequestID: "public-mention-applied"})
	items, err = store.Snapshot(locationA)
	if err != nil || len(items) != 0 {
		t.Fatalf("applied ACK 后应只消费已注入的公区 @: items=%+v err=%v", items, err)
	}
}

func TestSyncQueuedPublicUserMessageKeepsFirstReplyRootAndMergesTargets(t *testing.T) {
	root := t.TempDir()
	conversationID := "conversation-stable-public-user-message"
	roomID := "room-stable-public-user-message"
	sharedSessionKey := protocol.BuildRoomSharedSessionKey(conversationID)
	history := workspacestore.NewRoomHistoryStore(root)
	service := &RealtimeService{
		roomHistory: history,
		permission:  permissionctx.NewContext(),
	}
	contextValue := &protocol.ConversationContextAggregate{
		Room:         protocol.RoomRecord{ID: roomID, RoomType: protocol.RoomTypeGroup},
		Conversation: protocol.ConversationRecord{ID: conversationID, RoomID: roomID},
	}
	baseItem := protocol.InputQueueItem{
		ID: "source-round", SourceMessageID: "shared-user-message", Source: protocol.InputQueueSourceUser,
		Content: "同时交给两个 Agent", DeliveryPolicy: protocol.ChatDeliveryPolicyGuide,
	}
	first := baseItem
	first.AgentID = "agent-a"
	first.TargetAgentIDs = []string{"agent-a"}
	first.RootRoundID = "agent-round-a"
	if err := service.syncQueuedPublicUserMessage(context.Background(), sharedSessionKey, contextValue, first, "reply-root-a", true); err != nil {
		t.Fatal(err)
	}
	second := baseItem
	second.AgentID = "agent-b"
	second.TargetAgentIDs = []string{"agent-b"}
	second.RootRoundID = "agent-round-b"
	if err := service.syncQueuedPublicUserMessage(context.Background(), sharedSessionKey, contextValue, second, "reply-root-b", true); err != nil {
		t.Fatal(err)
	}

	messages, err := history.ReadMessages(conversationID, nil)
	if err != nil {
		t.Fatal(err)
	}
	var userMessages []protocol.Message
	for _, message := range messages {
		if message["message_id"] == "shared-user-message" {
			userMessages = append(userMessages, message)
		}
	}
	if len(userMessages) != 1 {
		t.Fatalf("同一 userMessageId 必须只保留一条公开消息: %+v", userMessages)
	}
	message := userMessages[0]
	if protocol.MessageRoundID(message) != "reply-root-a" || message["source_round_id"] != "source-round" {
		t.Fatalf("后续消费者不能覆盖首个回复归组: %+v", message)
	}
	if message["agent_round_id"] != nil {
		t.Fatalf("多目标公开消息不应归入任一单独 Agent round: %+v", message)
	}
	targets := roomMessageTargetAgentIDs(message["target_agent_ids"])
	if len(targets) != 2 || targets[0] != "agent-a" || targets[1] != "agent-b" {
		t.Fatalf("多个消费者的目标必须聚合: %+v", targets)
	}
}
