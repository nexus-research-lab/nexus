// INPUT: runtime 注入的 Actor 与当前数据库中的 Agent/Room 身份。
// OUTPUT: 每次调用重新解析的 owner-main、agent-self、room-host 或 room-member 权限。
// POS: configuration 的唯一身份与作用域判定边界；工具参数不得覆盖这里的结果。
package configuration

import (
	"context"
	"errors"
	"fmt"
	"strings"

	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type roomConfigurationService interface {
	ListRooms(context.Context, int) ([]protocol.RoomAggregate, error)
	GetRoom(context.Context, string) (*protocol.RoomAggregate, error)
	GetRoomContexts(context.Context, string) ([]protocol.ConversationContextAggregate, error)
	GetRoomAuthorizationSnapshot(context.Context, string, string) (*protocol.RoomAuthorizationSnapshot, error)
	GetConversationContext(context.Context, string) (*protocol.ConversationContextAggregate, error)
	CreateRoom(context.Context, protocol.CreateRoomRequest) (*protocol.ConversationContextAggregate, error)
	CreateConversationAtVersion(context.Context, string, protocol.CreateConversationRequest, int64) (*protocol.ConversationContextAggregate, error)
	UpdateConversationAtVersion(context.Context, string, string, protocol.UpdateConversationRequest, int64) (*protocol.ConversationContextAggregate, error)
	DeleteConversationAtVersion(context.Context, string, string, int64) (*protocol.ConversationContextAggregate, error)
	UpdateRoom(context.Context, string, protocol.UpdateRoomRequest) (*protocol.ConversationContextAggregate, error)
	AddRoomMember(context.Context, string, protocol.AddRoomMemberRequest) (*protocol.ConversationContextAggregate, error)
	AddRoomMemberAtVersion(context.Context, string, protocol.AddRoomMemberRequest, int64) (*protocol.ConversationContextAggregate, error)
	RemoveRoomMember(context.Context, string, string) (*protocol.ConversationContextAggregate, error)
	RemoveRoomMemberAtVersion(context.Context, string, string, int64) (*protocol.ConversationContextAggregate, error)
	DeleteRoomAtVersion(context.Context, string, int64) error
}

type sessionConfigurationService interface {
	ListMutableSessions(context.Context) ([]protocol.Session, error)
	GetSession(context.Context, string) (*protocol.Session, error)
	GetMutableSession(context.Context, string) (*protocol.Session, error)
	UpdateSessionTitleAtVersion(context.Context, string, string, int64) (*protocol.Session, error)
	DeleteSessionAtVersion(context.Context, string, int64) error
}

type roomRuntimeController interface {
	SetPermissionModeForAgent(context.Context, string, sdkpermission.Mode) error
	InterruptAgentTasks(context.Context, string, string, string) error
}

type roomParticipationController interface {
	SetRoomMemberParticipationAtVersion(
		context.Context,
		string,
		string,
		bool,
		int64,
	) (*protocol.ConversationContextAggregate, error)
}

type configurationNotifier interface {
	AgentChanged(context.Context, string, string)
	RoomChanged(context.Context, string, string, string)
	RoomMemberChanged(context.Context, string, string, bool)
}

type resolvedActor struct {
	Actor
	Agent             *protocol.Agent
	RoomAuthorization *protocol.RoomAuthorizationSnapshot
	Authority         string
	Context           ScopeRef
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
		return nil, errors.New("配置调用缺少可信 owner 或 agent 身份")
	}
	if principal := authctx.PrincipalFromContext(ctx); principal != nil &&
		strings.TrimSpace(principal.UserID) != actor.OwnerUserID {
		return nil, errors.New("配置调用的认证主体与 owner 作用域不匹配")
	}
	if queued, ok := authctx.QueuedHumanPrincipalBindingFromContext(ctx); ok &&
		strings.TrimSpace(queued.UserID) != actor.OwnerUserID {
		return nil, errors.New("配置调用的 queued human principal 与 owner 作用域不匹配")
	}
	if err := s.requireActiveRound(actor); err != nil {
		return nil, err
	}
	actor = actorWithTrustedRequestPrincipal(ctx, actor)
	if authctx.IsLocalSingleUserControlPlane(ctx, actor.OwnerUserID) {
		actor.PrincipalRole = authctx.RoleOwner
	} else if s.roleResolver != nil {
		role, roleErr := s.roleResolver.ResolveActivePrincipalRole(ctx, actor.OwnerUserID)
		if roleErr != nil {
			return nil, fmt.Errorf("重新验证配置 principal 角色: %w", roleErr)
		}
		actor.PrincipalRole = strings.TrimSpace(role)
	}
	if s.agents == nil {
		return nil, fmt.Errorf("%w：Agent 身份服务未装配，不能信任调用方自报的 main 身份", ErrMainAgentRequired)
	}
	scoped := scopedContext(ctx, actor)
	agentValue, err := s.agents.GetAgent(scoped, actor.AgentID)
	if err != nil {
		return nil, fmt.Errorf("验证配置 actor: %w", err)
	}
	if agentValue == nil || strings.TrimSpace(agentValue.OwnerUserID) != actor.OwnerUserID {
		return nil, errors.New("配置 actor 与 owner 作用域不匹配")
	}
	actor.IsMainAgent = agentValue.IsMain
	resolved := &resolvedActor{Actor: actor, Agent: agentValue}
	switch actor.ContextKind {
	case ContextKindAgent:
		if actor.ContextID != actor.AgentID {
			return nil, errors.New("配置上下文与当前 Agent 私有 DM 不匹配")
		}
		if err = validateAgentRuntimeIdentity(actor); err != nil {
			return nil, err
		}
		if agentValue.IsMain {
			resolved.Authority = AuthorityOwnerMain
			resolved.Context = ScopeRef{Kind: ScopeKindOwner, ID: actor.OwnerUserID}
			return resolved, nil
		}
		resolved.Authority = AuthorityAgentSelf
		resolved.Context = ScopeRef{Kind: ScopeKindAgent, ID: actor.AgentID}
		return resolved, nil
	case ContextKindRoom:
		// Group Room 的成员写入口本就拒绝主智能体；这里再次 fail closed，
		// 防止历史坏数据或旁路写入把 owner control plane 投影成 Room 成员。
		if agentValue.IsMain {
			return nil, errors.New("主智能体不能作为 Group Room 成员使用配置能力")
		}
		return s.resolveRoomActor(scoped, resolved)
	default:
		return nil, errors.New("配置调用缺少可信 agent 或 room 上下文")
	}
}

