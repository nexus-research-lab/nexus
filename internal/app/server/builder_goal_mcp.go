package server

import (
	"strings"

	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"

	"github.com/nexus-research-lab/nexus/internal/config"
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
		if !cfg.GoalEnabled || svc == nil || strings.TrimSpace(sessionKey) == "" {
			return nil
		}
		sctx := goalmcpcontract.ServerContext{CurrentSessionKey: sessionKey}
		return map[string]sdkmcp.SDKMCPServer{
			goalmcpcontract.ServerName: goalmcp.NewServer(svc, sctx),
		}
	}
}
