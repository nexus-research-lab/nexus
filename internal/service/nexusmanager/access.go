// INPUT: server 固化的业务上下文/runtime lease 与当前数据库/round 状态。
// OUTPUT: lease 不可跨 DM/Room slot 转移的 owner-main、agent-self 或 Room member 权限结论。
// POS: nexus_manager 每个操作必须经过的唯一授权边界。
package nexusmanager

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type resolvedActor struct {
	Actor
	Agent       *protocol.Agent
	Authority   string
	Context     context.Context
	RoomContext *protocol.ConversationContextAggregate
}

func (s *Service) resolveActor(ctx context.Context, actor Actor) (*resolvedActor, error) {
	actor.OwnerUserID = strings.TrimSpace(actor.OwnerUserID)
	actor.AgentID = strings.TrimSpace(actor.AgentID)
	actor.SessionKey = strings.TrimSpace(actor.SessionKey)
	actor.RoundID = strings.TrimSpace(actor.RoundID)
	actor.LeaseSessionKey = strings.TrimSpace(actor.LeaseSessionKey)
	actor.LeaseRoundID = strings.TrimSpace(actor.LeaseRoundID)
	actor.ContextKind = strings.ToLower(strings.TrimSpace(actor.ContextKind))
	actor.ContextID = strings.TrimSpace(actor.ContextID)
	actor.RoomID = strings.TrimSpace(actor.RoomID)
	actor.ConversationID = strings.TrimSpace(actor.ConversationID)
	if actor.OwnerUserID == "" || actor.AgentID == "" {
		return nil, errors.New("nexus_manager 调用缺少可信 owner 或 agent 身份")
	}
	if principal := authctx.PrincipalFromContext(ctx); principal != nil &&
		strings.TrimSpace(principal.UserID) != actor.OwnerUserID {
		return nil, errors.New("nexus_manager 认证主体与 owner 作用域不匹配")
	}
	if err := s.requireActiveRound(actor); err != nil {
		return nil, err
	}
	if s.agents == nil {
		return nil, errors.New("nexus_manager 未装配 Agent 身份服务")
	}
	scoped := authctx.WithPrincipal(ctx, &authctx.Principal{
		UserID: actor.OwnerUserID, Username: actor.AgentID,
		Role: authctx.RoleMember, AuthMethod: "nexus_manager_runtime",
	})
	agentValue, err := s.agents.GetAgent(scoped, actor.AgentID)
	if err != nil {
		return nil, fmt.Errorf("重新验证 nexus_manager Agent 身份: %w", err)
	}
	if agentValue == nil || strings.TrimSpace(agentValue.OwnerUserID) != actor.OwnerUserID {
		return nil, errors.New("nexus_manager Agent 与 owner 作用域不匹配")
	}
	resolved := &resolvedActor{Actor: actor, Agent: agentValue, Context: scoped}
	switch actor.ContextKind {
	case ContextKindAgent:
		return s.resolveAgentActor(resolved)
	case ContextKindRoom:
		return s.resolveRoomActor(resolved)
	default:
		return nil, errors.New("nexus_manager 只接受可信的 Agent 私有 DM 或 Room 上下文")
	}
}

func (s *Service) requireActiveRound(actor Actor) error {
	if s.runtime == nil {
		return errors.New("nexus_manager 缺少 runtime round 校验器")
	}
	if actor.LeaseSessionKey == "" || actor.LeaseRoundID == "" {
		return errors.New("nexus_manager 调用缺少可信 session 或 round lease")
	}
	if slices.Contains(s.runtime.GetRunningRoundIDs(actor.LeaseSessionKey), actor.LeaseRoundID) {
		return nil
	}
	return errors.New("nexus_manager 调用所属 round 已结束或不再可信")
}

