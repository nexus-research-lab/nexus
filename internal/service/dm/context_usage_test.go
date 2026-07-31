package dm

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
)

type contextUsageDMClient struct {
	*fakeDMClient
	usage agentclient.ContextUsageResponse
}

func (c *contextUsageDMClient) ContextUsage(
	context.Context,
) (agentclient.ContextUsageResponse, error) {
	return c.usage, nil
}

func TestBroadcastContextUsagePersistsSessionMeta(t *testing.T) {
	storeRoot := t.TempDir()
	workspacePath := filepath.Join(storeRoot, "workspace", "agent-a")
	manager := runtimectx.NewManager()
	service := NewService(
		config.Config{WorkspacePath: storeRoot},
		nil,
		manager,
		permissionctx.NewContext(),
	)
	sessionKey := protocol.BuildAgentSessionKey(
		"agent-a",
		protocol.SessionChannelWebSocketSegment,
		protocol.RoomTypeDM,
		"conversation-a",
		"",
	)
	now := time.Now().UTC()
	runner := &roundRunner{
		service:       service,
		workspacePath: workspacePath,
		sessionKey:    sessionKey,
		roundID:       "round-a",
		agentRoundID:  "agent-round-a",
		agent:         &protocol.Agent{AgentID: "agent-a"},
		session: protocol.Session{
			SessionKey:   sessionKey,
			AgentID:      "agent-a",
			ChannelType:  protocol.SessionChannelWebSocket,
			ChatType:     protocol.RoomTypeDM,
			Status:       "closed",
			CreatedAt:    now,
			LastActivity: now,
			Title:        "Context Usage",
			Options:      map[string]any{},
		},
		client: &contextUsageDMClient{
			fakeDMClient: newFakeDMClient(),
			usage: agentclient.ContextUsageResponse{
				TotalTokens:  37_500,
				RawMaxTokens: 131_100,
				Model:        "glm-4.5-air",
			},
		},
	}

	runner.broadcastContextUsage()
	runner.refreshSessionMetaAfterRoundFinished()

	persisted, _, err := service.files.FindSession(
		[]string{workspacePath},
		sessionKey,
	)
	if err != nil {
		t.Fatalf("读取 Session meta 失败: %v", err)
	}
	if persisted == nil || persisted.ContextUsage == nil ||
		persisted.ContextUsage.TotalTokens != 37_500 ||
		persisted.ContextUsage.MaxTokens != 131_100 ||
		persisted.ContextUsage.Percentage != 28.6 ||
		persisted.ContextUsage.Model != "glm-4.5-air" {
		t.Fatalf("persisted context usage = %+v", persisted)
	}
}
