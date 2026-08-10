// INPUT: Channel authorization 服务、fresh Agent/principal 与真实 runtime round lease。
// OUTPUT: 仅注入 owner 主智能体 WebSocket 私有 DM 的专用 MCP server。
// POS: Channel 真人授权的应用层 capability 装配边界。
package server

import (
	"context"
	"strings"
	"sync/atomic"

	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	channelauthorizationmcp "github.com/nexus-research-lab/nexus/internal/mcp/channelauthorization"
	channelauthorizationcontract "github.com/nexus-research-lab/nexus/internal/mcp/channelauthorization/contract"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
)

func newChannelAuthorizationMCPBuilder(
	svc channelauthorizationcontract.Service,
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
		agentID := strings.TrimSpace(agentValue.AgentID)
		sessionKey = strings.TrimSpace(sessionKey)
		roundID = strings.TrimSpace(roundID)
		sourceContextType = strings.ToLower(strings.TrimSpace(sourceContextType))
		sourceContextID = strings.TrimSpace(sourceContextID)
		if agentID == "" ||
			sessionKey == "" ||
			roundID == "" ||
			sourceContextType != "agent" ||
			sourceContextID != agentID {
			return nil
		}
		lease, hasLease := runtimectx.MCPRoundLeaseFromContext(ctx)
		if !hasLease {
			return nil
		}
		record, err := agents.GetAgent(ctx, agentID)
		if err != nil ||
			record == nil ||
			strings.TrimSpace(record.OwnerUserID) == "" ||
			strings.TrimSpace(record.AgentID) != agentID ||
			!record.IsMain {
			return nil
		}
		role, authMethod, localSingleUser, principalOK := trustedConfigurationPrincipal(
			ctx,
			record.OwnerUserID,
		)
		if !principalOK {
			return nil
		}
		principal := authctx.PrincipalFromContext(ctx)
		if principal == nil ||
			strings.TrimSpace(principal.UserID) != record.OwnerUserID {
			return nil
		}
		authSessionID := ""
		if principal.SessionID != nil {
			authSessionID = strings.TrimSpace(*principal.SessionID)
		}
		switch authMethod {
		case authctx.AuthMethodPassword:
			if authSessionID == "" {
				return nil
			}
		case authctx.AuthMethodLocal:
			evidence, hasEvidence := authctx.InteractiveHumanEvidenceFromContext(ctx)
			if !localSingleUser ||
				!hasEvidence ||
				evidence.Source != "desktop_session_token" {
				return nil
			}
		default:
			return nil
		}
		if _, routeOK := trustedConfigurationRuntimeRoute(
			record.AgentID,
			sourceContextType,
			sessionKey,
			roundID,
			lease.SessionKey,
			lease.RoundID,
		); !routeOK {
			return nil
		}
		sctx := channelauthorizationcontract.ServerContext{
			OwnerUserID:       record.OwnerUserID,
			CurrentAgentID:    record.AgentID,
			CurrentSessionKey: sessionKey,
			CurrentRoundID:    roundID,
			LeaseSessionKey:   strings.TrimSpace(lease.SessionKey),
			LeaseRoundID:      strings.TrimSpace(lease.RoundID),
			ContextKind:       sourceContextType,
			ContextID:         sourceContextID,
			IsMainAgent:       true,
			PrincipalRole:     role,
			AuthMethod:        authMethod,
			AuthSessionID:     authSessionID,
			LocalSingleUser:   localSingleUser,
		}
		return map[string]sdkmcp.ServerConfig{
			channelauthorizationcontract.ServerName: sdkmcp.SDKServerConfig{
				Name: channelauthorizationcontract.ServerName,
				Instance: channelauthorizationmcp.NewServer(
					svc,
					sctx,
				),
			},
		}
	}
}
