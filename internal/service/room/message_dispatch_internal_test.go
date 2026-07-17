package room

import (
	"context"
	"testing"

	roomdomain "github.com/nexus-research-lab/nexus/internal/chat/room"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
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
	service := &RealtimeService{activeRounds: map[string]*activeRoomRound{}}
	roundValue := &activeRoomRound{
		ConversationID: "conversation-1",
		RoundID:        "round-child",
		RootRoundID:    "round-root",
		HopIndex:       3,
		Slots: map[string]*activeRoomSlot{
			"slot-1": {AgentID: "agent-1"},
		},
	}
	service.activeRounds["active"] = roundValue
	root, cause, hop := service.resolveRoomMessageCausality("conversation-1", "agent-1", "round-root")
	if root != "round-root" || cause != "round-child" || hop != 3 {
		t.Fatalf("工具消息未继承当前 Room 因果链: root=%s cause=%s hop=%d", root, cause, hop)
	}
}

func TestPublicInputBatchIgnoresStoredCursorWhenRuntimeCannotResume(t *testing.T) {
	workspacePath := t.TempDir()
	history := workspacestore.NewAgentHistoryStore(t.TempDir())
	service := &RealtimeService{history: history}
	roundValue := &activeRoomRound{ConversationID: "conversation-1"}
	slot := &activeRoomSlot{
		AgentID:           "agent-1",
		AgentRoundID:      "agent-round-1",
		RuntimeSessionKey: "agent:agent-1:ws:group:conversation-1",
		WorkspacePath:     workspacePath,
		ContextColdStart:  true,
	}
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

	slot.ContextColdStart = false
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
