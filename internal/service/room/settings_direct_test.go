package room_test

import (
	"context"
	"strings"
	"testing"

	serverapp "github.com/nexus-research-lab/nexus/internal/app/server"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	skillspkg "github.com/nexus-research-lab/nexus/internal/service/skills"

	_ "modernc.org/sqlite"
)

func TestRoomServicePersistsRoomSkills(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := serverapp.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := serverapp.NewRoomServiceWithDB(cfg, db, agentService)
	roomService.SetSkillCatalog(fakeRoomSkillCatalog{
		"room-playbook": {
			Info: skillspkg.Info{
				Name:  "room-playbook",
				Title: "协作房间规则",
				Scope: skillspkg.ScopeRoom,
			},
			ReadmeMarkdown: "---\nname: room-playbook\n---\n\n# 协作房间规则\n\n房间规则正文",
		},
		"agent-only": {
			Info: skillspkg.Info{
				Name:  "agent-only",
				Title: "Agent Only",
				Scope: "any",
			},
		},
	})

	ctx := context.Background()
	agentA := createTestAgent(t, agentService, ctx, "测试助手A")
	agentB := createTestAgent(t, agentService, ctx, "测试助手B")

	mainContext, err := roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs:   []string{agentA.AgentID, agentB.AgentID},
		Name:       "Room Skill 测试",
		SkillNames: []string{"room-playbook", "room-playbook"},
	})
	if err != nil {
		t.Fatalf("创建带 room skill 的 room 失败: %v", err)
	}
	if got, want := mainContext.Room.SkillNames, []string{"room-playbook"}; len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("room skill_names 未按预期归一化: got=%#v want=%#v", got, want)
	}

	prompt, err := roomService.BuildRoomSkillPrompt(ctx, mainContext.Room.SkillNames)
	if err != nil {
		t.Fatalf("构造 room skill prompt 失败: %v", err)
	}
	if !strings.Contains(prompt, "房间规则正文") || strings.Contains(prompt, "name: room-playbook") {
		t.Fatalf("room skill prompt 内容不正确: %s", prompt)
	}

	emptySkills := []string{}
	mainContext, err = roomService.UpdateRoom(ctx, mainContext.Room.ID, protocol.UpdateRoomRequest{
		SkillNames: &emptySkills,
	})
	if err != nil {
		t.Fatalf("清空 room skill 失败: %v", err)
	}
	if len(mainContext.Room.SkillNames) != 0 {
		t.Fatalf("room skill 未清空: %#v", mainContext.Room.SkillNames)
	}

	if _, err = roomService.UpdateRoom(ctx, mainContext.Room.ID, protocol.UpdateRoomRequest{
		SkillNames: &[]string{"agent-only"},
	}); err == nil {
		t.Fatal("非 room scope skill 不应允许启用到 room")
	}
}

func TestRoomServiceValidatesRoomHostSettings(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := serverapp.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := serverapp.NewRoomServiceWithDB(cfg, db, agentService)

	ctx := context.Background()
	agentA := createTestAgent(t, agentService, ctx, "测试助手A")
	agentB := createTestAgent(t, agentService, ctx, "测试助手B")
	agentC := createTestAgent(t, agentService, ctx, "测试助手C")

	if _, err = roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs:             []string{agentA.AgentID, agentB.AgentID},
		Name:                 "无群主接管房间",
		HostAutoReplyEnabled: true,
	}); err == nil {
		t.Fatal("启用群主接管时必须要求设置群主")
	}

	if _, err = roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs:    []string{agentA.AgentID, agentB.AgentID},
		Name:        "群主非成员房间",
		HostAgentID: agentC.AgentID,
	}); err == nil {
		t.Fatal("非成员不应允许成为群主")
	}

	mainContext, err := roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs:             []string{agentA.AgentID, agentB.AgentID},
		Name:                 "群主校验房间",
		HostAgentID:          agentA.AgentID,
		HostAutoReplyEnabled: true,
	})
	if err != nil {
		t.Fatalf("创建 room 失败: %v", err)
	}

	updatedContext, err := roomService.RemoveRoomMember(ctx, mainContext.Room.ID, agentA.AgentID)
	if err != nil {
		t.Fatalf("移除群主失败: %v", err)
	}
	if updatedContext.Room.HostAgentID != "" || updatedContext.Room.HostAutoReplyEnabled {
		t.Fatalf("移除群主成员后应清空群主接管设置: %+v", updatedContext.Room)
	}
}

