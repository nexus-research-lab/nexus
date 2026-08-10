// INPUT: configuration 服务、Agent 身份、业务 source context 与真实 runtime lease。
// OUTPUT: 只在可信用户 DM 或当前 Room 活跃 slot 中注入、由服务端动态鉴权的 nexus_config MCP builder。
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
		sourceContextType = strings.ToLower(strings.TrimSpace(sourceContextType))
		sourceContextID = strings.TrimSpace(sourceContextID)
		if sourceContextType != "agent" && sourceContextType != "room" {
			// Goal continuation、Automation、外部 IM 和未知来源默认没有持久配置 capability。
			return nil
		}
		sessionKey = strings.TrimSpace(sessionKey)
		roundID = strings.TrimSpace(roundID)
		lease, hasLease := runtimectx.MCPRoundLeaseFromContext(ctx)
		if sessionKey == "" || roundID == "" || !hasLease {
			// 业务 session/root round 负责计划和审计归属；runtime lease 负责
			// 证明当前 DM round 或 Room Agent slot 仍在执行，缺一不可。
			return nil
		}
		// agentValue 只是 runtime 快照。每次构造 server 都重新读取当前 Agent/
		// owner/main 身份，避免旧 client 快照扩大配置作用域。
		record, err := agents.GetAgent(ctx, agentValue.AgentID)
		if err != nil || record == nil || strings.TrimSpace(record.OwnerUserID) == "" {
			return nil
		}
		role, authMethod, localSingleUser, ok := trustedConfigurationPrincipal(
			ctx,
			record.OwnerUserID,
		)
		if !ok {
			return nil
		}
		authSessionID := ""
		if principal := authctx.PrincipalFromContext(ctx); principal != nil &&
			principal.SessionID != nil {
			authSessionID = strings.TrimSpace(*principal.SessionID)
		}
		if queued, hasQueuedBinding := authctx.QueuedHumanPrincipalBindingFromContext(ctx); hasQueuedBinding {
			if strings.TrimSpace(queued.UserID) != strings.TrimSpace(record.OwnerUserID) {
				return nil
			}
			// A queue worker's synthetic RoleOwner is only an owner-scoping
			// transport principal. Persist no role in the admission and fail
			// closed until configuration.resolveActor reloads the active role.
			role = authctx.RoleMember
			authMethod = queued.AuthMethod
			authSessionID = queued.SessionID
			localSingleUser = false
		}
		if sourceContextType == "agent" && sourceContextID != record.AgentID {
			return nil
		}
		if sourceContextType == "room" && sourceContextID == "" {
			return nil
		}
		conversationID, routeOK := trustedConfigurationRuntimeRoute(
			record.AgentID,
			sourceContextType,
			sessionKey,
			roundID,
			lease.SessionKey,
			lease.RoundID,
		)
		if !routeOK {
			return nil
		}
		sctx := configurationcontract.ServerContext{
			OwnerUserID:       record.OwnerUserID,
			CurrentAgentID:    record.AgentID,
			CurrentSessionKey: sessionKey,
			CurrentRoundID:    roundID,
			LeaseSessionKey:   lease.SessionKey,
			LeaseRoundID:      lease.RoundID,
			ContextKind:       sourceContextType,
			ContextID:         sourceContextID,
			ConversationID:    conversationID,
			SourceContext:     strings.Trim(strings.TrimSpace(sourceContextType)+":"+strings.TrimSpace(sourceContextID), ":"),
			IsMainAgent:       record.IsMain,
			PrincipalRole:     role,
			AuthMethod:        authMethod,
			AuthSessionID:     authSessionID,
			LocalSingleUser:   localSingleUser,
		}
		if sourceContextType == "room" {
			sctx.RoomID = sourceContextID
		}
		return map[string]sdkmcp.ServerConfig{
			configurationcontract.ServerName: sdkmcp.SDKServerConfig{
				Name: configurationcontract.ServerName, Instance: configurationmcp.NewServer(svc, sctx),
			},
		}
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
