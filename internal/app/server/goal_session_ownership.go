// INPUT: Goal create 的 authenticated owner、canonical Agent/Room session key 与可选 runtime Agent identity。
// OUTPUT: owner-scoped Agent/Room 持久事实证明及经成员目录验证的可信 Room creator/lead identity。
// POS: App 装配层跨 Agent/Room 服务实现 GoalSessionOwnershipVerifier 的唯一适配器。
package server

import (
	"context"
	"errors"
	"fmt"
	"strings"

	roomdomain "github.com/nexus-research-lab/nexus/internal/chat/room"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
	roomsvc "github.com/nexus-research-lab/nexus/internal/service/room"
)

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

func newGoalSessionOwnershipVerifier(
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
		// The persisted Agent identity is the ownership boundary for every Agent
		// session. Its ref may be a provider chat, a DM thread, or a Room member
		// runtime, so it must not be reinterpreted as a Room conversation here.
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