func TestRoomServiceAllowsMainAgentDirectRoom(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := serverapp.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := serverapp.NewRoomServiceWithDB(cfg, db, agentService)

	ctx := context.Background()
	if err = agentService.EnsureReady(ctx); err != nil {
		t.Fatalf("初始化主智能体失败: %v", err)
	}

	dmContext, err := roomService.EnsureDirectRoom(ctx, cfg.DefaultAgentID)
	if err != nil {
		t.Fatalf("主智能体直聊创建失败: %v", err)
	}
	if dmContext.Room.RoomType != protocol.RoomTypeDM {
		t.Fatalf("主智能体直聊类型不正确: got=%s", dmContext.Room.RoomType)
	}
	if len(dmContext.Sessions) != 1 {
		t.Fatalf("主智能体直聊 session 数量不正确: got=%d want=1", len(dmContext.Sessions))
	}
	if dmContext.Sessions[0].AgentID != cfg.DefaultAgentID {
		t.Fatalf("主智能体直聊 session agent_id 不正确: got=%s want=%s", dmContext.Sessions[0].AgentID, cfg.DefaultAgentID)
	}

	reusedContext, err := roomService.EnsureDirectRoom(ctx, cfg.DefaultAgentID)
	if err != nil {
		t.Fatalf("复用主智能体直聊失败: %v", err)
	}
	if reusedContext.Room.ID != dmContext.Room.ID {
		t.Fatalf("主智能体直聊未复用既有 room: got=%s want=%s", reusedContext.Room.ID, dmContext.Room.ID)
	}
	if reusedContext.Conversation.ID != dmContext.Conversation.ID {
		t.Fatalf("主智能体直聊未复用既有对话: got=%s want=%s", reusedContext.Conversation.ID, dmContext.Conversation.ID)
	}
}

func TestRoomServiceEnsureDirectRoomReturnsLatestConversation(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := serverapp.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := serverapp.NewRoomServiceWithDB(cfg, db, agentService)

	ctx := context.Background()
	if err = agentService.EnsureReady(ctx); err != nil {
		t.Fatalf("初始化主智能体失败: %v", err)
	}

	dmContext, err := roomService.EnsureDirectRoom(ctx, cfg.DefaultAgentID)
	if err != nil {
		t.Fatalf("主智能体直聊创建失败: %v", err)
	}

	nextContext, err := roomService.CreateConversation(ctx, dmContext.Room.ID, protocol.CreateConversationRequest{
		Title: "新的主智能体会话",
	})
	if err != nil {
		t.Fatalf("创建新的 DM 对话失败: %v", err)
	}
	if _, err = db.ExecContext(ctx, `
UPDATE sessions
SET last_activity_at = datetime('now', '+1 hour'), updated_at = datetime('now', '+1 hour')
WHERE conversation_id = ?`, nextContext.Conversation.ID); err != nil {
		t.Fatalf("标记最新 session 失败: %v", err)
	}

	reusedContext, err := roomService.EnsureDirectRoom(ctx, cfg.DefaultAgentID)
	if err != nil {
		t.Fatalf("复用主智能体直聊失败: %v", err)
	}
	if reusedContext.Room.ID != dmContext.Room.ID {
		t.Fatalf("主智能体直聊未复用既有 room: got=%s want=%s", reusedContext.Room.ID, dmContext.Room.ID)
	}
	if reusedContext.Conversation.ID != nextContext.Conversation.ID {
		t.Fatalf("主智能体直聊未返回最新对话: got=%s want=%s", reusedContext.Conversation.ID, nextContext.Conversation.ID)
	}
}

func TestRoomServiceRejectsMainAgentAsGroupMember(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := serverapp.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := serverapp.NewRoomServiceWithDB(cfg, db, agentService)

	ctx := context.Background()
	agentA := createTestAgent(t, agentService, ctx, "分组测试助手A")

	if _, err = roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs: []string{cfg.DefaultAgentID, agentA.AgentID},
	}); err == nil {
		t.Fatal("group room 不应允许主智能体作为成员")
	}
	if _, err = roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs: []string{cfg.DefaultAgentID},
	}); err == nil {
		t.Fatal("仅主智能体不应创建 group room")
	}
}
