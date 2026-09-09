// INPUT: 平台通讯、Room realtime、当前 Agent 记录及带 runtime lease 的 Agent/Room 上下文。
// OUTPUT: 仅普通活跃 Agent 可见、按 DM/Room 上下文生成的统一通讯工具。
// POS: nexus MCP 唯一通讯工具装配入口。
package runtime

import (
	"context"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	nexusmcp "github.com/nexus-research-lab/nexus/internal/mcp"
	communicationmcp "github.com/nexus-research-lab/nexus/internal/mcp/communication"
	"github.com/nexus-research-lab/nexus/internal/mcp/sdktool"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	communicationsvc "github.com/nexus-research-lab/nexus/internal/service/communication"
)

// NewCommunicationToolBuilder 构建 DM、跨会话与当前 Room 的统一通讯工具。
func NewCommunicationToolBuilder(
	svc *communicationsvc.Service,
	room communicationmcp.RoomService,
	agents AgentResolver,
	getRoom func(context.Context, string) (*protocol.RoomAggregate, error),
) ToolBuilder {
	return func(ctx context.Context, round nexusmcp.RoundContext) []sdktool.Tool {
		if svc == nil {
			return nil
		}
		actor, ok := communicationRuntimeActor(
			ctx,
			agents,
			round.CommandContext.Agent,
			round.SessionKey,
			round.RoundID,
			round.SourceContextType,
			round.SourceContextID,
		)
		if !ok {
			return nil
		}
		sctx := communicationmcp.RuntimeContext{
			Actor:                actor,
			CurrentAgentRoundID:  actor.LeaseRoundID,
			CurrentRoomAvailable: actor.ContextKind == communicationsvc.ContextKindRoom,
		}
		if sctx.CurrentRoomAvailable && getRoom != nil {
			record, err := getRoom(ctx, actor.RoomID)
			if err == nil && record != nil && record.Room.IsContactChannel {
				sctx.CurrentRoomAvailable = false
			}
		}
		return communicationmcp.BuildTools(svc, room, sctx)
	}
}

func communicationRuntimeActor(
	ctx context.Context,
	agents AgentResolver,
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
	lease, ok := runtimectx.RuntimeRoundLeaseFromContext(ctx)
	if !ok {
		return communicationsvc.Actor{}, false
	}
	record, err := agents.GetAgent(ctx, strings.TrimSpace(agentValue.AgentID))
	if err != nil || record == nil || record.IsMain {
		return communicationsvc.Actor{}, false
	}
	recordAgentID := strings.TrimSpace(record.AgentID)
	ownerUserID := strings.TrimSpace(record.OwnerUserID)
	if recordAgentID == "" || ownerUserID == "" {
		return communicationsvc.Actor{}, false
	}
	if principal := authctx.PrincipalFromContext(ctx); principal != nil &&
		strings.TrimSpace(principal.UserID) != ownerUserID {
		return communicationsvc.Actor{}, false
	}
	actor := communicationsvc.Actor{
		OwnerUserID: ownerUserID, AgentID: recordAgentID,
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
			parsed.ConversationID == "" {
			return communicationsvc.Actor{}, false
		}
		actor.RoomID = sourceContextID
		actor.ConversationID = parsed.ConversationID
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
