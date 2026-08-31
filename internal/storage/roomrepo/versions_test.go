// INPUT: Room 创建、共享配置与 host 版本推进操作。
// OUTPUT: configuration_version/authority_epoch 初值、持久化和递增语义验证。
// POS: Room 资源版本持久层回归测试。
package roomrepo

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

func TestRoomConfigurationVersionsPersistAndAdvance(t *testing.T) {
	databaseURL := filepath.Join(t.TempDir(), "nexus.db")
	db, err := sql.Open("sqlite", databaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err = goose.SetDialect("sqlite3"); err != nil {
		t.Fatalf("设置 goose 方言失败: %v", err)
	}
	if err = goose.Up(db, roomrepoMigrationDir(t)); err != nil {
		t.Fatalf("执行 migration 失败: %v", err)
	}

	repository := NewSQLRepository("sqlite", db)
	roomContext, err := repository.CreateRoom(context.Background(), CreateRoomBundle{
		Room: protocol.RoomRecord{
			ID:          "room-version-test",
			OwnerUserID: "owner-version-test",
			RoomType:    protocol.RoomTypeGroup,
			Name:        "版本测试房间",
			Description: "初始描述",
			SkillNames:  []string{},
		},
		Conversation: protocol.ConversationRecord{
			ID:               "conversation-version-test",
			RoomID:           "room-version-test",
			ConversationType: protocol.ConversationTypeMain,
			Title:            "版本测试房间",
		},
	})
	if err != nil {
		t.Fatalf("创建 room 失败: %v", err)
	}
	assertStoredRoomVersions(t, roomContext.Room, 1, 1)

	if _, err = db.Exec(`
INSERT INTO agents (
    id, owner_user_id, slug, name, description, definition, status,
    workspace_path, is_main, vibe_tags, business_tags
) VALUES (?, ?, ?, ?, '', '', 'active', ?, 0, '["沉稳"]', '["企业"]')`,
		"agent-version-test",
		"owner-version-test",
		"agent-version-test",
		"版本测试助手",
		t.TempDir(),
	); err != nil {
		t.Fatalf("创建 host 测试 Agent 失败: %v", err)
	}
	if _, err = db.Exec(`
INSERT INTO members (id, room_id, member_type, member_agent_id)
VALUES (?, ?, 'agent', ?)`,
		"member-version-test",
		"room-version-test",
		"agent-version-test",
	); err != nil {
		t.Fatalf("创建 host 测试成员失败: %v", err)
	}

	description := "更新后的描述"
	roomContext, err = repository.UpdateRoom(
		context.Background(),
		"owner-version-test",
		"room-version-test",
		UpdateRoomPatch{Description: &description},
	)
	if err != nil {
		t.Fatalf("更新 room 共享配置失败: %v", err)
	}
	if len(roomContext.MemberAgents) != 1 ||
		len(roomContext.MemberAgents[0].BusinessTags) != 1 ||
		roomContext.MemberAgents[0].BusinessTags[0] != "企业" ||
		len(roomContext.MemberAgents[0].VibeTags) != 1 ||
		roomContext.MemberAgents[0].VibeTags[0] != "沉稳" {
		t.Fatalf("Room 成员投影应区分业务标签与风格标签: %+v", roomContext.MemberAgents)
	}
	assertStoredRoomVersions(t, roomContext.Room, 2, 1)

	hostAgentID := "agent-version-test"
	roomContext, err = repository.UpdateRoom(
		context.Background(),
		"owner-version-test",
		"room-version-test",
		UpdateRoomPatch{HostAgentID: &hostAgentID},
	)
	if err != nil {
		t.Fatalf("更新 room host 失败: %v", err)
	}
	assertStoredRoomVersions(t, roomContext.Room, 3, 2)

	roomContext, err = repository.UpdateRoom(
		context.Background(),
		"owner-version-test",
		"room-version-test",
		UpdateRoomPatch{HostAgentID: &hostAgentID},
	)
	if err != nil {
		t.Fatalf("重复写入 room host 失败: %v", err)
	}
	assertStoredRoomVersions(t, roomContext.Room, 4, 2)

	expectedVersion := roomContext.Room.ConfigurationVersion
	conversationContext, err := repository.CreateConversation(
		context.Background(),
		CreateConversationBundle{
			OwnerUserID: "owner-version-test",
			RoomID:      "room-version-test",
			Conversation: protocol.ConversationRecord{
				ID:               "conversation-version-topic",
				RoomID:           "room-version-test",
				ConversationType: protocol.ConversationTypeTopic,
				Title:            "版本化话题",
			},
			ExpectedConfigurationVersion: &expectedVersion,
		},
	)
	if err != nil {
		t.Fatalf("创建 conversation 失败: %v", err)
	}
	assertStoredRoomVersions(t, conversationContext.Room, 5, 2)

	if _, err = repository.UpdateConversationAtVersion(
		context.Background(),
		"owner-version-test",
		"room-version-test",
		"conversation-version-topic",
		"过期计划不应写入",
		expectedVersion,
	); !errors.Is(err, ErrConfigurationVersionConflict) {
		t.Fatalf("stale conversation update err = %v, want version conflict", err)
	}
	conversationContext, err = repository.UpdateConversationAtVersion(
		context.Background(),
		"owner-version-test",
		"room-version-test",
		"conversation-version-topic",
		"新标题",
		5,
	)
	if err != nil {
		t.Fatalf("更新 conversation 失败: %v", err)
	}
	if conversationContext.Conversation.Title != "新标题" {
		t.Fatalf("conversation title = %q, want 新标题", conversationContext.Conversation.Title)
	}
	assertStoredRoomVersions(t, conversationContext.Room, 6, 2)

	fallbackContext, err := repository.DeleteConversationAtVersion(
		context.Background(),
		"owner-version-test",
		"room-version-test",
		"conversation-version-topic",
		6,
	)
	if err != nil {
		t.Fatalf("删除 conversation 失败: %v", err)
	}
	if fallbackContext == nil ||
		fallbackContext.Conversation.ID != "conversation-version-test" {
		t.Fatalf("unexpected conversation fallback: %+v", fallbackContext)
	}
	assertStoredRoomVersions(t, fallbackContext.Room, 7, 2)
}

func assertStoredRoomVersions(t *testing.T, roomValue protocol.RoomRecord, configurationVersion int64, authorityEpoch int64) {
	t.Helper()

	if roomValue.ConfigurationVersion != configurationVersion || roomValue.AuthorityEpoch != authorityEpoch {
		t.Fatalf(
			"room versions = configuration:%d authority:%d, want configuration:%d authority:%d",
			roomValue.ConfigurationVersion,
			roomValue.AuthorityEpoch,
			configurationVersion,
			authorityEpoch,
		)
	}
}

func roomrepoMigrationDir(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("定位测试文件失败")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "db", "migrations", "sqlite")
}
