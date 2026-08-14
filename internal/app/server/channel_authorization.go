// INPUT: Channel authorization 服务、fresh Agent/principal 与真实 runtime round lease。
// OUTPUT: 按 owner 主智能体 WebSocket 私有 DM Session 稳定注入的专用 MCP server。
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
		contextKind, _, surfaceOK := runtimeMCPSurfaceContext(agentID, sessionKey)
		if agentID == "" || sessionKey == "" || roundID == "" ||
			!surfaceOK || contextKind != "agent" {
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
		sctx := channelauthorizationcontract.ServerContext{
			OwnerUserID:       strings.TrimSpace(record.OwnerUserID),
			CurrentAgentID:    agentID,
			CurrentSessionKey: sessionKey,
			CurrentRoundID:    roundID,
			LeaseSessionKey:   strings.TrimSpace(lease.SessionKey),
			LeaseRoundID:      strings.TrimSpace(lease.RoundID),
			ContextKind:       contextKind,
			ContextID:         agentID,
			IsMainAgent:       true,
		}
		server := func() map[string]sdkmcp.ServerConfig {
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
		if sourceContextType != contextKind || sourceContextID != agentID {
			return server()
		}
		role, authMethod, localSingleUser, principalOK := trustedConfigurationPrincipal(
			ctx,
			record.OwnerUserID,
		)
		if !principalOK {
			return server()
		}
		principal := authctx.PrincipalFromContext(ctx)
		if principal == nil ||
			strings.TrimSpace(principal.UserID) != record.OwnerUserID {
			return server()
		}
		authSessionID := ""
		if principal.SessionID != nil {
			authSessionID = strings.TrimSpace(*principal.SessionID)
		}
		switch authMethod {
		case authctx.AuthMethodPassword:
			if authSessionID == "" {
				return server()
			}
		case authctx.AuthMethodLocal:
			evidence, hasEvidence := authctx.InteractiveHumanEvidenceFromContext(ctx)
			if !localSingleUser ||
				!hasEvidence ||
				evidence.Source != "desktop_session_token" {
				return server()
			}
		default:
			return server()
		}
		if _, routeOK := trustedConfigurationRuntimeRoute(
			record.AgentID,
			sourceContextType,
			sessionKey,
			roundID,
			lease.SessionKey,
			lease.RoundID,
		); !routeOK {
			return server()
		}
		sctx.PrincipalRole = role
		sctx.AuthMethod = authMethod
		sctx.AuthSessionID = authSessionID
		sctx.LocalSingleUser = localSingleUser
		return server()
	}
}
