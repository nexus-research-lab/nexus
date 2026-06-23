package room_test

import (
	"context"
	"strings"
	"sync"
	"testing"

	serverapp "github.com/nexus-research-lab/nexus/internal/app/server"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	roomsvc "github.com/nexus-research-lab/nexus/internal/service/room"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
	_ "modernc.org/sqlite"
)

func TestRealtimeServiceHandleInterruptCancelsAllSlots(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := serverapp.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := serverapp.NewRoomServiceWithDB(cfg, db, agentService)
	if err != nil {
		t.Fatalf("创建 room service 失败: %v", err)
	}

	ctx := context.Background()
	agentA := createTestAgent(t, agentService, ctx, "助手甲")
	agentB := createTestAgent(t, agentService, ctx, "助手乙")
	roomContext, err := roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs: []string{agentA.AgentID, agentB.AgentID},
		Name:     "中断测试房间",
		Title:    "主对话",
	})
	if err != nil {
		t.Fatalf("创建 room 失败: %v", err)
	}

	clientA := newFakeRoomClient()
	clientA.onQuery = func(_ context.Context, _ string) error {
		return nil
	}
	clientA.onInterrupt = func(_ context.Context) {
		go func() {
			clientA.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeResult,
				SessionID: clientA.sessionID,
				UUID:      "room-interrupted-a",
				Result: &sdkprotocol.ResultMessage{
					Subtype:    "interrupted",
					DurationMS: 1,
					NumTurns:   1,
				},
			}
		}()
	}
	clientB := newFakeRoomClient()
	clientB.onQuery = func(_ context.Context, _ string) error {
		return nil
	}
	clientB.onInterrupt = func(_ context.Context) {
		go func() {
			clientB.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeResult,
				SessionID: clientB.sessionID,
				UUID:      "room-interrupted-b",
				Result: &sdkprotocol.ResultMessage{
					Subtype:    "interrupted",
					DurationMS: 1,
					NumTurns:   1,
				},
			}
		}()
	}

	permission := permissionctx.NewContext()
	runtimeManager := runtimectx.NewManager()
	service := NewRealtimeServiceWithFactory(
		cfg,
		roomService,
		agentService,
		runtimeManager,
		permission,
		&fakeRoomFactory{clients: []*fakeRoomClient{clientA, clientB}},
	)
	roomHistory := workspacestore.NewRoomHistoryStore(cfg.WorkspacePath)

	sharedSessionKey := protocol.BuildRoomSharedSessionKey(roomContext.Conversation.ID)
	sender := newRealtimeTestSender("room-sender-2")
	permission.BindSession(sharedSessionKey, sender)

	if err = service.HandleChat(ctx, roomsvc.ChatRequest{
		SessionKey:     sharedSessionKey,
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
		Content:        "@助手甲 @助手乙 处理一下",
		RoundID:        "room-round-2",
		ReqID:          "room-round-2",
	}); err != nil {
		t.Fatalf("HandleChat 失败: %v", err)
	}

	_ = collectRoomEventsUntil(t, sender.events, func(events []protocol.EventMessage, event protocol.EventMessage) bool {
		return countEventType(events, protocol.EventTypeStreamStart) >= 2
	})

	if err = service.HandleInterrupt(ctx, roomsvc.InterruptRequest{SessionKey: sharedSessionKey}); err != nil {
		t.Fatalf("HandleInterrupt 失败: %v", err)
	}

	events := collectRoomEventsUntil(t, sender.events, func(events []protocol.EventMessage, event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus && event.Data["status"] == "interrupted"
	})
	if countRoomResultSubtype(events, "interrupted") < 2 {
		t.Fatalf("期望每个 slot 都产出 interrupted result: %+v", events)
	}

	clientA.mu.Lock()
	interruptA := clientA.interruptCalls
	clientA.mu.Unlock()
	clientB.mu.Lock()
	interruptB := clientB.interruptCalls
	clientB.mu.Unlock()
	if interruptA == 0 || interruptB == 0 {
		t.Fatalf("所有 slot 都应收到 interrupt: a=%d b=%d", interruptA, interruptB)
	}

	sharedMessages, err := roomHistory.ReadMessages(roomContext.Conversation.ID, nil)
	if err != nil {
		t.Fatalf("读取中断后的共享 Room 消息失败: %v", err)
	}
	sharedInterrupted := 0
	for _, message := range sharedMessages {
		summary, ok := message["result_summary"].(map[string]any)
		if ok && summary["subtype"] == "interrupted" {
			sharedInterrupted++
		}
	}
	if sharedInterrupted < 2 {
		t.Fatalf("共享日志未完整落 interrupted result: %+v", sharedMessages)
	}

	for _, agentValue := range []*protocol.Agent{agentA, agentB} {
		privateSessionKey := protocol.BuildRoomAgentSessionKey(roomContext.Conversation.ID, agentValue.AgentID, roomContext.Room.RoomType)
		writeRoomTranscriptFixture(t, agentValue.WorkspacePath, "room-sdk-session", []map[string]any{
			{
				"type":      "user",
				"uuid":      "interrupt-user-" + agentValue.AgentID,
				"sessionId": "room-sdk-session",
				"timestamp": "2026-04-19T18:20:00Z",
				"message": map[string]any{
					"role":    "user",
					"content": "dispatch prompt",
				},
			},
		})
		privateMessages := readRoomPrivateHistory(
			t,
			cfg.WorkspacePath,
			agentValue.WorkspacePath,
			privateSessionKey,
			agentValue.AgentID,
			"room-sdk-session",
		)
		foundInterrupted := false
		for _, message := range privateMessages {
			summary, ok := message["result_summary"].(map[string]any)
			if ok && summary["subtype"] == "interrupted" {
				foundInterrupted = true
				break
			}
		}
		if !foundInterrupted {
			t.Fatalf("私有日志未落 interrupted result: agent=%s messages=%+v", agentValue.AgentID, privateMessages)
		}
	}
}

