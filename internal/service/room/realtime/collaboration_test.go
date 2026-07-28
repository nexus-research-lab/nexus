package realtime

import (
	"context"
	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkhook "github.com/nexus-research-lab/nexus-agent-sdk-bridge/hook"
	roomdomain "github.com/nexus-research-lab/nexus/internal/chat/room"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestCoalesceRoomDirectedWakeEntriesKeepsPerTargetOrder(t *testing.T) {
	location := workspacestore.InputQueueLocation{WorkspacePath: "/tmp/agent", SessionKey: "room:conversation-1:agent-1"}
	newEntry := func(id string, source protocol.InputQueueSource, root string) roomInputQueueEntry {
		return roomInputQueueEntry{
			Location: location,
			Item: protocol.InputQueueItem{
				ID:          id,
				AgentID:     "agent-1",
				Source:      source,
				RootRoundID: root,
				ReplyRoute:  protocol.RoomReplyRoute{Mode: protocol.RoomReplyRoutePublic},
			},
		}
	}
	entries := []roomInputQueueEntry{
		newEntry("direct-1", protocol.InputQueueSourceAgentRoomMessage, "root-1"),
		newEntry("direct-2", protocol.InputQueueSourceAgentRoomMessage, "root-1"),
		newEntry("user-1", protocol.InputQueueSourceUser, ""),
		newEntry("direct-3", protocol.InputQueueSourceAgentRoomMessage, "root-1"),
	}
	batch := coalesceRoomDirectedWakeEntries(entries[0], entries)
	if len(batch) != 2 || batch[0].Item.ID != "direct-1" || batch[1].Item.ID != "direct-2" {
		t.Fatalf("合并应在同目标的第一条不兼容消息前停止: %+v", batch)
	}
}

func TestResolveRoomMessageCausalityUsesActiveRound(t *testing.T) {
	service := &Service{rounds: newRoomRoundRegistry()}
	roundValue := &activeRoomRound{
		ConversationID: "conversation-1",
		RoundID:        "round-child",
		RootRoundID:    "round-root",
		HopIndex:       3,
		Slots: map[string]*activeRoomSlot{
			"slot-1": {AgentID: "agent-1"},
		},
	}
	service.rounds.register(roundValue)
	root, cause, hop := service.resolveRoomMessageCausality("conversation-1", "agent-1", "round-root")
	if root != "round-root" || cause != "round-child" || hop != 3 {
		t.Fatalf("工具消息未继承当前 Room 因果链: root=%s cause=%s hop=%d", root, cause, hop)
	}
}

func TestPublicInputBatchIgnoresStoredCursorWhenRuntimeCannotResume(t *testing.T) {
	workspacePath := t.TempDir()
	history := workspacestore.NewAgentHistoryStore(t.TempDir())
	service := &Service{history: history}
	roundValue := &activeRoomRound{ConversationID: "conversation-1"}
	slot := &activeRoomSlot{
		AgentID:           "agent-1",
		AgentRoundID:      "agent-round-1",
		RuntimeSessionKey: "agent:agent-1:ws:group:conversation-1",
		WorkspacePath:     workspacePath,
	}
	slot.setContextColdStart(true)
	if err := history.AppendRoomPublicCursor(workspacePath, slot.RuntimeSessionKey, workspacestore.RoomPublicCursor{
		ConversationID:      roundValue.ConversationID,
		AgentID:             slot.AgentID,
		LastPublicMessageID: "message-1",
		LastPublicTimestamp: 1,
	}); err != nil {
		t.Fatalf("写入 Room public cursor 失败: %v", err)
	}
	publicHistory := []protocol.Message{
		{"message_id": "message-1", "role": "user", "content": "旧上下文", "timestamp": int64(1)},
		{"message_id": "message-2", "role": "user", "content": "新上下文", "timestamp": int64(2)},
	}

	coldBatch, err := service.publicInputBatchForSlot(
		context.Background(),
		roundValue,
		slot,
		publicHistory,
		roomdomain.PublicCursor{},
		false,
	)
	if err != nil {
		t.Fatalf("构造冷启动 public batch 失败: %v", err)
	}
	if !coldBatch.ColdStart || len(coldBatch.Messages) != 2 {
		t.Fatalf("runtime 无法 resume 时必须忽略旧 cursor: %+v", coldBatch)
	}

	slot.setContextColdStart(false)
	warmBatch, err := service.publicInputBatchForSlot(
		context.Background(),
		roundValue,
		slot,
		publicHistory,
		roomdomain.PublicCursor{},
		false,
	)
	if err != nil {
		t.Fatalf("构造 warm public batch 失败: %v", err)
	}
	if warmBatch.ColdStart || len(warmBatch.Messages) != 1 || warmBatch.LastMessageID != "message-2" {
		t.Fatalf("可 resume 时应从旧 cursor 后继续: %+v", warmBatch)
	}
}

// 公共移交意图测试。

func TestAnnotatePublicAssistantMessageSeparatesDisplayMentionFromDefaultHandoff(t *testing.T) {
	contextValue := &protocol.ConversationContextAggregate{
		Conversation: protocol.ConversationRecord{ID: "conversation-intent"},
		Members: []protocol.MemberRecord{
			{MemberType: protocol.MemberTypeAgent, MemberAgentID: "agent-source"},
			{MemberType: protocol.MemberTypeAgent, MemberAgentID: "agent-amy"},
			{MemberType: protocol.MemberTypeAgent, MemberAgentID: "agent-devin"},
		},
		MemberAgents: []protocol.Agent{
			{AgentID: "agent-source", Name: "Source"},
			{AgentID: "agent-amy", Name: "Amy"},
			{AgentID: "agent-devin", Name: "Devin"},
		},
	}
	roundValue := &activeRoomRound{
		Context: contextValue, ConversationID: contextValue.Conversation.ID,
		RoomID: "room-intent", RootRoundID: "root-intent",
	}
	slot := &activeRoomSlot{AgentID: "agent-source", AgentRoundID: "source-round"}
	message := protocol.Message{
		"message_id":  "message-intent",
		"role":        "assistant",
		"is_complete": true,
		// runtime 传入的旧 annotation 不能绕过服务端的单目标选择。
		"agent_mentions": []protocol.AgentMention{{
			AgentID: "agent-devin", HandoffID: "runtime-forged-handoff",
		}},
		"content": []map[string]any{{
			"type": "text", "text": "先请 @Amy 处理，@Devin 作为展示候选。",
		}},
	}
	service := &Service{}
	if err := service.annotatePublicAssistantMessage(roundValue, slot, message); err != nil {
		t.Fatal(err)
	}
	mentions := protocolAgentMentions(message["agent_mentions"])
	if len(mentions) != 2 || mentions[0].HandoffID == "" || mentions[1].HandoffID != "" {
		t.Fatalf("默认单目标 handoff 标注不正确: %+v", mentions)
	}
	if wakes := publicMentionWakesFromMessage(roundValue, slot, message, roomdomain.ExtractAssistantResultText(message)); len(wakes) != 1 || wakes[0].TargetAgentID != "agent-amy" {
		t.Fatalf("展示 mention 不应唤醒目标: %+v", wakes)
	}
}

func TestAnnotatePublicAssistantMessageAcceptsParenthesizedAgentID(t *testing.T) {
	contextValue := &protocol.ConversationContextAggregate{
		Conversation: protocol.ConversationRecord{ID: "conversation-parenthesized"},
		Members: []protocol.MemberRecord{
			{MemberType: protocol.MemberTypeAgent, MemberAgentID: "agent-source"},
			{MemberType: protocol.MemberTypeAgent, MemberAgentID: "agent-plan"},
		},
		MemberAgents: []protocol.Agent{
			{AgentID: "agent-source", Name: "Source"},
			{AgentID: "agent-plan", Name: "生活方案制定员"},
		},
	}
	roundValue := &activeRoomRound{
		Context: contextValue, ConversationID: contextValue.Conversation.ID,
		RoomID: "room-parenthesized", RootRoundID: "root-parenthesized",
	}
	slot := &activeRoomSlot{AgentID: "agent-source", AgentRoundID: "source-round"}
	message := protocol.Message{
		"message_id":  "message-parenthesized",
		"role":        "assistant",
		"is_complete": true,
		"content": []map[string]any{{
			"type": "text", "text": "@生活方案制定员（c742e12ab802）请承接本次咨询。",
		}},
	}
	service := &Service{}
	if err := service.annotatePublicAssistantMessage(roundValue, slot, message); err != nil {
		t.Fatal(err)
	}
	mentions := protocolAgentMentions(message["agent_mentions"])
	if len(mentions) != 1 || mentions[0].AgentID != "agent-plan" || mentions[0].HandoffID == "" {
		t.Fatalf("带括号 agent id 的 mention 应创建默认 handoff: %+v", mentions)
	}
	if mentions[0].Label != "生活方案制定员" || mentions[0].StartRune != 0 || mentions[0].EndRune != 8 {
		t.Fatalf("mention span 不应包含括号中的 agent id: %+v", mentions[0])
	}
}

func TestAnnotatePublicAssistantMessageRequiresExplicitFanoutMarker(t *testing.T) {
	contextValue := &protocol.ConversationContextAggregate{
		Conversation: protocol.ConversationRecord{ID: "conversation-fanout"},
		Members: []protocol.MemberRecord{
			{MemberType: protocol.MemberTypeAgent, MemberAgentID: "agent-source"},
			{MemberType: protocol.MemberTypeAgent, MemberAgentID: "agent-amy"},
			{MemberType: protocol.MemberTypeAgent, MemberAgentID: "agent-devin"},
		},
		MemberAgents: []protocol.Agent{
			{AgentID: "agent-source", Name: "Source"},
			{AgentID: "agent-amy", Name: "Amy"},
			{AgentID: "agent-devin", Name: "Devin"},
		},
	}
	roundValue := &activeRoomRound{
		Context: contextValue, ConversationID: contextValue.Conversation.ID,
		RoomID: "room-fanout", RootRoundID: "root-fanout",
	}
	slot := &activeRoomSlot{AgentID: "agent-source", AgentRoundID: "source-round"}
	message := protocol.Message{
		"message_id":  "message-fanout",
		"role":        "assistant",
		"is_complete": true,
		"content": []map[string]any{{
			"type": "text", "text": "请 @Amy 和 @Devin 并行处理。<nexus_room_fanout/>",
		}},
	}
	service := &Service{}
	if err := service.annotatePublicAssistantMessage(roundValue, slot, message); err != nil {
		t.Fatal(err)
	}
	content := roomdomain.ExtractAssistantResultText(message)
	if strings.Contains(content, roomdomain.FanoutMarker) {
		t.Fatalf("fanout 控制标记不应进入正文: %q", content)
	}
	mentions := protocolAgentMentions(message["agent_mentions"])
	if len(mentions) != 2 || mentions[0].HandoffID == "" || mentions[1].HandoffID == "" {
		t.Fatalf("显式 fanout 应为全部目标写入 handoff: %+v", mentions)
	}
	if wakes := publicMentionWakesFromMessage(roundValue, slot, message, content); len(wakes) != 2 {
		t.Fatalf("显式 fanout 应唤醒两个目标: %+v", wakes)
	}
}

func TestPublicHandoffAdmissionDetectsCycle(t *testing.T) {
	edges := []workspacestore.RoomPublicHandoff{
		{SourceAgentID: "agent-a", TargetAgentID: "agent-b"},
		{SourceAgentID: "agent-b", TargetAgentID: "agent-c"},
	}
	if !roomPublicHandoffCreatesCycle(edges, "agent-c", "agent-a") {
		t.Fatal("应拒绝会回到 source 的 root handoff 环")
	}
	if roomPublicHandoffCreatesCycle(edges, "agent-c", "agent-d") {
		t.Fatal("无环的新目标不应被拒绝")
	}
}

func TestPublicHandoffAdmissionRejectsRecordedCycleAndRootOverflow(t *testing.T) {
	root := t.TempDir()
	t.Setenv("NEXUS_STATE_ROOT", root)
	t.Setenv("NEXUS_CONFIG_DIR", root)
	conversationID := "conversation-admission-guard"
	store := workspacestore.NewRoomPublicHandoffStore(root)
	detect := func(handoff workspacestore.RoomPublicHandoff) {
		t.Helper()
		if _, _, err := store.Detect("owner", handoff); err != nil {
			t.Fatal(err)
		}
		if err := store.MarkSourceFinished("owner", conversationID, handoff.HandoffID); err != nil {
			t.Fatal(err)
		}
	}
	detect(workspacestore.RoomPublicHandoff{
		HandoffID: "rh-cycle-forward", ConversationID: conversationID, RootRoundID: "root-guard",
		SourceMessageID: "message-forward", SourceAgentID: "agent-a", TargetAgentID: "agent-b",
	})
	detect(workspacestore.RoomPublicHandoff{
		HandoffID: "rh-cycle-back", ConversationID: conversationID, RootRoundID: "root-guard",
		SourceMessageID: "message-back", SourceAgentID: "agent-b", TargetAgentID: "agent-a",
	})
	service := &Service{publicHandoffs: store}
	parent := &activeRoomRound{ConversationID: conversationID, RootRoundID: "root-guard", OwnerUserID: "owner"}
	accepted, err := service.admitPublicMentionWakes(context.Background(), parent, []publicMentionWake{{
		HandoffID: "rh-cycle-back", QueueSource: protocol.InputQueueSourceAgentPublicMention,
		SourceAgentID: "agent-b", TargetAgentID: "agent-a",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(accepted) != 0 {
		t.Fatalf("已记录的 reciprocal edge 仍必须经过 cycle guard: %+v", accepted)
	}
	cycle, ok, err := store.Get("owner", conversationID, "rh-cycle-back")
	if err != nil || !ok || cycle.Status != "error" {
		t.Fatalf("cycle handoff 应收口为 error: handoff=%+v ok=%v err=%v", cycle, ok, err)
	}

	for index := 0; index < roomMaxRootHandoffs; index++ {
		detect(workspacestore.RoomPublicHandoff{
			HandoffID:       "rh-overflow-" + string(rune('a'+index)),
			ConversationID:  conversationID,
			RootRoundID:     "root-overflow",
			SourceMessageID: "message-overflow-" + string(rune('a'+index)),
			SourceAgentID:   "agent-source-" + string(rune('a'+index)),
			TargetAgentID:   "agent-target-" + string(rune('a'+index)),
		})
	}
	overflowID := "rh-overflow-new"
	detect(workspacestore.RoomPublicHandoff{
		HandoffID: overflowID, ConversationID: conversationID, RootRoundID: "root-overflow",
		SourceMessageID: "message-overflow-new", SourceAgentID: "agent-source-new", TargetAgentID: "agent-target-new",
	})
	accepted, err = service.admitPublicMentionWakes(context.Background(), &activeRoomRound{
		ConversationID: conversationID, RootRoundID: "root-overflow", OwnerUserID: "owner",
	}, []publicMentionWake{{
		HandoffID: overflowID, QueueSource: protocol.InputQueueSourceAgentPublicMention,
		SourceAgentID: "agent-source-new", TargetAgentID: "agent-target-new",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(accepted) != 0 {
		t.Fatalf("root handoff 超限后不应继续接受新边: %+v", accepted)
	}
}

// 公共提及队列测试。

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
		slot := &activeRoomSlot{
			AgentID:           agent.AgentID,
			AgentRoundID:      agentRoundID,
			RuntimeSessionKey: protocol.BuildRoomAgentSessionKey(conversationID, agent.AgentID, protocol.RoomTypeGroup),
			WorkspacePath:     agent.WorkspacePath,
		}
		slot.setStatus("running")
		return slot
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
	service := &Service{
		inputQueue: store,
		runtime:    runtimeManager,
		permission: permissionctx.NewContext(),
		rounds: newRoomRoundRegistryFromRounds(map[string]*activeRoomRound{
			"root-a": {
				SessionKey: sharedSessionKey, ConversationID: conversationID, RootRoundID: "root-a",
				Slots: map[string]*activeRoomSlot{slotA.AgentID: slotA},
			},
			"root-b": {
				SessionKey: sharedSessionKey, ConversationID: conversationID, RootRoundID: "root-b",
				Slots: map[string]*activeRoomSlot{slotB.AgentID: slotB},
			},
		}),
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
	output, err := service.roomSlotGuidanceHook(service.rounds.findByRoundID(sharedSessionKey, "root-a"), slotA, locationA)(
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
	t.Setenv("NEXUS_STATE_ROOT", root)
	conversationID := "conversation-stable-public-user-message"
	roomID := "room-stable-public-user-message"
	sharedSessionKey := protocol.BuildRoomSharedSessionKey(conversationID)
	history := workspacestore.NewRoomHistoryStore(root)
	service := &Service{
		roomHistory: history,
		permission:  permissionctx.NewContext(),
	}
	contextValue := &protocol.ConversationContextAggregate{
		Room:         protocol.RoomRecord{ID: roomID, OwnerUserID: "owner", RoomType: protocol.RoomTypeGroup},
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

	messages, err := history.ReadMessages(contextValue.Room.OwnerUserID, conversationID, nil)
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

func TestLatestActiveRootRoundAgentIDsPrefersRegistrationSequence(t *testing.T) {
	service := &Service{rounds: newRoomRoundRegistry()}
	sessionKey := protocol.BuildRoomSharedSessionKey("conversation-latest-root")
	register := func(roundID string, agentID string) {
		slot := &activeRoomSlot{
			AgentID:      agentID,
			AgentRoundID: roundID + "-agent",
			TimestampMS:  100,
		}
		slot.setStatus("running")
		service.registerRound(&activeRoomRound{
			SessionKey:     sessionKey,
			ConversationID: "conversation-latest-root",
			RoundID:        roundID,
			RootRoundID:    roundID,
			Slots:          map[string]*activeRoomSlot{agentID: slot},
		})
	}

	// root id 的字典序故意与注册顺序相反，确保并列时间戳不参与主判定。
	register("a-earlier-root", "agent-earlier")
	register("z-later-root", "agent-later")

	got := service.latestActiveRootRoundAgentIDs(sessionKey, "conversation-latest-root")
	if want := []string{"agent-later"}; !slices.Equal(got, want) {
		t.Fatalf("最近活跃 root 目标 = %+v, want %+v", got, want)
	}
}

func TestResolveChatTargetAgentIDsUsesExplicitTargets(t *testing.T) {
	contextValue := &protocol.ConversationContextAggregate{
		Members: []protocol.MemberRecord{
			{MemberType: protocol.MemberTypeAgent, MemberAgentID: "agent-amy"},
			{MemberType: protocol.MemberTypeAgent, MemberAgentID: "agent-tom"},
		},
	}
	targets, resolution, err := resolveChatTargetAgentIDs(
		ChatRequest{Content: "没有 mention 也要给 Amy", TargetAgentIDs: []string{"agent-amy", "agent-amy", " "}},
		contextValue,
		map[string]string{"agent-amy": "Amy", "agent-tom": "Tom"},
	)
	if err != nil {
		t.Fatalf("显式 Room 目标解析失败: %v", err)
	}
	if resolution != "explicit_target" || len(targets) != 1 || targets[0] != "agent-amy" {
		t.Fatalf("显式 Room 目标解析不正确: targets=%+v resolution=%s", targets, resolution)
	}
}

func TestNormalizeRoomAgentIDsPreservesOrderAndDropsDuplicates(t *testing.T) {
	got := normalizeRoomAgentIDs([]string{" agent-b ", "", "agent-a", "agent-b", "agent-a"})
	if want := []string{"agent-b", "agent-a"}; !slices.Equal(got, want) {
		t.Fatalf("Room Agent ID 归一化结果 = %+v, want %+v", got, want)
	}
}

func TestNewRoomUserMessagePersistsResolvedTargets(t *testing.T) {
	message := newRoomUserMessage(
		ChatRequest{RoundID: "round-targets", UserMessageID: "message-targets", Content: "只调整 Agent1 的回复"},
		"room:group:conversation-targets",
		"room-targets",
		"conversation-targets",
		nil,
		[]string{"agent-1"},
		protocol.ChatDeliveryPolicyGuide,
	)
	targets, ok := message["target_agent_ids"].([]string)
	if !ok || len(targets) != 1 || targets[0] != "agent-1" {
		t.Fatalf("target_agent_ids = %#v, want resolved target", message["target_agent_ids"])
	}
}

func TestResolveChatTargetAgentIDsRejectsNonMemberTarget(t *testing.T) {
	contextValue := &protocol.ConversationContextAggregate{
		Members: []protocol.MemberRecord{
			{MemberType: protocol.MemberTypeAgent, MemberAgentID: "agent-amy"},
		},
	}
	_, _, err := resolveChatTargetAgentIDs(
		ChatRequest{Content: "整理一下", TargetAgentIDs: []string{"agent-outsider"}},
		contextValue,
		map[string]string{"agent-amy": "Amy"},
	)
	if err == nil || !strings.Contains(err.Error(), "not a room member") {
		t.Fatalf("非成员目标应被拒绝: %v", err)
	}
}

func TestBuildPublicMentionSlotKeepsPublicTriggerMessage(t *testing.T) {
	contextValue := &protocol.ConversationContextAggregate{
		Room:         protocol.RoomRecord{ID: "room-1", RoomType: protocol.RoomTypeGroup},
		Conversation: protocol.ConversationRecord{ID: "conversation-1"},
	}
	parentRound := &activeRoomRound{
		Context:     contextValue,
		OwnerUserID: "owner-public-mention",
		RoundID:     "handoff-source-round",
		RootRoundID: "persisted-handoff-root",
	}
	roundValue := newPublicMentionRound(parentRound, "room:group:conversation-1", "wake-round-1")
	slot := buildPublicMentionSlot(
		roundValue,
		contextValue,
		protocol.SessionRecord{ID: "session-devin"},
		&protocol.Agent{AgentID: "agent-devin", WorkspacePath: t.TempDir()},
		publicMentionWake{
			SourceAgentID: "agent-amy",
			TargetAgentID: "agent-devin",
			Content:       "@Devin @sam 谁先来？",
			MessageID:     "message-1",
		},
		"round-1",
		"message-slot-1",
		0,
	)

	if slot.Trigger.TriggerType != "public_mention" ||
		slot.Trigger.SourceAgentID != "agent-amy" ||
		slot.Trigger.TargetAgentID != "agent-devin" ||
		slot.Trigger.MessageID != "message-1" ||
		slot.Trigger.Content != "@Devin @sam 谁先来？" {
		t.Fatalf("公区 @ slot 应只保留可直接渲染成消息行的触发信息: %+v", slot.Trigger)
	}
	if slot.OwnerUserID != roundValue.OwnerUserID ||
		slot.GoalUsageScopeRoundID != roundValue.RootRoundID {
		t.Fatalf(
			"公区 @ slot Goal usage scope = owner:%q scope:%q, want owner:%q scope:%q",
			slot.OwnerUserID,
			slot.GoalUsageScopeRoundID,
			roundValue.OwnerUserID,
			roundValue.RootRoundID,
		)
	}
}
