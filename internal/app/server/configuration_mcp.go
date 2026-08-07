// INPUT: configuration 服务、Agent 身份与 runtime source context。
// OUTPUT: 仅主智能体可见的 nexus_config MCP builder。
// POS: configuration MCP 的应用装配入口。
package server

import (
	"context"
	"strings"
	"sync/atomic"

	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	configurationmcp "github.com/nexus-research-lab/nexus/internal/mcp/configuration"
	configurationcontract "github.com/nexus-research-lab/nexus/internal/mcp/configuration/contract"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type configurationAgentResolver interface {
	GetAgent(context.Context, string) (*protocol.Agent, error)
}

func newConfigurationMCPBuilder(
	svc configurationcontract.Service,
	agents configurationAgentResolver,
) func(context.Context, *protocol.Agent, string, string, string, string, string, *atomic.Int64, sdkpermission.Mode) map[string]sdkmcp.ServerConfig {
	return func(
		ctx context.Context,
		agentValue *protocol.Agent,
		sessionKey string,
		_ string,
		sourceContextType string,
		sourceContextID string,
		_ string,
		_ *atomic.Int64,
		_ sdkpermission.Mode,
	) map[string]sdkmcp.ServerConfig {
		if svc == nil || agents == nil || agentValue == nil || strings.TrimSpace(agentValue.AgentID) == "" {
			return nil
		}
		record, err := agents.GetAgent(ctx, agentValue.AgentID)
		if err != nil || record == nil || !record.IsMain || strings.TrimSpace(record.OwnerUserID) == "" {
			return nil
		}
		sctx := configurationcontract.ServerContext{
			OwnerUserID:       record.OwnerUserID,
			CurrentAgentID:    record.AgentID,
			CurrentSessionKey: sessionKey,
			SourceContext:     strings.Trim(strings.TrimSpace(sourceContextType)+":"+strings.TrimSpace(sourceContextID), ":"),
			IsMainAgent:       true,
		}
		return map[string]sdkmcp.ServerConfig{
			configurationcontract.ServerName: sdkmcp.SDKServerConfig{
				Name: configurationcontract.ServerName, Instance: configurationmcp.NewServer(svc, sctx),
			},
		}
	}
}
