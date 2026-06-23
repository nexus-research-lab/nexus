package server

import (
	"strings"

	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"

	"github.com/nexus-research-lab/nexus/internal/config"
	goalmcp "github.com/nexus-research-lab/nexus/internal/mcp/goal"
	goalmcpcontract "github.com/nexus-research-lab/nexus/internal/mcp/goal/contract"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func newGoalMCPBuilder(
	cfg config.Config,
	svc goalmcpcontract.Service,
) func(string, string, string, string, string, string) map[string]sdkmcp.ServerConfig {
	return func(
		agentID string,
		sessionKey string,
		roundID string,
		sourceContextType string,
		sourceContextID string,
		sourceContextLabel string,
	) map[string]sdkmcp.ServerConfig {
		goalSessionKey := resolveGoalMCPSessionKey(sessionKey, sourceContextType)
		if !cfg.GoalEnabled || svc == nil || goalSessionKey == "" {
			return nil
		}
		sctx := goalmcpcontract.ServerContext{
			CurrentSessionKey: goalSessionKey,
			CurrentRoundID:    strings.TrimSpace(roundID),
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
