// INPUT: Channel authorization 服务、fresh Agent/principal 与真实 runtime round lease。
// OUTPUT: 按 owner 主智能体 WebSocket 私有 DM Session 稳定可见的 Channel 授权工具。
// POS: nexus MCP 的 Channel 真人授权工具装配边界。
package runtime

import (
	"context"
	nexusmcp "github.com/nexus-research-lab/nexus/internal/mcp"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	channelauthorizationmcp "github.com/nexus-research-lab/nexus/internal/mcp/channelauthorization"
	channelauthorizationcontract "github.com/nexus-research-lab/nexus/internal/mcp/channelauthorization/contract"
	"github.com/nexus-research-lab/nexus/internal/mcp/sdktool"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
)

// NewChannelAuthorizationToolBuilder 构建当前 round 可用的 Channel 授权工具。
func NewChannelAuthorizationToolBuilder(
	svc channelauthorizationcontract.Service,
	agents AgentResolver,
) ToolBuilder {
	return func(ctx context.Context, round nexusmcp.RoundContext) []sdktool.Tool {
		agentValue := round.CommandContext.Agent
		sessionKey := round.SessionKey
		roundID := round.RoundID
		sourceContextType := round.SourceContextType
		sourceContextID := round.SourceContextID
		if svc == nil || agents == nil || agentValue == nil {
			return nil
		}
		agentID := strings.TrimSpace(agentValue.AgentID)
		sessionKey = strings.TrimSpace(sessionKey)
		roundID = strings.TrimSpace(roundID)
		sourceContextType = strings.ToLower(strings.TrimSpace(sourceContextType))
		sourceContextID = strings.TrimSpace(sourceContextID)
		contextKind, _, surfaceOK := mcpSurfaceContext(agentID, sessionKey)
		if agentID == "" || sessionKey == "" || roundID == "" ||
			!surfaceOK || contextKind != "agent" {
			return nil
		}
		lease, hasLease := runtimectx.RuntimeRoundLeaseFromContext(ctx)
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
		tools := func() []sdktool.Tool {
			return channelauthorizationmcp.BuildTools(svc, sctx)
		}
		if sourceContextType != contextKind || sourceContextID != agentID {
			return tools()
		}
		role, authMethod, localSingleUser, principalOK := trustedPrincipal(
			ctx,
			record.OwnerUserID,
		)
		if !principalOK {
			return tools()
		}
		principal := authctx.PrincipalFromContext(ctx)
		if principal == nil ||
			strings.TrimSpace(principal.UserID) != record.OwnerUserID {
			return tools()
		}
		authSessionID := ""
		if principal.SessionID != nil {
			authSessionID = strings.TrimSpace(*principal.SessionID)
		}
		switch authMethod {
		case authctx.AuthMethodPassword:
			if authSessionID == "" {
				return tools()
			}
		case authctx.AuthMethodLocal:
			evidence, hasEvidence := authctx.InteractiveHumanEvidenceFromContext(ctx)
			if !localSingleUser ||
				!hasEvidence ||
				evidence.Source != "desktop_session_token" {
				return tools()
			}
		default:
			return tools()
		}
		if _, routeOK := trustedRoute(
			record.AgentID,
			sourceContextType,
			sessionKey,
			roundID,
			lease.SessionKey,
			lease.RoundID,
		); !routeOK {
			return tools()
		}
		sctx.PrincipalRole = role
		sctx.AuthMethod = authMethod
		sctx.AuthSessionID = authSessionID
		sctx.LocalSingleUser = localSingleUser
		return tools()
	}
}
