package session_test

import (
	"context"
	"testing"
	"time"

	serverapp "github.com/nexus-research-lab/nexus/internal/app/server"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	sessionsvc "github.com/nexus-research-lab/nexus/internal/service/session"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

func TestSessionServiceReadsPersistedDMContextUsage(t *testing.T) {
	cfg := newSessionTestConfig(t)
	migrateSessionSQLite(t, cfg.DatabaseURL)

	agentService, db, err := serverapp.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	sessionService := serverapp.NewSessionServiceWithDB(cfg, db, agentService)
	ctx := context.Background()
	agentValue, err := agentService.CreateAgent(
		ctx,
		protocol.CreateRequest{Name: "Context Usage 助手"},
	)
	if err != nil {
		t.Fatalf("创建 agent 失败: %v", err)
	}
	sessionKey := protocol.BuildAgentSessionKey(
		agentValue.AgentID,
		protocol.SessionChannelWebSocketSegment,
		protocol.RoomTypeDM,
		"context-usage",
		"",
	)
	sessionValue, err := sessionService.CreateSession(
		ctx,
		sessionsvc.CreateRequest{SessionKey: sessionKey},
	)
	if err != nil {
		t.Fatalf("创建 session 失败: %v", err)
	}
	sessionValue.ContextUsage = &protocol.ContextUsageData{
		TotalTokens: 37_500,
		MaxTokens:   131_100,
		Percentage:  28.6,
		Model:       "glm-4.5-air",
	}
	store := workspacestore.NewSessionFileStore(cfg.WorkspacePath)
	if _, err = store.UpsertSession(agentValue.WorkspacePath, *sessionValue); err != nil {
		t.Fatalf("写入 context usage 失败: %v", err)
	}

	usages, err := sessionService.GetPersistedContextUsageSnapshots(ctx, sessionKey)
	if err != nil {
		t.Fatalf("GetPersistedContextUsageSnapshots() error = %v", err)
	}
	usage, ok := usages[agentValue.AgentID]
	if !ok ||
		usage.TotalTokens != 37_500 ||
		usage.MaxTokens != 131_100 ||
		usage.Percentage != 28.6 ||
		usage.Model != "glm-4.5-air" {
		t.Fatalf("GetPersistedContextUsageSnapshots() = %+v", usages)
	}
}

func TestSessionServiceReadsPersistedRoomContextUsageByAgent(t *testing.T) {
	cfg := newSessionTestConfig(t)
	migrateSessionSQLite(t, cfg.DatabaseURL)

	agentService, db, err := serverapp.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := serverapp.NewRoomServiceWithDB(cfg, db, agentService)
	sessionService := serverapp.NewSessionServiceWithDB(cfg, db, agentService)
	ctx := context.Background()
	agentA, err := agentService.CreateAgent(
		ctx,
		protocol.CreateRequest{Name: "Context Usage A"},
	)
	if err != nil {
		t.Fatalf("创建 Agent A 失败: %v", err)
	}
	agentB, err := agentService.CreateAgent(
		ctx,
		protocol.CreateRequest{Name: "Context Usage B"},
	)
	if err != nil {
		t.Fatalf("创建 Agent B 失败: %v", err)
	}
	roomContext, err := roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs: []string{agentA.AgentID, agentB.AgentID},
		Name:     "Context Usage Room",
	})
	if err != nil {
		t.Fatalf("创建 Room 失败: %v", err)
	}

	store := workspacestore.NewSessionFileStore(cfg.WorkspacePath)
	now := time.Now().UTC()
	agents := []struct {
		agentID       string
		workspacePath string
		totalTokens   int
	}{
		{agentID: agentA.AgentID, workspacePath: agentA.WorkspacePath, totalTokens: 37_500},
		{agentID: agentB.AgentID, workspacePath: agentB.WorkspacePath, totalTokens: 61_000},
	}
	for _, agent := range agents {
		sessionKey := protocol.BuildRoomAgentSessionKey(
			roomContext.Conversation.ID,
			agent.agentID,
			protocol.RoomTypeGroup,
		)
		if _, err = store.UpsertSession(agent.workspacePath, protocol.Session{
			SessionKey:     sessionKey,
			AgentID:        agent.agentID,
			RoomID:         stringPointer(roomContext.Room.ID),
			ConversationID: stringPointer(roomContext.Conversation.ID),
			ChannelType:    protocol.SessionChannelWebSocketSegment,
			ChatType:       protocol.RoomTypeGroup,
			Status:         "active",
			CreatedAt:      now,
			LastActivity:   now,
			Title:          roomContext.Conversation.Title,
			Options:        map[string]any{},
			ContextUsage: &protocol.ContextUsageData{
				TotalTokens: agent.totalTokens,
				MaxTokens:   131_100,
				Percentage:  float64(agent.totalTokens) / 131_100 * 100,
			},
			IsActive: true,
		}); err != nil {
			t.Fatalf("写入 Agent %s context usage 失败: %v", agent.agentID, err)
		}
	}

	usages, err := sessionService.GetPersistedContextUsageSnapshots(
		ctx,
		protocol.BuildRoomSharedSessionKey(roomContext.Conversation.ID),
	)
	if err != nil {
		t.Fatalf("GetPersistedContextUsageSnapshots() error = %v", err)
	}
	if len(usages) != 2 ||
		usages[agentA.AgentID].TotalTokens != 37_500 ||
		usages[agentB.AgentID].TotalTokens != 61_000 {
		t.Fatalf("Room context usage = %+v, want both Agent snapshots", usages)
	}
}