func (s *Service) resolveAgentActor(actor *resolvedActor) (*resolvedActor, error) {
	if actor.ContextID != actor.AgentID {
		return nil, errors.New("nexus_manager 上下文与当前 Agent 私有 DM 不匹配")
	}
	if actor.SessionKey != actor.LeaseSessionKey || actor.RoundID != actor.LeaseRoundID {
		return nil, errors.New("nexus_manager 私有 DM 业务身份与 runtime lease 不匹配")
	}
	parsed := protocol.ParseSessionKey(actor.SessionKey)
	if !parsed.IsStructured ||
		parsed.Kind != protocol.SessionKeyKindAgent ||
		parsed.Channel != protocol.SessionChannelWebSocketSegment ||
		parsed.ChatType != protocol.RoomTypeDM ||
		strings.TrimSpace(parsed.AgentID) != actor.AgentID {
		return nil, errors.New("nexus_manager owner/self 能力只允许 WebSocket 私有 DM")
	}
	if actor.Agent.IsMain {
		actor.Authority = AuthorityOwnerMain
	} else {
		actor.Authority = AuthorityAgentSelf
	}
	return actor, nil
}

func (s *Service) resolveRoomActor(actor *resolvedActor) (*resolvedActor, error) {
	if s.rooms == nil {
		return nil, errors.New("nexus_manager 未装配 Room 服务")
	}
	roomID := actor.RoomID
	if roomID == "" {
		roomID = actor.ContextID
	}
	if roomID == "" || actor.ContextID != roomID {
		return nil, errors.New("nexus_manager Room 上下文缺少固定 room_id")
	}
	parsed := protocol.ParseSessionKey(actor.SessionKey)
	if !parsed.IsStructured ||
		parsed.Kind != protocol.SessionKeyKindRoom ||
		strings.TrimSpace(parsed.ConversationID) == "" ||
		strings.TrimSpace(parsed.ConversationID) != actor.ConversationID {
		return nil, errors.New("nexus_manager Room session 与 conversation 不匹配")
	}
	authorization, err := s.rooms.GetRoomAuthorizationSnapshot(
		actor.Context, roomID, actor.AgentID,
	)
	if err != nil {
		return nil, fmt.Errorf("重新验证 nexus_manager Room 成员身份: %w", err)
	}
	if authorization == nil ||
		strings.TrimSpace(authorization.RoomID) != roomID ||
		!authorization.AgentIsMember {
		return nil, errors.New("当前 Agent 已不是该 Room 成员")
	}
	contextValue, err := s.rooms.GetConversationContext(actor.Context, actor.ConversationID)
	if err != nil {
		return nil, fmt.Errorf("重新验证 nexus_manager conversation: %w", err)
	}
	if contextValue == nil || strings.TrimSpace(contextValue.Room.ID) != roomID {
		return nil, errors.New("nexus_manager conversation 不属于当前 Room")
	}
	expectedLeaseSession := protocol.BuildRoomAgentSessionKey(
		actor.ConversationID,
		actor.AgentID,
		contextValue.Room.RoomType,
	)
	if actor.LeaseSessionKey != expectedLeaseSession {
		return nil, errors.New("nexus_manager runtime lease 不属于当前 Room Agent slot")
	}
	actor.RoomID = roomID
	actor.ContextID = roomID
	actor.RoomContext = contextValue
	actor.Authority = AuthorityRoomMember
	return actor, nil
}

func requireOwnerMain(actor *resolvedActor) error {
	if actor == nil || actor.Authority != AuthorityOwnerMain {
		return errors.New("该 nexus_manager 操作只允许主智能体在自己的私有 DM 中执行")
	}
	return nil
}

func (s *Service) workspaceAgent(actor *resolvedActor, requestedAgentID string) (string, error) {
	requestedAgentID = strings.TrimSpace(requestedAgentID)
	switch actor.Authority {
	case AuthorityOwnerMain:
		if requestedAgentID == "" {
			return "", errors.New("主智能体读取 workspace 时必须指定 agent_id")
		}
		target, err := s.agents.GetAgent(actor.Context, requestedAgentID)
		if err != nil {
			return "", err
		}
		if target == nil || strings.TrimSpace(target.OwnerUserID) != actor.OwnerUserID {
			return "", errors.New("目标 Agent 不属于当前 owner")
		}
		return target.AgentID, nil
	case AuthorityAgentSelf:
		if requestedAgentID != "" && requestedAgentID != actor.AgentID {
			return "", errors.New("普通 Agent 只能读取自己的 workspace")
		}
		return actor.AgentID, nil
	default:
		return "", errors.New("Room 上下文不能读取任何 Agent workspace")
	}
}
