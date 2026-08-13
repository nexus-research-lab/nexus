// INPUT: 多 conversation group Room、目标成员与持久暂停/恢复请求。
// OUTPUT: 所有 conversation 读取同一 participation_paused 状态且非目标成员不受影响。
// POS: Room 成员参与状态事务的持久化回归测试。
package room_test

import (
	"context"
	"errors"
	"testing"
	"time"

	serverapp "github.com/nexus-research-lab/nexus/internal/app/server"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/storage/roomrepo"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"

	_ "modernc.org/sqlite"
)

func TestRoomServicePersistsMemberParticipationAcrossConversations(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := newRoomTestAgentService(t, cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := serverapp.NewRoomServiceWithDB(cfg, db, agentService)
	ctx := context.Background()
	agentA := createTestAgent(t, agentService, ctx, "暂停成员 A")
	agentB := createTestAgent(t, agentService, ctx, "继续成员 B")
	mainContext, err := roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs: []string{agentA.AgentID, agentB.AgentID},
		Name:     "成员参与状态测试",
	})
	if err != nil {
		t.Fatalf("创建 Room 失败: %v", err)
	}
	if err = roomService.MarkConversationStarted(
		ctx,
		mainContext.Conversation.ID,
		time.Now().UTC(),
	); err != nil {
		t.Fatalf("标记主 conversation 已开始失败: %v", err)
	}
	if _, err = roomService.CreateConversation(
		ctx,
		mainContext.Room.ID,
		protocol.CreateConversationRequest{Title: "第二话题"},
	); err != nil {
		t.Fatalf("创建第二 conversation 失败: %v", err)
	}
	beforeParticipation, err := roomService.GetRoom(ctx, mainContext.Room.ID)
	if err != nil {
		t.Fatalf("读取暂停前 Room 版本失败: %v", err)
	}

	updated, err := roomService.SetRoomMemberParticipation(
		ctx,
		mainContext.Room.ID,
		agentA.AgentID,
		true,
	)
	if err != nil {
		t.Fatalf("暂停 Room 成员失败: %v", err)
	}
	assertRoomMemberParticipation(t, updated.Members, agentA.AgentID, true)
	assertRoomMemberParticipation(t, updated.Members, agentB.AgentID, false)
	if updated.Room.ConfigurationVersion != beforeParticipation.Room.ConfigurationVersion+1 {
		t.Fatalf(
			"暂停成员 configuration_version = %d, want %d",
			updated.Room.ConfigurationVersion,
			beforeParticipation.Room.ConfigurationVersion+1,
		)
	}
	if updated.Room.AuthorityEpoch != beforeParticipation.Room.AuthorityEpoch+1 {
		t.Fatalf(
			"暂停成员 authority_epoch = %d, want %d",
			updated.Room.AuthorityEpoch,
			beforeParticipation.Room.AuthorityEpoch+1,
		)
	}
	if _, staleErr := roomService.SetRoomMemberParticipationAtVersion(
		ctx,
		mainContext.Room.ID,
		agentA.AgentID,
		false,
		beforeParticipation.Room.ConfigurationVersion,
	); !errors.Is(staleErr, roomrepo.ErrConfigurationVersionConflict) {
		t.Fatalf("过期成员参与状态写入 error = %v, want version conflict", staleErr)
	}

	contexts, err := roomService.GetRoomContexts(ctx, mainContext.Room.ID)
	if err != nil {
		t.Fatalf("重载 Room contexts 失败: %v", err)
	}
	if len(contexts) != 2 {
		t.Fatalf("Room contexts 数量 = %d, want 2", len(contexts))
	}
	for _, contextValue := range contexts {
		assertRoomMemberParticipation(t, contextValue.Members, agentA.AgentID, true)
		assertRoomMemberParticipation(t, contextValue.Members, agentB.AgentID, false)
	}

	resumed, err := roomService.SetRoomMemberParticipationAtVersion(
		ctx,
		mainContext.Room.ID,
		agentA.AgentID,
		false,
		updated.Room.ConfigurationVersion,
	)
	if err != nil {
		t.Fatalf("恢复 Room 成员失败: %v", err)
	}
	if resumed.Room.ConfigurationVersion != updated.Room.ConfigurationVersion+1 ||
		resumed.Room.AuthorityEpoch != updated.Room.AuthorityEpoch+1 {
		t.Fatalf("恢复成员未推进版本和权限世代: before=%+v after=%+v", updated.Room, resumed.Room)
	}
	contexts, err = roomService.GetRoomContexts(ctx, mainContext.Room.ID)
	if err != nil {
		t.Fatalf("恢复后重载 Room contexts 失败: %v", err)
	}
	for _, contextValue := range contexts {
		assertRoomMemberParticipation(t, contextValue.Members, agentA.AgentID, false)
		assertRoomMemberParticipation(t, contextValue.Members, agentB.AgentID, false)
	}
}

func TestRoomServiceHydratesConversationMessageCountFromCanonicalHistory(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := newRoomTestAgentService(t, cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := serverapp.NewRoomServiceWithDB(cfg, db, agentService)
	ctx := context.Background()
	agentA := createTestAgent(t, agentService, ctx, "消息计数成员 A")
	agentB := createTestAgent(t, agentService, ctx, "消息计数成员 B")
	roomContext, err := roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs: []string{agentA.AgentID, agentB.AgentID},
		Name:     "消息计数测试",
	})
	if err != nil {
		t.Fatalf("创建 Room 失败: %v", err)
	}
	history := workspacestore.NewRoomHistoryStore(cfg.WorkspacePath)
	for _, message := range []protocol.Message{
		{
			"message_id": "count-user",
			"round_id":   "count-round",
			"role":       "user",
			"content":    "/goal 校正消息计数",
			"timestamp":  int64(1000),
		},
		{
			"message_id":  "count-assistant",
			"round_id":    "count-round",
			"role":        "assistant",
			"content":     "开始处理",
			"is_complete": true,
			"stop_reason": "end_turn",
			"timestamp":   int64(1100),
		},
	} {
		if err = history.AppendInlineMessage(
			roomContext.Room.OwnerUserID,
			roomContext.Conversation.ID,
			message,
		); err != nil {
			t.Fatalf("写入 canonical Room 历史失败: %v", err)
		}
	}

	loaded, err := roomService.GetConversationContext(ctx, roomContext.Conversation.ID)
	if err != nil {
		t.Fatalf("读取 Room conversation 失败: %v", err)
	}
	if loaded.Conversation.MessageCount != 2 {
		t.Fatalf("conversation message_count = %d, want 2", loaded.Conversation.MessageCount)
	}
}

func assertRoomMemberParticipation(
	t *testing.T,
	members []protocol.MemberRecord,
	agentID string,
	wantPaused bool,
) {
	t.Helper()
	for _, member := range members {
		if member.MemberType != protocol.MemberTypeAgent || member.MemberAgentID != agentID {
			continue
		}
		if member.ParticipationPaused != wantPaused {
			t.Fatalf(
				"member %s participation_paused = %t, want %t",
				agentID,
				member.ParticipationPaused,
				wantPaused,
			)
		}
		return
	}
	t.Fatalf("Room members 缺少 agent %s: %+v", agentID, members)
}
