package room_test

import (
	"context"
	"errors"
	"testing"

	serverapp "github.com/nexus-research-lab/nexus/internal/app/server"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	roomsvc "github.com/nexus-research-lab/nexus/internal/service/room"
	memorysvc "github.com/nexus-research-lab/nexus/internal/workspace/memory"

	_ "modernc.org/sqlite"
)

func TestRoomServiceSharedMemoryCRUD(t *testing.T) {
	cfg := newRoomTestConfig(t)
	cfg.MemoryEnabled = true
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := serverapp.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := serverapp.NewRoomServiceWithDB(cfg, db, agentService)

	ctx := context.Background()
	agentValue := createTestAgent(t, agentService, ctx, "记忆助手")
	roomContext, err := roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs: []string{agentValue.AgentID},
		Name:     "记忆测试 room",
	})
	if err != nil {
		t.Fatalf("创建 room 失败: %v", err)
	}

	created, err := roomService.AddRoomSharedMemory(ctx, roomContext.Room.ID, roomContext.Conversation.ID, memorysvc.MemoryWriteInput{
		Title:    "共享决策",
		Content:  "Room 决定优先实现共享记忆面板",
		Kind:     "LRN",
		Category: "room",
		Status:   "candidate",
		Priority: "high",
	})
	if err != nil {
		t.Fatalf("新增 room shared memory 失败: %v", err)
	}
	if created.Scope != "room_shared:"+roomContext.Room.ID+":"+roomContext.Conversation.ID {
		t.Fatalf("shared memory scope 不正确: %q", created.Scope)
	}
	if created.Source != "room_manual" {
		t.Fatalf("shared memory source 不正确: %q", created.Source)
	}

	items, err := roomService.ListRoomSharedMemory(ctx, roomContext.Room.ID, roomContext.Conversation.ID, 20, nil)
	if err != nil {
		t.Fatalf("读取 shared memory 失败: %v", err)
	}
	if len(items) != 1 || items[0].EntryID != created.EntryID {
		t.Fatalf("shared memory 列表不符合预期: %#v", items)
	}

	stats, err := roomService.RoomSharedMemoryStats(ctx, roomContext.Room.ID, roomContext.Conversation.ID)
	if err != nil {
		t.Fatalf("读取 shared memory stats 失败: %v", err)
	}
	if stats.Total != 1 || stats.Candidate != 1 {
		t.Fatalf("shared memory stats 不符合预期: %#v", stats)
	}

	updated, err := roomService.UpdateRoomSharedMemory(ctx, roomContext.Room.ID, roomContext.Conversation.ID, created.EntryID, memorysvc.MemoryWriteInput{
		Title:   "共享决策更新",
		Content: "Room 决定先提供 MVP，再补成员 session memory 只读展示",
		Status:  "active",
	})
	if err != nil {
		t.Fatalf("更新 shared memory 失败: %v", err)
	}
	if updated.Title != "共享决策更新" || updated.Status != "active" {
		t.Fatalf("shared memory 更新结果不符合预期: %#v", updated)
	}

	err = roomService.DeleteRoomSharedMemory(ctx, roomContext.Room.ID, roomContext.Conversation.ID, created.EntryID)
	if err != nil {
		t.Fatalf("删除 shared memory 失败: %v", err)
	}
	items, err = roomService.ListRoomSharedMemory(ctx, roomContext.Room.ID, roomContext.Conversation.ID, 20, nil)
	if err != nil {
		t.Fatalf("删除后读取 shared memory 失败: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("删除后 shared memory 应为空: %#v", items)
	}

	_, err = roomService.ListRoomSharedMemory(ctx, "wrong-room", roomContext.Conversation.ID, 20, nil)
	if !errors.Is(err, roomsvc.ErrConversationNotFound) {
		t.Fatalf("错误 room_id 应返回 ErrConversationNotFound，实际: %v", err)
	}
}

func TestRoomServiceListsMemberSessionMemory(t *testing.T) {
	cfg := newRoomTestConfig(t)
	cfg.MemoryEnabled = true
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := serverapp.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := serverapp.NewRoomServiceWithDB(cfg, db, agentService)

	ctx := context.Background()
	agentA := createTestAgent(t, agentService, ctx, "成员 A")
	agentB := createTestAgent(t, agentService, ctx, "成员 B")
	roomContext, err := roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs: []string{agentA.AgentID, agentB.AgentID},
		Name:     "成员记忆测试 room",
	})
	if err != nil {
		t.Fatalf("创建 room 失败: %v", err)
	}

	agentScope := memorysvc.MemoryScope{
		Kind:           memorysvc.ScopeKindRoomAgentSession,
		UserID:         roomContext.Room.OwnerUserID,
		AgentID:        agentA.AgentID,
		SessionKey:     protocol.BuildRoomAgentSessionKey(roomContext.Conversation.ID, agentA.AgentID, roomContext.Room.RoomType),
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
	}
	agentEngine := memorysvc.NewEngine(agentA.WorkspacePath, memorysvc.DefaultOptions())
	created, err := agentEngine.Add(ctx, agentScope, memorysvc.MemoryWriteInput{
		Title:    "成员 A 私有结论",
		Content:  "成员 A 记录了只对自己 session 有意义的分析线索",
		Kind:     "LRN",
		Category: "room",
		Status:   "candidate",
		Priority: "medium",
		Source:   "room_auto",
	})
	if err != nil {
		t.Fatalf("写入成员 session memory 失败: %v", err)
	}

	groups, err := roomService.ListRoomAgentSessionMemory(ctx, roomContext.Room.ID, roomContext.Conversation.ID, 20, nil)
	if err != nil {
		t.Fatalf("读取成员 session memory 失败: %v", err)
	}
	if len(groups) != 2 {
		t.Fatalf("期望两个成员分组，实际: %#v", groups)
	}

	groupA := findRoomAgentMemoryGroup(groups, agentA.AgentID)
	groupB := findRoomAgentMemoryGroup(groups, agentB.AgentID)
	if groupA == nil || groupB == nil {
		t.Fatalf("成员分组缺失: %#v", groups)
	}
	if len(groupA.Items) != 1 || groupA.Items[0].EntryID != created.EntryID {
		t.Fatalf("成员 A memory 不符合预期: %#v", groupA)
	}
	if groupA.Items[0].Scope != agentScope.Key() {
		t.Fatalf("成员 A memory scope 不正确: %q", groupA.Items[0].Scope)
	}
	if len(groupB.Items) != 0 {
		t.Fatalf("成员 B 不应读取到成员 A memory: %#v", groupB.Items)
	}
}

func findRoomAgentMemoryGroup(groups []roomsvc.RoomAgentMemoryGroup, agentID string) *roomsvc.RoomAgentMemoryGroup {
	for index := range groups {
		if groups[index].AgentID == agentID {
			return &groups[index]
		}
	}
	return nil
}
