package server

import (
	"context"
	"strings"

	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
	imagegenmcp "github.com/nexus-research-lab/nexus/internal/mcp/imagegen"
	imagegenmcpcontract "github.com/nexus-research-lab/nexus/internal/mcp/imagegen/contract"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// newImagegenMCPBuilder 返回 DM/Room 实时链路所需的图片生成 MCPServerBuilder。
func newImagegenMCPBuilder(
	svc imagegenmcpcontract.Service,
) func(context.Context, *protocol.Agent, string, string, string, string) map[string]sdkmcp.ServerConfig {
	return func(
		_ context.Context,
		agentValue *protocol.Agent,
		sessionKey string,
		sourceContextType string,
		sourceContextID string,
		sourceContextLabel string,
	) map[string]sdkmcp.ServerConfig {
		if svc == nil || agentValue == nil ||
			strings.TrimSpace(agentValue.AgentID) == "" ||
			strings.TrimSpace(agentValue.WorkspacePath) == "" {
			return nil
		}
		sctx := imagegenmcpcontract.ServerContext{
			OwnerUserID:   strings.TrimSpace(agentValue.OwnerUserID),
			WorkspacePath: strings.TrimSpace(agentValue.WorkspacePath),
		}
		return map[string]sdkmcp.ServerConfig{
			imagegenmcpcontract.ServerName: sdkmcp.SDKServerConfig{
				Name:     imagegenmcpcontract.ServerName,
				Instance: imagegenmcp.NewServer(svc, sctx),
			},
		}
	}
}