func (s *Service) requireActiveRound(actor Actor) error {
	if !actor.RoundLeaseRequired {
		return nil
	}
	if s.runtime == nil {
		return errors.New("配置调用缺少 runtime round 校验器")
	}
	if actor.LeaseSessionKey == "" || actor.LeaseRoundID == "" {
		return errors.New("配置调用缺少可信 session 或 round lease")
	}
	for _, roundID := range s.runtime.GetRunningRoundIDs(actor.LeaseSessionKey) {
		if roundID == actor.LeaseRoundID {
			return nil
		}
	}
	return errors.New("配置调用所属 round 已结束或不再可信")
}

func (s *Service) resolveRoomActor(ctx context.Context, actor *resolvedActor) (*resolvedActor, error) {
	if s.rooms == nil {
		return nil, errors.New("Room 配置控制尚未装配")
	}
	roomID := actor.RoomID
	if roomID == "" {
		roomID = actor.ContextID
	}
	if roomID == "" {
		return nil, errors.New("Room 配置调用缺少可信 room_id")
	}
	roomAuthorization, err := s.rooms.GetRoomAuthorizationSnapshot(ctx, roomID, actor.AgentID)
	if err != nil {
		return nil, fmt.Errorf("验证 Room 配置上下文: %w", err)
	}
	if roomAuthorization == nil || roomAuthorization.RoomID != roomID {
		return nil, errors.New("Room 配置上下文不匹配")
	}
	if !roomAuthorization.AgentIsMember {
		return nil, errors.New("当前 Agent 已不是该 Room 成员")
	}
	if err = s.validateRoomRuntimeIdentity(ctx, actor.Actor, roomID); err != nil {
		return nil, err
	}
	actor.RoomAuthorization = roomAuthorization
	actor.RoomID = roomID
	actor.ContextID = roomID
	actor.Context = ScopeRef{Kind: ScopeKindRoom, ID: roomID}
	if strings.TrimSpace(roomAuthorization.HostAgentID) == actor.AgentID {
		actor.Authority = AuthorityRoomHost
	} else {
		actor.Authority = AuthorityRoomMember
	}
	return actor, nil
}

