// INPUT: nexusmanager 服务、可信 Agent 记录与 direct-user DM/Room runtime 上下文。
// OUTPUT: 仅在可信用户轮次注入、且每次调用重新鉴权的 nexus_manager MCP builder。
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
		if sessionKey == "" || roundID == "" ||
			(sourceContextType != "agent" && sourceContextType != "room") {
			return nil
		}
		lease, ok := runtimectx.MCPRoundLeaseFromContext(ctx)
		if !ok {
			return nil
		}
		record, err := agents.GetAgent(ctx, strings.TrimSpace(agentValue.AgentID))
		if err != nil || record == nil ||
			strings.TrimSpace(record.AgentID) == "" ||
			strings.TrimSpace(record.OwnerUserID) == "" {
			return nil
		}
		if _, _, _, ok := trustedConfigurationPrincipal(ctx, record.OwnerUserID); !ok {
			return nil
		}
		sctx := managercontract.ServerContext{
			OwnerUserID: record.OwnerUserID, CurrentAgentID: record.AgentID,
			CurrentSessionKey: sessionKey, CurrentRoundID: roundID,
			LeaseSessionKey: lease.SessionKey, LeaseRoundID: lease.RoundID,
			ContextKind: sourceContextType, ContextID: sourceContextID,
			IsMainAgent: record.IsMain,
		}
		switch sourceContextType {
		case "agent":
			if sourceContextID != record.AgentID {
				return nil
			}
		case "room":
			parsed := protocol.ParseSessionKey(sessionKey)
			if sourceContextID == "" ||
				!parsed.IsStructured ||
				parsed.Kind != protocol.SessionKeyKindRoom ||
				strings.TrimSpace(parsed.ConversationID) == "" {
				return nil
			}
			sctx.RoomID = sourceContextID
			sctx.ConversationID = parsed.ConversationID
		}
		return map[string]sdkmcp.ServerConfig{
			managercontract.ServerName: sdkmcp.SDKServerConfig{
				Name:     managercontract.ServerName,
				Instance: managermcp.NewServer(svc, sctx),
			},
		}
	}
}
