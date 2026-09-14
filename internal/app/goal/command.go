// INPUT: Goal command、runtime authority 与 owner-scoped Agent/Room 持久事实。
// OUTPUT: canonical Session 路由、round 权限与 Goal 会话所有权证明。
// POS: Goal 命令进入 DM/Room 业务服务前的应用层信任边界。
package goal

import (
	"context"
	"errors"
	"fmt"
	"strings"

	roomdomain "github.com/nexus-research-lab/nexus/internal/chat/room"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	goalcontract "github.com/nexus-research-lab/nexus/internal/mcp/command/goal/contract"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
	dmsvc "github.com/nexus-research-lab/nexus/internal/service/dm"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
	roomsvc "github.com/nexus-research-lab/nexus/internal/service/room"
	roomrealtime "github.com/nexus-research-lab/nexus/internal/service/room/realtime"
)

type goalCommandRouter struct {
	dm    *dmsvc.Service
	room  *roomrealtime.Service
	goals *goalsvc.Service
}

// NewCommandRouter 创建 Agent 与 Room 共用的 Goal 命令路由器。
func NewCommandRouter(
	dm *dmsvc.Service,
	room *roomrealtime.Service,
	goals *goalsvc.Service,
) *goalCommandRouter {
	return &goalCommandRouter{dm: dm, room: room, goals: goals}
}

func (r goalCommandRouter) ExecuteGoalCommand(
	ctx context.Context,
	request protocol.GoalCommandRequest,
) (protocol.GoalCommandResult, error) {
	switch protocol.ParseSessionKey(request.SessionKey).Kind {
	case protocol.SessionKeyKindAgent:
		if r.dm == nil {
			return protocol.GoalCommandResult{}, errors.New("DM service is unavailable")
		}
		return r.dm.SetGoalFromCommand(ctx, request)
	case protocol.SessionKeyKindRoom:
		if r.room == nil {
			return protocol.GoalCommandResult{}, errors.New("Room service is unavailable")
		}
		return r.room.SetGoalFromCommand(ctx, request)
	default:
		return protocol.GoalCommandResult{}, errors.New("Goal command requires an Agent or Room session")
	}
}

func (r goalCommandRouter) DispatchGoalContinuation(ctx context.Context, item protocol.Goal) {
	if r.goals != nil {
		r.goals.DispatchActiveGoalContinuation(ctx, item)
	}
}

type goalCommandMutationAuthorityResolver interface {
	CurrentModelMutationAuthority(context.Context, string, string, string) (*protocol.Goal, error)
}

// ResolveCommandMutationAuthority 从持久 Goal 补齐当前 round 的模型变更权限。
func ResolveCommandMutationAuthority(
	ctx context.Context,
	svc goalcontract.Service,
	sessionKey string,
	sourceContextType string,
	agentValue *protocol.Agent,
	roundAuthority *runtimectx.GoalAuthorityState,
) *runtimectx.GoalAuthorityState {
	if roundAuthority == nil {
		roundAuthority = runtimectx.NewGoalAuthorityState("", 0, "")
	}
	if _, ok := roundAuthority.Load(); ok || agentValue == nil ||
		!allowsDurableGoalOwnerAuthority(sessionKey, sourceContextType) {
		return roundAuthority
	}
	resolver, ok := svc.(goalCommandMutationAuthorityResolver)
	if !ok || resolver == nil {
		return roundAuthority
	}
	item, err := resolver.CurrentModelMutationAuthority(
		ctx,
		sessionKey,
		strings.TrimSpace(agentValue.OwnerUserID),
		strings.TrimSpace(agentValue.AgentID),
	)
	if err != nil || item == nil || strings.TrimSpace(item.ID) == "" || item.ObjectiveRevision() <= 0 {
		return roundAuthority
	}
	return runtimectx.NewGoalAuthorityState(item.ID, item.ObjectiveRevision(), "")
}

func allowsDurableGoalOwnerAuthority(sessionKey string, sourceContextType string) bool {
	if protocol.IsRoomSharedSessionKey(sessionKey) {
		switch strings.TrimSpace(sourceContextType) {
		case "room", "room_handoff":
			return true
		default:
			return false
		}
	}
	return strings.TrimSpace(sourceContextType) == "agent"
}

// AllowsTrustedUserRetarget 判断当前可信来源能否承载用户 Goal 改写。
func AllowsTrustedUserRetarget(sourceContextType string) bool {
	switch strings.TrimSpace(sourceContextType) {
	case "agent", "room":
		return true
	default:
		return false
	}
}

// ResolveCommandSessionKey 把 Room 成员 runtime identity 归一为共享 Room Session。
func ResolveCommandSessionKey(sessionKey string, sourceContextType string) string {
	normalized := strings.TrimSpace(sessionKey)
	if normalized == "" || strings.TrimSpace(sourceContextType) != "room" {
		return normalized
	}
	parsed := protocol.ParseSessionKey(normalized)
	if parsed.Kind == protocol.SessionKeyKindRoom {
		return normalized
	}
	if parsed.Kind == protocol.SessionKeyKindAgent && parsed.ChatType == "group" && strings.TrimSpace(parsed.Ref) != "" {
		return protocol.BuildRoomSharedSessionKey(parsed.Ref)
	}
	return normalized
}

type goalSessionAgentReader interface {
	GetAgent(context.Context, string) (*protocol.Agent, error)
}

type goalSessionRoomReader interface {
	GetConversationContext(context.Context, string) (*protocol.ConversationContextAggregate, error)
}

