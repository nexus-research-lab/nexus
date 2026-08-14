// INPUT: configuration 服务、Agent 身份、业务 source context 与真实 runtime lease。
// OUTPUT: 按 Session 拓扑稳定注入、由服务端逐轮动态鉴权的 nexus_config MCP builder。
// POS: configuration MCP 的应用装配入口。
package server

import (
	"context"
	"strings"
	"sync/atomic"

	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	configurationmcp "github.com/nexus-research-lab/nexus/internal/mcp/configuration"
	configurationcontract "github.com/nexus-research-lab/nexus/internal/mcp/configuration/contract"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	configurationsvc "github.com/nexus-research-lab/nexus/internal/service/configuration"
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
		roundID string,
		sourceContextType string,
		sourceContextID string,
		_ string,
		_ *atomic.Int64,
		_ sdkpermission.Mode,
	) map[string]sdkmcp.ServerConfig {
		if svc == nil || agents == nil || agentValue == nil || strings.TrimSpace(agentValue.AgentID) == "" {
			return nil
		}
		agentID := strings.TrimSpace(agentValue.AgentID)
		sessionKey = strings.TrimSpace(sessionKey)
		roundID = strings.TrimSpace(roundID)
		sourceContextType = strings.ToLower(strings.TrimSpace(sourceContextType))
		sourceContextID = strings.TrimSpace(sourceContextID)
		contextKind, _, surfaceOK := runtimeMCPSurfaceContext(agentID, sessionKey)
		if !surfaceOK {
			return nil
		}
		lease, hasLease := runtimectx.MCPRoundLeaseFromContext(ctx)
		if sessionKey == "" || roundID == "" || !hasLease {
			// 业务 session/root round 负责计划和审计归属；runtime lease 负责
			// 证明当前 DM round 或 Room Agent slot 仍在执行，缺一不可。
			return nil
		}
		// agentValue 只是 runtime 快照。每次构造 server 都重新读取当前 Agent/
		// owner/main 身份，避免旧 client 快照扩大配置作用域。
		record, err := agents.GetAgent(ctx, agentID)
		if err != nil || record == nil ||
			strings.TrimSpace(record.AgentID) != agentID ||
			strings.TrimSpace(record.OwnerUserID) == "" {
			return nil
		}
		sctx := configurationcontract.ServerContext{
			OwnerUserID:       strings.TrimSpace(record.OwnerUserID),
			CurrentAgentID:    agentID,
			CurrentSessionKey: sessionKey,
			CurrentRoundID:    roundID,
			LeaseSessionKey:   strings.TrimSpace(lease.SessionKey),
			LeaseRoundID:      strings.TrimSpace(lease.RoundID),
			ContextKind:       contextKind,
			SourceContext:     strings.Trim(sourceContextType+":"+sourceContextID, ":"),
			IsMainAgent:       record.IsMain,
		}
		server := func() map[string]sdkmcp.ServerConfig {
			return map[string]sdkmcp.ServerConfig{
				configurationcontract.ServerName: sdkmcp.SDKServerConfig{
					Name: configurationcontract.ServerName, Instance: configurationmcp.NewServer(svc, sctx),
				},
			}
		}

		// 非用户触发轮仍保留同一工具表，但不签发 ContextID 和
		// human principal；真实 service 调用会在作用域边界 fail closed。
		if sourceContextType != contextKind {
			return server()
		}
		role, authMethod, localSingleUser, ok := trustedConfigurationPrincipal(
			ctx,
			record.OwnerUserID,
		)
		if !ok {
			return server()
		}
		authSessionID := ""
		if principal := authctx.PrincipalFromContext(ctx); principal != nil &&
			principal.SessionID != nil {
			authSessionID = strings.TrimSpace(*principal.SessionID)
		}
		if queued, hasQueuedBinding := authctx.QueuedHumanPrincipalBindingFromContext(ctx); hasQueuedBinding {
			if strings.TrimSpace(queued.UserID) != strings.TrimSpace(record.OwnerUserID) {
				return server()
			}
			// A queue worker's synthetic RoleOwner is only an owner-scoping
			// transport principal. Persist no role in the admission and fail
			// closed until configuration.resolveActor reloads the active role.
			role = authctx.RoleMember
			authMethod = queued.AuthMethod
			authSessionID = queued.SessionID
			localSingleUser = false
		}
		if contextKind == configurationsvc.ContextKindAgent && sourceContextID != agentID {
			return server()
		}
		if contextKind == configurationsvc.ContextKindRoom && sourceContextID == "" {
			return server()
		}
		conversationID, routeOK := trustedConfigurationRuntimeRoute(
			agentID,
			contextKind,
			sessionKey,
			roundID,
			lease.SessionKey,
			lease.RoundID,
		)
		if !routeOK {
			return server()
		}
		sctx.ContextID = sourceContextID
		sctx.ConversationID = conversationID
		sctx.PrincipalRole = role
		sctx.AuthMethod = authMethod
		sctx.AuthSessionID = authSessionID
		sctx.LocalSingleUser = localSingleUser
		if contextKind == configurationsvc.ContextKindRoom {
			sctx.RoomID = sourceContextID
		}
		return server()
	}
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

func trustedConfigurationRuntimeRoute(
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

func trustedConfigurationPrincipal(
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