func TestRealtimeServiceTreatsClosedStreamAfterInterruptAsInterrupted(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := serverapp.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := serverapp.NewRoomServiceWithDB(cfg, db, agentService)
	if err != nil {
		t.Fatalf("创建 room service 失败: %v", err)
	}

	ctx := context.Background()
	agentValue := createTestAgent(t, agentService, ctx, "助手甲")
	roomContext, err := roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs: []string{agentValue.AgentID},
		Name:     "中断关流测试房间",
		Title:    "主对话",
	})
	if err != nil {
		t.Fatalf("创建 room 失败: %v", err)
	}

	client := newFakeRoomClient()
	client.onQuery = func(_ context.Context, _ string) error {
		return nil
	}
	var closeOnce sync.Once
	client.onInterrupt = func(_ context.Context) {
		closeOnce.Do(func() {
			close(client.messages)
		})
	}

	permission := permissionctx.NewContext()
	service := NewRealtimeServiceWithFactory(
		cfg,
		roomService,
		agentService,
		runtimectx.NewManager(),
		permission,
		&fakeRoomFactory{clients: []*fakeRoomClient{client}},
	)
	roomHistory := workspacestore.NewRoomHistoryStore(cfg.WorkspacePath)

	sharedSessionKey := protocol.BuildRoomSharedSessionKey(roomContext.Conversation.ID)
	sender := newRealtimeTestSender("room-sender-interrupt-closed-stream")
	permission.BindSession(sharedSessionKey, sender)

	if err = service.HandleChat(ctx, roomsvc.ChatRequest{
		SessionKey:     sharedSessionKey,
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
		Content:        "@助手甲 处理一下",
		RoundID:        "room-round-closed-stream",
		ReqID:          "room-round-closed-stream",
	}); err != nil {
		t.Fatalf("HandleChat 失败: %v", err)
	}

	_ = collectRoomEventsUntil(t, sender.events, func(events []protocol.EventMessage, event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeStreamStart
	})

	if err = service.HandleInterrupt(ctx, roomsvc.InterruptRequest{SessionKey: sharedSessionKey}); err != nil {
		t.Fatalf("HandleInterrupt 失败: %v", err)
	}

	events := collectRoomEventsUntil(t, sender.events, func(events []protocol.EventMessage, event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus &&
			(event.Data["status"] == "interrupted" || event.Data["status"] == "error")
	})
	terminalStatus := ""
	for _, event := range events {
		if event.EventType == protocol.EventTypeRoundStatus {
			terminalStatus = anyToString(event.Data["status"])
		}
	}
	if terminalStatus != "interrupted" {
		t.Fatalf("主动中断后的关流应归类为 interrupted，实际 status=%s events=%+v", terminalStatus, events)
	}
	if countRoomResultSubtype(events, "error") > 0 {
		t.Fatalf("主动中断后的关流不应广播 error result: %+v", events)
	}

	sharedMessages, err := roomHistory.ReadMessages(roomContext.Conversation.ID, nil)
	if err != nil {
		t.Fatalf("读取中断后的共享 Room 消息失败: %v", err)
	}
	foundInterrupted := false
	for _, message := range sharedMessages {
		summary, ok := message["result_summary"].(map[string]any)
		if !ok {
			continue
		}
		if summary["subtype"] == "error" {
			t.Fatalf("主动中断后的共享日志不应落 error summary: %+v", sharedMessages)
		}
		if summary["subtype"] == "interrupted" {
			foundInterrupted = true
			if strings.Contains(anyToString(summary["result"]), "round stream closed before terminal") {
				t.Fatalf("interrupted summary 不应暴露底层 stream 错误: %+v", summary)
			}
		}
	}
	if !foundInterrupted {
		t.Fatalf("共享日志未落 interrupted summary: %+v", sharedMessages)
	}
}
