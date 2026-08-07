package realtime

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

func TestPersistContextUsageUpdatesRoomAgentSessionMeta(t *testing.T) {
	storeRoot := t.TempDir()
	workspacePath := filepath.Join(storeRoot, "workspace", "agent-a")
	files := workspacestore.NewSessionFileStore(storeRoot)
	sessionKey := protocol.BuildRoomAgentSessionKey(
		"conversation-a",
		"agent-a",
		protocol.RoomTypeGroup,
	)
	now := time.Now().UTC()
	if _, err := files.UpsertSession(workspacePath, protocol.Session{
		SessionKey:   sessionKey,
		AgentID:      "agent-a",
		ChannelType:  protocol.SessionChannelWebSocket,
		ChatType:     "group",
		Status:       "closed",
		CreatedAt:    now.Add(-time.Hour),
		LastActivity: now.Add(-time.Minute),
		Title:        "保留标题",
		Options:      map[string]any{"model": "glm-4.5-air"},
	}); err != nil {
		t.Fatalf("写入 Room Agent Session meta 失败: %v", err)
	}
	execution := &slotExecution{
		service: &Service{files: files},
		round: &activeRoomRound{
			SessionKey:     protocol.BuildRoomSharedSessionKey("conversation-a"),
			RoomID:         "room-a",
			ConversationID: "conversation-a",
			RoomType:       protocol.RoomTypeGroup,
		},
		slot: &activeRoomSlot{
			AgentID:           "agent-a",
			RuntimeSessionKey: sessionKey,
			WorkspacePath:     workspacePath,
		},
	}
	usage := protocol.ContextUsageData{
		TotalTokens: 37_500,
		MaxTokens:   131_100,
		Percentage:  28.6,
		Model:       "glm-4.5-air",
	}

	if err := execution.persistContextUsage(usage); err != nil {
		t.Fatalf("persistContextUsage() error = %v", err)
	}
	persisted, _, err := files.FindSession([]string{workspacePath}, sessionKey)
	if err != nil {
		t.Fatalf("读取 Room Agent Session meta 失败: %v", err)
	}
	if persisted == nil || persisted.ContextUsage == nil ||
		*persisted.ContextUsage != usage {
		t.Fatalf("persisted context usage = %+v", persisted)
	}
	if persisted.Title != "保留标题" ||
		persisted.Options["model"] != "glm-4.5-air" {
		t.Fatalf("context usage 更新不应覆盖既有 Session 元数据: %+v", persisted)
	}
}

func TestPersistContextUsageCreatesMissingRoomAgentSessionMeta(t *testing.T) {
	storeRoot := t.TempDir()
	workspacePath := filepath.Join(storeRoot, "workspace", "agent-a")
	files := workspacestore.NewSessionFileStore(storeRoot)
	sessionKey := protocol.BuildRoomAgentSessionKey(
		"conversation-a",
		"agent-a",
		protocol.RoomTypeGroup,
	)
	now := time.Now().UTC()
	execution := &slotExecution{
		service: &Service{files: files},
		round: &activeRoomRound{
			SessionKey:     protocol.BuildRoomSharedSessionKey("conversation-a"),
			RoomID:         "room-a",
			ConversationID: "conversation-a",
			RoomType:       protocol.RoomTypeGroup,
			Context: &protocol.ConversationContextAggregate{
				Room: protocol.RoomRecord{
					ID:       "room-a",
					RoomType: protocol.RoomTypeGroup,
					Name:     "项目群",
				},
				Conversation: protocol.ConversationRecord{
					ID:             "conversation-a",
					RoomID:         "room-a",
					Title:          "上下文持久化",
					MessageCount:   7,
					LastActivityAt: now,
					CreatedAt:      now.Add(-time.Hour),
				},
				Sessions: []protocol.SessionRecord{{
					ID:             "room-session-a",
					ConversationID: "conversation-a",
					AgentID:        "agent-a",
					IsPrimary:      true,
					Options:        map[string]any{"model": "glm-4.5-air"},
					CreatedAt:      now.Add(-time.Hour),
				}},
			},
		},
		slot: &activeRoomSlot{
			RoomSessionID:     "room-session-a",
			AgentID:           "agent-a",
			RuntimeSessionKey: sessionKey,
			WorkspacePath:     workspacePath,
		},
	}
	usage := protocol.ContextUsageData{
		TotalTokens: 37_500,
		MaxTokens:   131_100,
		Percentage:  28.6,
		Model:       "glm-4.5-air",
	}

	if err := execution.persistContextUsage(usage); err != nil {
		t.Fatalf("persistContextUsage() error = %v", err)
	}
	persisted, _, err := files.FindSession([]string{workspacePath}, sessionKey)
	if err != nil {
		t.Fatalf("读取新建 Room Agent Session meta 失败: %v", err)
	}
	if persisted == nil ||
		persisted.ContextUsage == nil ||
		*persisted.ContextUsage != usage ||
		persisted.Title != "上下文持久化" ||
		persisted.MessageCount != 7 ||
		persisted.RoomSessionID == nil ||
		*persisted.RoomSessionID != "room-session-a" ||
		persisted.Options["model"] != "glm-4.5-air" {
		t.Fatalf("created Room Agent Session meta = %+v", persisted)
	}
}
