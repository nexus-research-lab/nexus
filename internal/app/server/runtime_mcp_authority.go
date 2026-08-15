// INPUT: Agent 身份、业务 source context 与真实 runtime lease。
// OUTPUT: Channel、Connector、Automation 与 Communication MCP 共用的可信上下文判定。
// POS: runtime MCP 应用装配的共享身份边界。
package server

import (
	"context"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	configurationsvc "github.com/nexus-research-lab/nexus/internal/service/configuration"
)

type runtimeAgentResolver interface {
	GetAgent(context.Context, string) (*protocol.Agent, error)
}

// runtimeMCPSurfaceContext 只根据不会在轮次间漂移的 Session 拓扑选择工具表。
func runtimeMCPSurfaceContext(agentID string, sessionKey string) (string, string, bool) {
	agentID = strings.TrimSpace(agentID)
	sessionKey = strings.TrimSpace(sessionKey)
	parsed := protocol.ParseSessionKey(sessionKey)
	if !parsed.IsStructured {
		return "", "", false
	}
	switch parsed.Kind {
	case protocol.SessionKeyKindAgent:
		if parsed.Channel != protocol.SessionChannelWebSocketSegment ||
			parsed.ChatType != protocol.RoomTypeDM ||
			strings.TrimSpace(parsed.AgentID) != agentID {
			return "", "", false
		}
		return configurationsvc.ContextKindAgent, "", true
	case protocol.SessionKeyKindRoom:
		conversationID := strings.TrimSpace(parsed.ConversationID)
		if !parsed.IsShared || conversationID == "" ||
			sessionKey != protocol.BuildRoomSharedSessionKey(conversationID) {
			return "", "", false
		}
		return configurationsvc.ContextKindRoom, conversationID, true
	default:
		return "", "", false
	}
}

func trustedRuntimeRoute(
	agentID string,
	contextKind string,
	sessionKey string,
	roundID string,
	leaseSessionKey string,
	leaseRoundID string,
) (string, bool) {
	agentID = strings.TrimSpace(agentID)
	sessionKey = strings.TrimSpace(sessionKey)
	roundID = strings.TrimSpace(roundID)
	leaseSessionKey = strings.TrimSpace(leaseSessionKey)
	leaseRoundID = strings.TrimSpace(leaseRoundID)
	switch strings.ToLower(strings.TrimSpace(contextKind)) {
	case configurationsvc.ContextKindAgent:
		parsed := protocol.ParseSessionKey(sessionKey)
		if sessionKey != leaseSessionKey ||
			roundID != leaseRoundID ||
			!parsed.IsStructured ||
			parsed.Kind != protocol.SessionKeyKindAgent ||
			parsed.Channel != protocol.SessionChannelWebSocketSegment ||
			parsed.ChatType != protocol.RoomTypeDM ||
			strings.TrimSpace(parsed.AgentID) != agentID {
			return "", false
		}
		return "", true
	case configurationsvc.ContextKindRoom:
		parsedBusiness := protocol.ParseSessionKey(sessionKey)
		conversationID := strings.TrimSpace(parsedBusiness.ConversationID)
		parsedLease := protocol.ParseSessionKey(leaseSessionKey)
		if !parsedBusiness.IsStructured ||
			parsedBusiness.Kind != protocol.SessionKeyKindRoom ||
			!parsedBusiness.IsShared ||
			sessionKey != protocol.BuildRoomSharedSessionKey(conversationID) ||
			conversationID == "" ||
			!parsedLease.IsStructured ||
			parsedLease.Kind != protocol.SessionKeyKindAgent ||
			parsedLease.Channel != protocol.SessionChannelWebSocketSegment ||
			(parsedLease.ChatType != protocol.RoomTypeDM &&
				parsedLease.ChatType != "group") ||
			strings.TrimSpace(parsedLease.AgentID) != agentID ||
			strings.TrimSpace(parsedLease.Ref) != conversationID {
			return "", false
		}
		return conversationID, true
	default:
		return "", false
	}
}

func trustedRuntimePrincipal(
	ctx context.Context,
	ownerUserID string,
) (role string, authMethod string, localSingleUser bool, ok bool) {
	ownerUserID = strings.TrimSpace(ownerUserID)
	if ownerUserID == "" {
		return "", "", false, false
	}
	if principal := authctx.PrincipalFromContext(ctx); principal != nil {
		if strings.TrimSpace(principal.UserID) != ownerUserID {
			return "", "", false, false
		}
		role = strings.TrimSpace(principal.Role)
		switch role {
		case authctx.RoleOwner, authctx.RoleAdmin, authctx.RoleMember:
		default:
			role = authctx.RoleMember
		}
		authMethod = strings.TrimSpace(principal.AuthMethod)
		if authMethod == "" {
			authMethod = "mcp_runtime"
		}
		localSingleUser = authctx.IsLocalSingleUserControlPlane(ctx, ownerUserID)
		return role, authMethod, localSingleUser, true
	}
	if !authctx.IsLocalSingleUserControlPlane(ctx, ownerUserID) {
		return "", "", false, false
	}
	return authctx.RoleOwner, authctx.AuthMethodLocal, true, true
}
