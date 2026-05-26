package server

import (
	"strings"

	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	goalmcp "github.com/nexus-research-lab/nexus/internal/runtime/mcp/goal"
	goalmcpcontract "github.com/nexus-research-lab/nexus/internal/runtime/mcp/goal/contract"
)

func newGoalMCPBuilder(
	cfg config.Config,
	svc goalmcpcontract.Service,
) func(string, string, string, string, string) map[string]sdkmcp.SDKMCPServer {
	return func(
		agentID string,
		sessionKey string,
		sourceContextType string,
		sourceContextID string,
		sourceContextLabel string,
	) map[string]sdkmcp.SDKMCPServer {
		goalSessionKey := resolveGoalMCPSessionKey(sessionKey, sourceContextType)
		if !cfg.GoalEnabled || svc == nil || goalSessionKey == "" {
			return nil
		}
		sctx := goalmcpcontract.ServerContext{CurrentSessionKey: goalSessionKey}
		return map[string]sdkmcp.SDKMCPServer{
			goalmcpcontract.ServerName: goalmcp.NewServer(svc, sctx),
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
