package room_test

import (
	"context"
	"strings"
	"testing"
	"time"

	serverapp "github.com/nexus-research-lab/nexus/internal/app/server"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestRoomServicePromotesFallbackWhenDeletingMainConversation(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := serverapp.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := serverapp.NewRoomServiceWithDB(cfg, db, agentService)

	ctx := context.Background()
	agentA := createTestAgent(t, agentService, ctx, "主对话删除助手A")
	agentB := createTestAgent(t, agentService, ctx, "主对话删除助手B")
	mainContext, err := roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs: []string{agentA.AgentID, agentB.AgentID},
		Name:     "主对话删除测试",
	})
	if err != nil {
		t.Fatalf("创建 room 失败: %v", err)
	}
	if err = roomService.MarkConversationStarted(ctx, mainContext.Conversation.ID, time.Now().UTC()); err != nil {
		t.Fatalf("标记主对话已开始失败: %v", err)
	}
	topicContext, err := roomService.CreateConversation(
		ctx,
		mainContext.Room.ID,
		protocol.CreateConversationRequest{Title: "保留会话"},
	)
	if err != nil {
		t.Fatalf("创建回退会话失败: %v", err)
	}

	fallbackContext, err := roomService.DeleteConversation(
		ctx,
		mainContext.Room.ID,
		mainContext.Conversation.ID,
	)
	if err != nil {
		t.Fatalf("删除主对话失败: %v", err)
	}
	if fallbackContext.Conversation.ID != topicContext.Conversation.ID {
		t.Fatalf("删除主对话后回退错误: %+v", fallbackContext.Conversation)
	}
	if fallbackContext.Conversation.ConversationType != protocol.ConversationTypeMain {
		t.Fatalf("回退会话未提升为主对话: %+v", fallbackContext.Conversation)
	}

	contexts, err := roomService.GetRoomContexts(ctx, mainContext.Room.ID)
	if err != nil {
		t.Fatalf("读取删除后的 room 上下文失败: %v", err)
	}
	if len(contexts) != 1 || contexts[0].Conversation.ID != topicContext.Conversation.ID {
		t.Fatalf("删除后应只保留回退会话: %+v", contexts)
	}
	if contexts[0].Conversation.ConversationType != protocol.ConversationTypeMain {
		t.Fatalf("持久化主对话类型错误: %+v", contexts[0].Conversation)
	}

	_, err = roomService.DeleteConversation(
		ctx,
		mainContext.Room.ID,
		topicContext.Conversation.ID,
	)
	if err == nil || !strings.Contains(err.Error(), "至少保留一个对话") {
		t.Fatalf("最后一个会话必须受到保护，实际: %v", err)
	}
}

func TestRoomServiceLeavesNewConversationTitleForSemanticGeneration(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := serverapp.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := serverapp.NewRoomServiceWithDB(cfg, db, agentService)

	ctx := context.Background()
	agentA := createTestAgent(t, agentService, ctx, "语义标题助手A")
	agentB := createTestAgent(t, agentService, ctx, "语义标题助手B")
	mainContext, err := roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs: []string{agentA.AgentID, agentB.AgentID},
		Name:     "语义标题测试",
	})
	if err != nil {
		t.Fatalf("创建 room 失败: %v", err)
	}
	if err = roomService.MarkConversationStarted(
		ctx,
		mainContext.Conversation.ID,
		time.Now().UTC(),
	); err != nil {
		t.Fatalf("标记初始会话已开始失败: %v", err)
	}
	topicContext, err := roomService.CreateConversation(
		ctx,
		mainContext.Room.ID,
		protocol.CreateConversationRequest{},
	)
	if err != nil {
		t.Fatalf("创建新会话失败: %v", err)
	}
	if topicContext.Conversation.Title != "" {
		t.Fatalf(
			"未输入标题的新会话应等待首条消息生成语义标题: %q",
			topicContext.Conversation.Title,
		)
	}
}