type goalSessionOwnershipVerifier struct {
	agents goalSessionAgentReader
	rooms  goalSessionRoomReader
}

// NewSessionOwnershipVerifier 创建 Agent/Room 会话所有权验证器。
func NewSessionOwnershipVerifier(
	agents goalSessionAgentReader,
	rooms goalSessionRoomReader,
) *goalSessionOwnershipVerifier {
	return &goalSessionOwnershipVerifier{agents: agents, rooms: rooms}
}

func (v *goalSessionOwnershipVerifier) VerifyGoalSessionOwnership(
	ctx context.Context,
	request goalsvc.GoalSessionOwnershipRequest,
) (goalsvc.GoalSessionOwnershipProof, error) {
	if v == nil || v.agents == nil || v.rooms == nil {
		return goalsvc.GoalSessionOwnershipProof{}, errors.New("Goal session ownership verifier is unavailable")
	}
	ownerUserID := strings.TrimSpace(request.OwnerUserID)
	if ownerUserID == "" {
		return goalsvc.GoalSessionOwnershipProof{}, errors.New("Goal session owner is required")
	}
	ownerContext, err := goalSessionOwnerContext(ctx, ownerUserID)
	if err != nil {
		return goalsvc.GoalSessionOwnershipProof{}, err
	}
	parsed := protocol.ParseSessionKey(request.SessionKey)
	if !parsed.IsStructured {
		return goalsvc.GoalSessionOwnershipProof{}, errors.New("Goal session key is not canonical")
	}
	trustedAgentID := strings.TrimSpace(request.TrustedAgentID)
	switch parsed.Kind {
	case protocol.SessionKeyKindAgent:
		targetAgent, agentErr := v.agents.GetAgent(ownerContext, parsed.AgentID)
		if agentErr != nil || targetAgent == nil ||
			strings.TrimSpace(targetAgent.OwnerUserID) != ownerUserID {
			return goalsvc.GoalSessionOwnershipProof{}, errors.New("Goal Agent session is outside the owner scope")
		}
		if trustedAgentID != "" && trustedAgentID != strings.TrimSpace(targetAgent.AgentID) {
			return goalsvc.GoalSessionOwnershipProof{}, errors.New("runtime Agent does not own the target DM session")
		}
		// 持久 Agent 身份是所有 Agent Session 的所有权边界，不能把 ref 重新解释成 Room。
		return goalsvc.GoalSessionOwnershipProof{
			TrustedAgentID:   trustedAgentID,
			TrustedAgentName: goalSessionAgentName(targetAgent),
		}, nil
	case protocol.SessionKeyKindRoom:
		_, roomErr := v.verifyRoomConversation(
			ownerContext,
			ownerUserID,
			parsed.ConversationID,
			trustedAgentID,
		)
		if roomErr != nil {
			return goalsvc.GoalSessionOwnershipProof{}, roomErr
		}
		trustedAgentName := ""
		if trustedAgentID != "" {
			agentValue, agentErr := v.agents.GetAgent(ownerContext, trustedAgentID)
			if agentErr != nil || agentValue == nil ||
				strings.TrimSpace(agentValue.OwnerUserID) != ownerUserID {
				return goalsvc.GoalSessionOwnershipProof{}, errors.New("Room Goal runtime Agent is outside the owner scope")
			}
			trustedAgentName = goalSessionAgentName(agentValue)
		}
		return goalsvc.GoalSessionOwnershipProof{
			TrustedAgentID:   trustedAgentID,
			TrustedAgentName: trustedAgentName,
		}, nil
	default:
		return goalsvc.GoalSessionOwnershipProof{}, errors.New("unsupported Goal session kind")
	}
}

func goalSessionAgentName(agentValue *protocol.Agent) string {
	if agentValue == nil {
		return ""
	}
	if value := strings.TrimSpace(agentValue.DisplayName); value != "" {
		return value
	}
	return strings.TrimSpace(agentValue.Name)
}

func (v *goalSessionOwnershipVerifier) verifyRoomConversation(
	ctx context.Context,
	ownerUserID string,
	conversationID string,
	trustedAgentID string,
) (*protocol.ConversationContextAggregate, error) {
	contextValue, err := v.rooms.GetConversationContext(ctx, strings.TrimSpace(conversationID))
	if err != nil || contextValue == nil ||
		strings.TrimSpace(contextValue.Room.OwnerUserID) != ownerUserID {
		return nil, errors.New("Goal Room conversation is outside the owner scope")
	}
	trustedAgentID = strings.TrimSpace(trustedAgentID)
	if trustedAgentID != "" && !roomdomain.IsMemberAgent(contextValue.Members, trustedAgentID) {
		return nil, errors.New("runtime Agent is not a member of the target Room")
	}
	return contextValue, nil
}

func goalSessionOwnerContext(ctx context.Context, ownerUserID string) (context.Context, error) {
	if currentOwner, ok := authctx.CurrentUserID(ctx); ok {
		if strings.TrimSpace(currentOwner) != ownerUserID {
			return nil, fmt.Errorf("authenticated owner %q does not match Goal owner", currentOwner)
		}
		return ctx, nil
	}
	return authctx.WithPrincipal(ctx, &authctx.Principal{
		UserID: ownerUserID,
		Role:   authctx.RoleOwner,
	}), nil
}

var (
	_ goalSessionAgentReader               = (*agentsvc.Service)(nil)
	_ goalSessionRoomReader                = (*roomsvc.Service)(nil)
	_ goalsvc.GoalSessionOwnershipVerifier = (*goalSessionOwnershipVerifier)(nil)
)
