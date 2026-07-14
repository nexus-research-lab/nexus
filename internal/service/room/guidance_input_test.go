package room

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	sdkhook "github.com/nexus-research-lab/nexus-agent-sdk-bridge/hook"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

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
	hook := service.roomSlotGuidanceHook(nil, slot, location)
	output, err := hook(context.Background(), sdkhook.Input{
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
	if len(items) != 1 {
		t.Fatalf("control response 尚未确认前应保留 Room 引导: %+v", items)
	}
	secondOutput, err := hook(context.Background(), sdkhook.Input{EventName: sdkhook.EventPostToolUse}, "tool-2")
	if err != nil {
		t.Fatalf("下一次 hook 确认前一次 Room 引导失败: %v", err)
	}
	if secondOutput.SpecificOutput != nil && strings.Contains(secondOutput.SpecificOutput.AdditionalContext, "@Amy 路径发给我吧") {
		t.Fatalf("已确认引导不应在同一 slot 重复注入: %+v", secondOutput)
	}
	items, err = store.Snapshot(location)
	if err != nil || len(items) != 0 {
		t.Fatalf("下一次 hook 应确认并消费前一次 Room 引导: items=%+v err=%v", items, err)
	}
}

func TestRoomSlotGuidanceTransportFailureKeepsDurableInput(t *testing.T) {
	storeRoot := t.TempDir()
	store := workspacestore.NewInputQueueStore(storeRoot)
	location := workspacestore.InputQueueLocation{
		Scope:         protocol.InputQueueScopeRoom,
		WorkspacePath: storeRoot,
		SessionKey:    protocol.BuildRoomAgentSessionKey("conversation-1", "agent-1", protocol.RoomTypeGroup),
	}
	if _, err := store.Enqueue(location, protocol.InputQueueItem{
		ID:             "room-guide-transport-failure",
		Content:        "不要丢掉这条插话",
		DeliveryPolicy: protocol.ChatDeliveryPolicyGuide,
		RootRoundID:    "agent-round-1",
	}); err != nil {
		t.Fatal(err)
	}
	service := &RealtimeService{inputQueue: store}
	slot := &activeRoomSlot{AgentRoundID: "agent-round-1"}
	hook := service.roomSlotGuidanceHook(nil, slot, location)
	if _, err := hook(context.Background(), sdkhook.Input{EventName: sdkhook.EventPostToolUse}, "tool-1"); err != nil {
		t.Fatal(err)
	}

	// 模拟 callback 返回后 control_response 写失败：slot 结束只清内存 pending，持久队列必须保留。
	service.forgetRoomSlotGuidance(slot)
	items, err := store.Snapshot(location)
	if err != nil || len(items) != 1 || items[0].ID != "room-guide-transport-failure" {
		t.Fatalf("transport 失败后 Room 引导必须可重放: items=%+v err=%v", items, err)
	}
}

func TestEnqueueActiveAgentSlotsBatchIsAllOrNoneAndIdempotent(t *testing.T) {
	root := t.TempDir()
	conversationID := "conversation-queue-batch"
	sharedSessionKey := protocol.BuildRoomSharedSessionKey(conversationID)
	validSlot := &activeRoomSlot{
		AgentID:           "agent-a",
		AgentRoundID:      "agent-round-a",
		RuntimeSessionKey: protocol.BuildRoomAgentSessionKey(conversationID, "agent-a", protocol.RoomTypeGroup),
		WorkspacePath:     filepath.Join(root, "agent-a"),
		Status:            "running",
	}
	invalidSlot := &activeRoomSlot{
		AgentID:           "agent-b",
		AgentRoundID:      "agent-round-b",
		RuntimeSessionKey: protocol.BuildRoomAgentSessionKey(conversationID, "agent-b", protocol.RoomTypeGroup),
		Status:            "running",
	}
	store := workspacestore.NewInputQueueStore(root)
	service := &RealtimeService{
		inputQueue: store,
		activeRounds: map[string]*activeRoomRound{
			"active": {
				SessionKey:     sharedSessionKey,
				ConversationID: conversationID,
				Slots: map[string]*activeRoomSlot{
					validSlot.AgentID:   validSlot,
					invalidSlot.AgentID: invalidSlot,
				},
			},
		},
	}
	queued, err := service.enqueueForActiveAgentSlots(
		context.Background(), sharedSessionKey, "room-queue-batch", conversationID,
		[]string{validSlot.AgentID, invalidSlot.AgentID}, "完成后继续处理", nil, "room-queue-batch", "owner",
	)
	if err == nil || len(queued) != 0 {
		t.Fatalf("任一目标位置无效时批量 queue 必须整体失败: queued=%+v err=%v", queued, err)
	}
	validLocation := workspacestore.InputQueueLocation{
		Scope:          protocol.InputQueueScopeRoom,
		WorkspacePath:  validSlot.WorkspacePath,
		SessionKey:     validSlot.RuntimeSessionKey,
		RoomID:         "room-queue-batch",
		ConversationID: conversationID,
	}
	items, snapshotErr := store.Snapshot(validLocation)
	if snapshotErr != nil || len(items) != 0 {
		t.Fatalf("批量失败不应留下部分 Room queue: items=%+v err=%v", items, snapshotErr)
	}

	invalidSlot.WorkspacePath = filepath.Join(root, "agent-b")
	for attempt := 0; attempt < 2; attempt++ {
		queued, err = service.enqueueForActiveAgentSlots(
			context.Background(), sharedSessionKey, "room-queue-batch", conversationID,
			[]string{validSlot.AgentID, invalidSlot.AgentID}, "完成后继续处理", nil, "room-queue-batch", "owner",
		)
		if err != nil || len(queued) != 2 {
			t.Fatalf("批量 Room queue 登记失败: queued=%+v err=%v", queued, err)
		}
	}
	for _, slot := range []*activeRoomSlot{validSlot, invalidSlot} {
		location := workspacestore.InputQueueLocation{
			Scope:          protocol.InputQueueScopeRoom,
			WorkspacePath:  slot.WorkspacePath,
			SessionKey:     slot.RuntimeSessionKey,
			RoomID:         "room-queue-batch",
			ConversationID: conversationID,
		}
		items, err = store.Snapshot(location)
		if err != nil || len(items) != 1 || items[0].ID != "room-queue-batch" || items[0].AgentID != slot.AgentID {
			t.Fatalf("重试不应重复且必须精确绑定目标 slot: slot=%s items=%+v err=%v", slot.AgentID, items, err)
		}
	}
}

func TestGuideActiveAgentSlotsBatchIsAllOrNoneAndIdempotent(t *testing.T) {
	root := t.TempDir()
	sharedSessionKey := protocol.BuildRoomSharedSessionKey("conversation-batch")
	validSlot := &activeRoomSlot{
		AgentID:           "agent-a",
		AgentRoundID:      "agent-round-a",
		RuntimeSessionKey: protocol.BuildRoomAgentSessionKey("conversation-batch", "agent-a", protocol.RoomTypeGroup),
		WorkspacePath:     filepath.Join(root, "agent-a"),
		Status:            "running",
	}
	invalidSlot := &activeRoomSlot{
		AgentID:           "agent-b",
		AgentRoundID:      "agent-round-b",
		RuntimeSessionKey: protocol.BuildRoomAgentSessionKey("conversation-batch", "agent-b", protocol.RoomTypeGroup),
		Status:            "running",
	}
	store := workspacestore.NewInputQueueStore(root)
	service := &RealtimeService{
		inputQueue: store,
		activeRounds: map[string]*activeRoomRound{
			"active": {
				SessionKey:     sharedSessionKey,
				ConversationID: "conversation-batch",
				Slots: map[string]*activeRoomSlot{
					validSlot.AgentID:   validSlot,
					invalidSlot.AgentID: invalidSlot,
				},
			},
		},
	}
	guided, err := service.guideActiveAgentSlots(
		context.Background(), sharedSessionKey, "room-batch", "conversation-batch",
		[]string{validSlot.AgentID, invalidSlot.AgentID}, "同时插话", nil, "room-guide-batch", "owner",
	)
	if err == nil || len(guided) != 0 {
		t.Fatalf("任一目标位置无效时批量登记必须整体失败: guided=%+v err=%v", guided, err)
	}
	validLocation := workspacestore.InputQueueLocation{
		Scope:          protocol.InputQueueScopeRoom,
		WorkspacePath:  validSlot.WorkspacePath,
		SessionKey:     validSlot.RuntimeSessionKey,
		RoomID:         "room-batch",
		ConversationID: "conversation-batch",
	}
	items, snapshotErr := store.Snapshot(validLocation)
	if snapshotErr != nil || len(items) != 0 {
		t.Fatalf("批量失败不应留下部分 Room guide: items=%+v err=%v", items, snapshotErr)
	}

	invalidSlot.WorkspacePath = filepath.Join(root, "agent-b")
	for attempt := 0; attempt < 2; attempt++ {
		guided, err = service.guideActiveAgentSlots(
			context.Background(), sharedSessionKey, "room-batch", "conversation-batch",
			[]string{validSlot.AgentID, invalidSlot.AgentID}, "同时插话", nil, "room-guide-batch", "owner",
		)
		if err != nil || len(guided) != 2 {
			t.Fatalf("批量 Room guide 登记失败: guided=%+v err=%v", guided, err)
		}
	}
	for _, slot := range []*activeRoomSlot{validSlot, invalidSlot} {
		location := workspacestore.InputQueueLocation{
			Scope:          protocol.InputQueueScopeRoom,
			WorkspacePath:  slot.WorkspacePath,
			SessionKey:     slot.RuntimeSessionKey,
			RoomID:         "room-batch",
			ConversationID: "conversation-batch",
		}
		items, err = store.Snapshot(location)
		if err != nil || len(items) != 1 || items[0].RootRoundID != slot.AgentRoundID {
			t.Fatalf("重试不应重复且必须精确绑定目标 slot: slot=%s items=%+v err=%v", slot.AgentID, items, err)
		}
	}
}

func TestGuideActiveAgentSlotsDoesNotSplitPublicMessageAcrossRoots(t *testing.T) {
	root := t.TempDir()
	conversationID := "conversation-common-root"
	sharedSessionKey := protocol.BuildRoomSharedSessionKey(conversationID)
	slotA := &activeRoomSlot{
		AgentID:           "agent-a",
		AgentRoundID:      "agent-round-a",
		RuntimeSessionKey: protocol.BuildRoomAgentSessionKey(conversationID, "agent-a", protocol.RoomTypeGroup),
		WorkspacePath:     filepath.Join(root, "agent-a"),
		Status:            "running",
		TimestampMS:       100,
	}
	slotB := &activeRoomSlot{
		AgentID:           "agent-b",
		AgentRoundID:      "agent-round-b",
		RuntimeSessionKey: protocol.BuildRoomAgentSessionKey(conversationID, "agent-b", protocol.RoomTypeGroup),
		WorkspacePath:     filepath.Join(root, "agent-b"),
		Status:            "running",
		TimestampMS:       200,
	}
	store := workspacestore.NewInputQueueStore(root)
	service := &RealtimeService{
		inputQueue: store,
		activeRounds: map[string]*activeRoomRound{
			"root-a": {
				SessionKey:     sharedSessionKey,
				ConversationID: conversationID,
				RoundID:        "root-a",
				RootRoundID:    "root-a",
				Slots:          map[string]*activeRoomSlot{slotA.AgentID: slotA},
			},
			"root-b": {
				SessionKey:     sharedSessionKey,
				ConversationID: conversationID,
				RoundID:        "root-b",
				RootRoundID:    "root-b",
				Slots:          map[string]*activeRoomSlot{slotB.AgentID: slotB},
			},
		},
	}

	targets := []string{slotA.AgentID, slotB.AgentID}
	guided, err := service.guideActiveAgentSlots(
		context.Background(), sharedSessionKey, "room-common-root", conversationID,
		targets, "同一条多目标插话", nil, "guide-cross-root", "owner",
	)
	if err != nil || len(guided) != 0 {
		t.Fatalf("跨 root 的多目标插话不应部分注入: guided=%+v err=%v", guided, err)
	}
	for _, slot := range []*activeRoomSlot{slotA, slotB} {
		items, snapshotErr := store.Snapshot(workspacestore.InputQueueLocation{
			Scope:          protocol.InputQueueScopeRoom,
			WorkspacePath:  slot.WorkspacePath,
			SessionKey:     slot.RuntimeSessionKey,
			RoomID:         "room-common-root",
			ConversationID: conversationID,
		})
		if snapshotErr != nil || len(items) != 0 {
			t.Fatalf("跨 root 不应留下可以分别 reparent 的 guide: slot=%s items=%+v err=%v", slot.AgentID, items, snapshotErr)
		}
	}

	service.activeRounds = map[string]*activeRoomRound{
		"shared-root": {
			SessionKey:     sharedSessionKey,
			ConversationID: conversationID,
			RoundID:        "shared-root",
			RootRoundID:    "shared-root",
			Slots: map[string]*activeRoomSlot{
				slotA.AgentID: slotA,
				slotB.AgentID: slotB,
			},
		},
	}
	guided, err = service.guideActiveAgentSlots(
		context.Background(), sharedSessionKey, "room-common-root", conversationID,
		targets, "同一条多目标插话", nil, "guide-shared-root", "owner",
	)
	if err != nil || len(guided) != len(targets) {
		t.Fatalf("同 root 多目标应整体注入: guided=%+v err=%v", guided, err)
	}
	for _, slot := range []*activeRoomSlot{slotA, slotB} {
		items, snapshotErr := store.Snapshot(workspacestore.InputQueueLocation{
			Scope:          protocol.InputQueueScopeRoom,
			WorkspacePath:  slot.WorkspacePath,
			SessionKey:     slot.RuntimeSessionKey,
			RoomID:         "room-common-root",
			ConversationID: conversationID,
		})
		if snapshotErr != nil || len(items) != 1 || items[0].ID != "guide-shared-root" {
			t.Fatalf("同 root guide 必须同时进入每个目标 slot: slot=%s items=%+v err=%v", slot.AgentID, items, snapshotErr)
		}
	}
}

func TestReleaseUndeliveredRoomGuidanceDoesNotFollowReplacementRound(t *testing.T) {
	root := t.TempDir()
	conversationID := "conversation-replacement"
	agentID := "agent-a"
	workspacePath := filepath.Join(root, agentID)
	sharedSessionKey := protocol.BuildRoomSharedSessionKey(conversationID)
	location := workspacestore.InputQueueLocation{
		Scope:          protocol.InputQueueScopeRoom,
		WorkspacePath:  workspacePath,
		SessionKey:     protocol.BuildRoomAgentSessionKey(conversationID, agentID, protocol.RoomTypeGroup),
		RoomID:         "room-replacement",
		ConversationID: conversationID,
	}
	store := workspacestore.NewInputQueueStore(root)
	for _, item := range []protocol.InputQueueItem{
		{ID: "stale", AgentID: agentID, Content: "旧 round 插话", DeliveryPolicy: protocol.ChatDeliveryPolicyGuide, RootRoundID: "agent-round-old"},
		{ID: "current", AgentID: agentID, Content: "当前 round 插话", DeliveryPolicy: protocol.ChatDeliveryPolicyGuide, RootRoundID: "agent-round-new"},
	} {
		if _, err := store.Enqueue(location, item); err != nil {
			t.Fatal(err)
		}
	}
	contextValue := &protocol.ConversationContextAggregate{
		Room:         protocol.RoomRecord{ID: "room-replacement", RoomType: protocol.RoomTypeGroup},
		Conversation: protocol.ConversationRecord{ID: conversationID, RoomID: "room-replacement"},
		Members:      []protocol.MemberRecord{{RoomID: "room-replacement", MemberType: protocol.MemberTypeAgent, MemberAgentID: agentID}},
		MemberAgents: []protocol.Agent{{AgentID: agentID, WorkspacePath: workspacePath}},
	}
	service := &RealtimeService{
		inputQueue: store,
		permission: permissionctx.NewContext(),
		activeRounds: map[string]*activeRoomRound{
			"replacement": {
				SessionKey:     sharedSessionKey,
				ConversationID: conversationID,
				Slots: map[string]*activeRoomSlot{
					agentID: {
						AgentID:           agentID,
						AgentRoundID:      "agent-round-new",
						RuntimeSessionKey: location.SessionKey,
						WorkspacePath:     workspacePath,
						Status:            "running",
					},
				},
			},
		},
	}
	service.releaseUndeliveredRoomGuidance(context.Background(), sharedSessionKey, contextValue)
	items, err := store.Snapshot(location)
	if err != nil || len(items) != 2 {
		t.Fatalf("读取 replacement round guide 失败: items=%+v err=%v", items, err)
	}
	if items[0].ID != "stale" || items[0].DeliveryPolicy != protocol.ChatDeliveryPolicyQueue || items[0].RootRoundID != "" {
		t.Fatalf("旧 round guide 必须退回普通队列: %+v", items[0])
	}
	if items[1].ID != "current" || items[1].DeliveryPolicy != protocol.ChatDeliveryPolicyGuide || items[1].RootRoundID != "agent-round-new" {
		t.Fatalf("当前 slot 的 guide 不应被释放: %+v", items[1])
	}
}

func TestConsumedRoomGuidanceMovesUserMessageIntoReplyRound(t *testing.T) {
	storeRoot := t.TempDir()
	store := workspacestore.NewInputQueueStore(storeRoot)
	roomHistory := workspacestore.NewRoomHistoryStore(storeRoot)
	conversationID := "conversation-guidance-order"
	agentID := "agent-1"
	location := workspacestore.InputQueueLocation{
		Scope:          protocol.InputQueueScopeRoom,
		WorkspacePath:  storeRoot,
		SessionKey:     protocol.BuildRoomAgentSessionKey(conversationID, agentID, protocol.RoomTypeGroup),
		RoomID:         "room-1",
		ConversationID: conversationID,
	}
	if err := roomHistory.AppendInlineMessage(conversationID, protocol.Message{
		"message_id":      "guided-user-message",
		"session_key":     protocol.BuildRoomSharedSessionKey(conversationID),
		"room_id":         "room-1",
		"conversation_id": conversationID,
		"round_id":        "guidance-source-round",
		"role":            "user",
		"content":         "然后评价一下",
		"delivery_policy": string(protocol.ChatDeliveryPolicyGuide),
		"timestamp":       int64(200),
	}); err != nil {
		t.Fatalf("写入 Room 引导用户消息失败: %v", err)
	}
	if _, err := store.Enqueue(location, protocol.InputQueueItem{
		ID:              "guided-user-message",
		AgentID:         agentID,
		SourceMessageID: "guidance-source-round",
		Content:         "然后评价一下",
		DeliveryPolicy:  protocol.ChatDeliveryPolicyGuide,
		RootRoundID:     "agent-reply-round",
	}); err != nil {
		t.Fatalf("写入待确认 Room 引导失败: %v", err)
	}

	service := &RealtimeService{
		permission:  permissionctx.NewContext(),
		inputQueue:  store,
		roomHistory: roomHistory,
	}
	contextValue := &protocol.ConversationContextAggregate{
		Room:         protocol.RoomRecord{ID: "room-1", RoomType: protocol.RoomTypeGroup},
		Conversation: protocol.ConversationRecord{ID: conversationID, RoomID: "room-1"},
		Members: []protocol.MemberRecord{{
			RoomID: "room-1", MemberType: protocol.MemberTypeAgent, MemberAgentID: agentID,
		}},
		MemberAgents: []protocol.Agent{{AgentID: agentID, Name: "Amy", WorkspacePath: storeRoot}},
	}
	roundValue := &activeRoomRound{
		SessionKey:     protocol.BuildRoomSharedSessionKey(conversationID),
		RoomID:         "room-1",
		ConversationID: conversationID,
		RootRoundID:    "goal-reply-round",
		Context:        contextValue,
	}
	slot := &activeRoomSlot{AgentID: agentID, AgentRoundID: "agent-reply-round", RuntimeSessionKey: location.SessionKey, WorkspacePath: storeRoot}
	hook := service.roomSlotGuidanceHook(roundValue, slot, location)
	if _, err := hook(context.Background(), sdkhook.Input{EventName: sdkhook.EventPostToolUse}, "tool-1"); err != nil {
		t.Fatalf("准备 Room 引导失败: %v", err)
	}
	readGuidedMessage := func() protocol.Message {
		messages, err := roomHistory.ReadMessages(conversationID, nil)
		if err != nil {
			t.Fatalf("读取 Room 引导历史失败: %v", err)
		}
		for _, candidate := range messages {
			if candidate["message_id"] == "guided-user-message" {
				return candidate
			}
		}
		return nil
	}
	message := readGuidedMessage()
	if message == nil {
		t.Fatal("Room 历史缺少引导用户消息")
	}
	if protocol.MessageRoundID(message) != "guidance-source-round" || message["source_round_id"] != nil {
		t.Fatalf("control response 未确认前不应移动用户消息: %+v", message)
	}
	if _, err := hook(context.Background(), sdkhook.Input{EventName: sdkhook.EventPostToolUse}, "tool-2"); err != nil {
		t.Fatalf("确认 Room 引导失败: %v", err)
	}
	message = readGuidedMessage()
	if protocol.MessageRoundID(message) != "goal-reply-round" ||
		message["source_round_id"] != "guidance-source-round" {
		t.Fatalf("已消费引导未归入模型回复 round: %+v", message)
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
