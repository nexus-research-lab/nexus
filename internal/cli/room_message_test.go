package cli

import (
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

func TestRoomMessageListAndCursorsUseDirectedMessageStore(t *testing.T) {
	cfg := newCLITestConfig(t)
	migrateCLISQLite(t, cfg.DatabaseURL)

	store := workspacestore.NewRoomDirectedMessageStore(cfg.WorkspacePath)
	if err := store.AppendMessage(protocol.RoomDirectedMessageRecord{
		MessageID:      "message-1",
		RoomID:         "room-1",
		ConversationID: "conversation-1",
		SourceAgentID:  "agent-amy",
		Recipients:     []string{"agent-devin"},
		Content:        "第一条",
		WakePolicy:     protocol.RoomWakePolicyNone,
		ReplyRoute:     protocol.RoomReplyRoute{Mode: protocol.RoomReplyRouteNone},
		Timestamp:      1000,
	}); err != nil {
		t.Fatalf("写入第一条 directed message 失败: %v", err)
	}
	if err := store.AppendMessage(protocol.RoomDirectedMessageRecord{
		MessageID:      "message-2",
		RoomID:         "room-1",
		ConversationID: "conversation-1",
		SourceAgentID:  "agent-sam",
		Recipients:     []string{"agent-amy"},
		Content:        "第二条",
		WakePolicy:     protocol.RoomWakePolicyNone,
		ReplyRoute: protocol.RoomReplyRoute{
			Mode:       protocol.RoomReplyRoutePrivate,
			Recipients: []string{"agent-devin"},
			WakePolicy: protocol.RoomWakePolicyNone,
		},
		Timestamp: 2000,
	}); err != nil {
		t.Fatalf("写入第二条 directed message 失败: %v", err)
	}
	if err := store.AppendMessageCursor(workspacestore.RoomDirectedMessageCursor{
		RoomID:               "room-1",
		ConversationID:       "conversation-1",
		AgentID:              "agent-devin",
		RoundID:              "round-1",
		LastMessageID:        "message-1",
		LastMessageTimestamp: 1000,
		Timestamp:            3000,
	}); err != nil {
		t.Fatalf("写入 directed message cursor 失败: %v", err)
	}

	payload := runCLICommand(
		t,
		cfg,
		"--json",
		"room",
		"message",
		"list",
		"--conversation-id",
		"conversation-1",
		"--agent-id",
		"agent-devin",
		"--include-content",
	)
	if payload["action"] != "room_message_list" || payload["count"] != float64(2) {
		t.Fatalf("room message list 输出不正确: %+v", payload)
	}
	items, ok := payload["items"].([]any)
	if !ok || len(items) != 2 {
		t.Fatalf("room message list items 不正确: %+v", payload)
	}
	if first, _ := items[0].(map[string]any); first["content"] != "第一条" {
		t.Fatalf("room message list 应输出 include-content 正文: %+v", items)
	}

	payload = runCLICommand(
		t,
		cfg,
		"--json",
		"room",
		"message",
		"list",
		"--conversation-id",
		"conversation-1",
		"--agent-id",
		"agent-devin",
		"--after-cursor",
		"--include-content",
	)
	if payload["action"] != "room_message_list" || payload["count"] != float64(1) {
		t.Fatalf("room message list --after-cursor 输出不正确: %+v", payload)
	}
	afterCursorItems, ok := payload["items"].([]any)
	if !ok || len(afterCursorItems) != 1 {
		t.Fatalf("room message list --after-cursor items 不正确: %+v", payload)
	}
	if only, _ := afterCursorItems[0].(map[string]any); only["message_id"] != "message-2" {
		t.Fatalf("room message list --after-cursor 应只返回 cursor 后消息: %+v", afterCursorItems)
	}

	payload = runCLICommand(
		t,
		cfg,
		"--json",
		"room",
		"message",
		"cursors",
		"--conversation-id",
		"conversation-1",
		"--agent-id",
		"agent-devin",
	)
	if payload["action"] != "room_message_cursors" || payload["count"] != float64(1) {
		t.Fatalf("room message cursors 输出不正确: %+v", payload)
	}
	cursorItems, ok := payload["items"].([]any)
	if !ok || len(cursorItems) != 1 {
		t.Fatalf("room message cursors items 不正确: %+v", payload)
	}
	if cursor, _ := cursorItems[0].(map[string]any); cursor["last_message_id"] != "message-1" {
		t.Fatalf("room message cursors 应输出 last_message_id: %+v", cursorItems)
	}
}