func validateAgentRuntimeIdentity(actor Actor) error {
	if !actor.RoundLeaseRequired {
		return nil
	}
	if actor.SessionKey != actor.LeaseSessionKey ||
		actor.RoundID != actor.LeaseRoundID {
		return errors.New("配置私有 DM 的业务身份与 runtime lease 不匹配")
	}
	parsed := protocol.ParseSessionKey(actor.SessionKey)
	if !parsed.IsStructured ||
		parsed.Kind != protocol.SessionKeyKindAgent ||
		parsed.Channel != protocol.SessionChannelWebSocketSegment ||
		parsed.ChatType != protocol.RoomTypeDM ||
		strings.TrimSpace(parsed.AgentID) != actor.AgentID {
		return errors.New("配置 owner/self 能力只允许当前 Agent 的 WebSocket 私有 DM")
	}
	return nil
}

func (s *Service) validateRoomRuntimeIdentity(
	ctx context.Context,
	actor Actor,
	roomID string,
) error {
	if !actor.RoundLeaseRequired {
		return nil
	}
	conversationID := strings.TrimSpace(actor.ConversationID)
	if conversationID == "" ||
		actor.SessionKey != protocol.BuildRoomSharedSessionKey(conversationID) {
		return errors.New("Room 配置业务 session 与 conversation 不匹配")
	}
	contextValue, err := s.rooms.GetConversationContext(ctx, conversationID)
	if err != nil {
		return fmt.Errorf("验证 Room 配置 conversation: %w", err)
	}
	return validateRoomRuntimeIdentityValue(actor, roomID, contextValue)
}

func validateRoomRuntimeIdentityValue(
	actor Actor,
	roomID string,
	contextValue *protocol.ConversationContextAggregate,
) error {
	conversationID := strings.TrimSpace(actor.ConversationID)
	if conversationID == "" ||
		actor.SessionKey != protocol.BuildRoomSharedSessionKey(conversationID) {
		return errors.New("Room 配置业务 session 与 conversation 不匹配")
	}
	if contextValue == nil ||
		strings.TrimSpace(contextValue.Room.ID) != roomID ||
		strings.TrimSpace(contextValue.Conversation.ID) != conversationID {
		return errors.New("Room 配置 conversation 不属于当前 Room")
	}
	expectedLeaseSession := protocol.BuildRoomAgentSessionKey(
		conversationID,
		actor.AgentID,
		contextValue.Room.RoomType,
	)
	if actor.LeaseSessionKey != expectedLeaseSession {
		return errors.New("Room 配置 runtime lease 不属于当前 Room Agent slot")
	}
	return nil
}

func (r *resolvedActor) isMain() bool {
	return r != nil && r.Authority == AuthorityOwnerMain
}

func (r *resolvedActor) isSelfDM() bool {
	return r != nil && r.Authority == AuthorityAgentSelf
}

func (r *resolvedActor) isRoomHost() bool {
	return r != nil && r.Authority == AuthorityRoomHost
}
