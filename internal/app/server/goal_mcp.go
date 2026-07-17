// INPUT: 当前 runtime Agent/session/round 与 Goal 服务。
// OUTPUT: 绑定当前 DM runner 或 Room slot 共用 Goal objective revision 状态的 MCP server。
// POS: nexus_goal MCP 的应用装配入口。
package server

import (
	"context"
	"strings"
	"sync/atomic"

	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"

	"github.com/nexus-research-lab/nexus/internal/config"
	goalmcp "github.com/nexus-research-lab/nexus/internal/mcp/goal"
	goalmcpcontract "github.com/nexus-research-lab/nexus/internal/mcp/goal/contract"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type goalObjectiveRevisionStateProvider interface {
	GoalObjectiveRevisionState(sessionKey string, roundID string, agentID string, initial int64) *atomic.Int64
}

func newGoalMCPBuilder(
	cfg config.Config,
	svc goalmcpcontract.Service,
	revisions goalObjectiveRevisionStateProvider,
) func(string, string, string, string, string, string, *atomic.Int64) map[string]sdkmcp.ServerConfig {
	return func(
		agentID string,
		sessionKey string,
		roundID string,
		sourceContextType string,
		sourceContextID string,
		sourceContextLabel string,
		goalObjectiveRevision *atomic.Int64,
	) map[string]sdkmcp.ServerConfig {
		goalSessionKey := resolveGoalMCPSessionKey(sessionKey, sourceContextType)
		if !cfg.GoalEnabled || svc == nil || goalSessionKey == "" {
			return nil
		}
		sctx := goalmcpcontract.ServerContext{
			CurrentSessionKey: goalSessionKey,
			CurrentRoundID:    strings.TrimSpace(roundID),
			CurrentAgentID:    strings.TrimSpace(agentID),
		}
		if goalObjectiveRevision != nil {
			sctx.GoalObjectiveRevision = goalObjectiveRevision
		} else {
			current, err := svc.CurrentOptional(context.Background(), goalSessionKey)
			if err != nil {
				return nil
			}
			revision := int64(1)
			if current != nil {
				revision = current.ObjectiveRevision()
			}
			if revisions != nil && protocol.IsRoomSharedSessionKey(goalSessionKey) {
				sctx.GoalObjectiveRevision = revisions.GoalObjectiveRevisionState(goalSessionKey, strings.TrimSpace(roundID), strings.TrimSpace(agentID), revision)
			}
			if sctx.GoalObjectiveRevision == nil {
				sctx.GoalObjectiveRevision = goalmcpcontract.NewGoalObjectiveRevision(revision)
			}
		}
		return map[string]sdkmcp.ServerConfig{
			goalmcpcontract.ServerName: sdkmcp.SDKServerConfig{
				Name:     goalmcpcontract.ServerName,
				Instance: goalmcp.NewServer(svc, sctx),
			},
		}
	}
}

func resolveGoalMCPSessionKey(sessionKey string, sourceContextType string) string {
	normalized := strings.TrimSpace(sessionKey)
	if normalized == "" || strings.TrimSpace(sourceContextType) != "room" {
		return normalized
	}
	parsed := protocol.ParseSessionKey(normalized)
	if parsed.Kind == protocol.SessionKeyKindRoom {
		return normalized
	}
	if parsed.Kind == protocol.SessionKeyKindAgent &&
		parsed.ChatType == "group" &&
		strings.TrimSpace(parsed.Ref) != "" {
		return protocol.BuildRoomSharedSessionKey(parsed.Ref)
	}
	return normalized
}
