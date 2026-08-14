// INPUT: 平台通讯服务、当前 Agent 记录及所有带 runtime lease 的 Agent/Room 上下文。
// OUTPUT: 仅普通活跃 Agent 可见、每次调用仍重校验身份的 nexus_comms MCP builder。
// POS: Agent 平台通讯能力的应用装配入口。
package server

import (
	"context"
	"strings"
	"sync/atomic"

	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	communicationmcp "github.com/nexus-research-lab/nexus/internal/mcp/communication"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	communicationsvc "github.com/nexus-research-lab/nexus/internal/service/communication"
)

func newCommunicationMCPBuilder(
	svc *communicationsvc.Service,
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
		if svc == nil {
			return nil
		}
		actor, ok := communicationRuntimeActor(
			ctx, agents, agentValue, sessionKey, roundID, sourceContextType, sourceContextID,
		)
		if !ok {
			return nil
		}
		return map[string]sdkmcp.ServerConfig{
			communicationmcp.ServerName: sdkmcp.SDKServerConfig{
				Name:     communicationmcp.ServerName,
				Instance: communicationmcp.NewServer(svc, actor),
			},
		}
	}
}

func communicationRuntimeActor(
	ctx context.Context,
	agents configurationAgentResolver,
	agentValue *protocol.Agent,
	sessionKey string,
	roundID string,
	sourceContextType string,
	sourceContextID string,
) (communicationsvc.Actor, bool) {
	if agents == nil || agentValue == nil {
		return communicationsvc.Actor{}, false
	}
	sessionKey = strings.TrimSpace(sessionKey)
	roundID = strings.TrimSpace(roundID)
	sourceContextType = strings.ToLower(strings.TrimSpace(sourceContextType))
	sourceContextID = strings.TrimSpace(sourceContextID)
	if sessionKey == "" || roundID == "" {
		return communicationsvc.Actor{}, false
	}
	contextKind := ""
	switch {
	case sourceContextType == communicationsvc.ContextKindAgent,
		strings.HasPrefix(sourceContextType, communicationsvc.ContextKindAgent+"_"):
		contextKind = communicationsvc.ContextKindAgent
	case sourceContextType == communicationsvc.ContextKindRoom,
		strings.HasPrefix(sourceContextType, communicationsvc.ContextKindRoom+"_"):
		contextKind = communicationsvc.ContextKindRoom
	default:
		return communicationsvc.Actor{}, false
	}
	lease, ok := runtimectx.MCPRoundLeaseFromContext(ctx)
	if !ok {
		return communicationsvc.Actor{}, false
	}
	record, err := agents.GetAgent(ctx, strings.TrimSpace(agentValue.AgentID))
	if err != nil || record == nil || record.IsMain ||
		strings.TrimSpace(record.AgentID) == "" ||
		strings.TrimSpace(record.OwnerUserID) == "" {
		return communicationsvc.Actor{}, false
	}
	if principal := authctx.PrincipalFromContext(ctx); principal != nil &&
		strings.TrimSpace(principal.UserID) != strings.TrimSpace(record.OwnerUserID) {
		return communicationsvc.Actor{}, false
	}
	actor := communicationsvc.Actor{
		OwnerUserID: strings.TrimSpace(record.OwnerUserID), AgentID: strings.TrimSpace(record.AgentID),
		SessionKey: sessionKey, RoundID: roundID,
		LeaseSessionKey: strings.TrimSpace(lease.SessionKey), LeaseRoundID: strings.TrimSpace(lease.RoundID),
		ContextKind: contextKind, ContextID: sourceContextID,
	}
	switch contextKind {
	case communicationsvc.ContextKindAgent:
		if sourceContextID != actor.AgentID {
			return communicationsvc.Actor{}, false
		}
	case communicationsvc.ContextKindRoom:
		parsed := protocol.ParseSessionKey(sessionKey)
		if sourceContextID == "" || !parsed.IsStructured ||
			parsed.Kind != protocol.SessionKeyKindRoom ||
			strings.TrimSpace(parsed.ConversationID) == "" {
			return communicationsvc.Actor{}, false
		}
		actor.RoomID = sourceContextID
		actor.ConversationID = strings.TrimSpace(parsed.ConversationID)
		if authority := runtimectx.ResponsibilityAuthorityStateFromContext(ctx); authority != nil {
			actor.GoalCollaborationBinding = func() *protocol.GoalCollaborationBinding {
				current, ok := authority.LoadGoalAuthority()
				if !ok {
					return nil
				}
				return &protocol.GoalCollaborationBinding{
					GoalID:            current.GoalID,
					ObjectiveRevision: current.ObjectiveRevision,
				}
			}
			break
		}
		if authority := runtimectx.GoalAuthorityStateFromContext(ctx); authority != nil {
			actor.GoalCollaborationBinding = func() *protocol.GoalCollaborationBinding {
				current, ok := authority.LoadBound()
				if !ok {
					return nil
				}
				return &protocol.GoalCollaborationBinding{
					GoalID:            current.GoalID,
					ObjectiveRevision: current.ObjectiveRevision,
				}
			}
		}
	}
	return actor, true
}
