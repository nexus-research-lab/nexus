// INPUT: nexusmanager 服务、可信 Agent 记录与 direct-user DM/Room runtime 上下文。
// OUTPUT: 按 Session 拓扑稳定注入、且每次调用重新鉴权的 nexus_manager MCP builder。
// POS: 受控 Nexus 资源管理 MCP 的应用装配入口。
package server

import (
	"context"
	"strings"
	"sync/atomic"

	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	managermcp "github.com/nexus-research-lab/nexus/internal/mcp/manager"
	managercontract "github.com/nexus-research-lab/nexus/internal/mcp/manager/contract"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	configurationsvc "github.com/nexus-research-lab/nexus/internal/service/configuration"
)

func newNexusManagerMCPBuilder(
	svc managercontract.Service,
	agents configurationAgentResolver,
) func(context.Context, *protocol.Agent, string, string, string, string, string, *atomic.Int64, sdkpermission.Mode) map[string]sdkmcp.ServerConfig {
	return func(
		ctx context.Context,
		agentValue *protocol.Agent,
		sessionKey string,
		roundID string,
		sourceContextType string,
		sourceContextID string,
		_ string,
		_ *atomic.Int64,
		_ sdkpermission.Mode,
	) map[string]sdkmcp.ServerConfig {
		if svc == nil || agents == nil || agentValue == nil {
			return nil
		}
		sessionKey = strings.TrimSpace(sessionKey)
		roundID = strings.TrimSpace(roundID)
		sourceContextType = strings.ToLower(strings.TrimSpace(sourceContextType))
		sourceContextID = strings.TrimSpace(sourceContextID)
		agentID := strings.TrimSpace(agentValue.AgentID)
		contextKind, conversationID, surfaceOK := runtimeMCPSurfaceContext(agentID, sessionKey)
		if sessionKey == "" || roundID == "" || !surfaceOK {
			return nil
		}
		lease, ok := runtimectx.MCPRoundLeaseFromContext(ctx)
		if !ok {
			return nil
		}
		record, err := agents.GetAgent(ctx, agentID)
		if err != nil || record == nil ||
			strings.TrimSpace(record.AgentID) != agentID ||
			strings.TrimSpace(record.OwnerUserID) == "" {
			return nil
		}
		sctx := managercontract.ServerContext{
			OwnerUserID: record.OwnerUserID, CurrentAgentID: record.AgentID,
			CurrentSessionKey: sessionKey, CurrentRoundID: roundID,
			LeaseSessionKey: lease.SessionKey, LeaseRoundID: lease.RoundID,
			ContextKind: contextKind,
			IsMainAgent: record.IsMain,
		}
		server := func() map[string]sdkmcp.ServerConfig {
			return map[string]sdkmcp.ServerConfig{
				managercontract.ServerName: sdkmcp.SDKServerConfig{
					Name:     managercontract.ServerName,
					Instance: managermcp.NewServer(svc, sctx),
				},
			}
		}
		if sourceContextType != contextKind {
			return server()
		}
		if _, _, _, ok := trustedConfigurationPrincipal(ctx, record.OwnerUserID); !ok {
			return server()
		}
		if contextKind == configurationsvc.ContextKindAgent && sourceContextID != agentID {
			return server()
		}
		if contextKind == configurationsvc.ContextKindRoom && sourceContextID == "" {
			return server()
		}
		if _, ok := trustedConfigurationRuntimeRoute(
			agentID,
			contextKind,
			sessionKey,
			roundID,
			lease.SessionKey,
			lease.RoundID,
		); !ok {
			return server()
		}
		sctx.ContextID = sourceContextID
		if contextKind == configurationsvc.ContextKindRoom {
			sctx.RoomID = sourceContextID
			sctx.ConversationID = conversationID
		}
		return server()
	}
}
