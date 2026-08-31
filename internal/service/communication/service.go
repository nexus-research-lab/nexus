// INPUT: runtime 固化 Actor、Agent 联系人、Room 成员关系、外部 Session 与消息正文。
// OUTPUT: 当前 Agent 的好友/群/外部私聊通讯录，以及现有 transport 的发送回执。
// POS: 平台 Agent 通讯业务边界；不定义第二套消息、队列、Session 或 SDK team 协议。
package communication

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"

	roomdomain "github.com/nexus-research-lab/nexus/internal/chat/room"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
	"github.com/nexus-research-lab/nexus/internal/service/channels"
	roomsvc "github.com/nexus-research-lab/nexus/internal/service/room"
)

const (
	ContextKindAgent          = "agent"
	ContextKindRoom           = "room"
	TargetTypeAgent           = "agent"
	TargetTypeRoom            = "room"
	TargetTypeExternalSession = "external_session"
	// ponytail: 本地通讯录先复用最近 Room 查询；单个 Agent 超过 300 个活跃群时改成成员游标分页。
	addressBookRoomLimit = 300
)

// Actor 是宿主根据当前 runtime 固化的通讯身份；字段不能来自模型参数。
type Actor struct {
	OwnerUserID     string
	AgentID         string
	SessionKey      string
	RoundID         string
	LeaseSessionKey string
	LeaseRoundID    string
	ContextKind     string
	ContextID       string
	RoomID          string
	ConversationID  string
	// GoalCollaborationBinding 只提供协作归因，不向目标 round 传播能力。
	GoalCollaborationBinding func() *protocol.GoalCollaborationBinding
}

// AddressBook 是一个 Agent 当前可寻址的好友、群与外部私聊。
type AddressBook struct {
	AgentID          string                          `json:"agent_id"`
	Contacts         []protocol.AgentContact         `json:"contacts"`
	Rooms            []RoomContact                   `json:"rooms"`
	ExternalSessions []channels.AgentExternalSession `json:"external_sessions"`
}

// RoomContact 是通讯录中的当前成员群。
type RoomContact struct {
	RoomID              string   `json:"room_id"`
	ConversationID      string   `json:"conversation_id"`
	Name                string   `json:"name"`
	Avatar              string   `json:"avatar,omitempty"`
	MemberAgentIDs      []string `json:"member_agent_ids"`
	ParticipationPaused bool     `json:"participation_paused"`
}

// SendRequest 表示一次平台通讯发送。
type SendRequest struct {
	TargetType     string `json:"target_type"`
	TargetID       string `json:"target_id"`
	ConversationID string `json:"conversation_id,omitempty"`
	Content        string `json:"content"`
}

// SendResult 返回现有 Room 或 Channels transport 的定位与接受状态。
type SendResult struct {
	MessageID      string `json:"message_id"`
	Status         string `json:"status"`
	TargetType     string `json:"target_type"`
	TargetID       string `json:"target_id"`
	RoomID         string `json:"room_id"`
	ConversationID string `json:"conversation_id"`
	RoutingSource  string `json:"routing_source,omitempty"`
	SessionKey     string `json:"session_key,omitempty"`
	Channel        string `json:"channel,omitempty"`
}

const (
	RoutingSourceExplicit       = "explicit"
	RoutingSourceCurrentContext = "current_context"
	RoutingSourceRoomMain       = "room_main"
)

// sendContext carries only host-minted invocation identity. It is never built
// from model arguments and is not propagated as capability to a target round.
type sendContext struct {
	RoomID         string
	ConversationID string
	RootRoundID    string
	GoalBinding    *protocol.GoalCollaborationBinding
}

// Service 组合现有联系人、Room、Channels 和 realtime 消息主链。
type Service struct {
	agents   *agentsvc.Service
	rooms    *roomsvc.Service
	realtime messageTransport
	runtime  roundLeaseVerifier
	external agentExternalSessionGateway
	// ponytail: 本地单进程产品先用一把锁避免同一好友对并发创建重复 Room；出现多实例写入时改成数据库 pair lease。
	directMu sync.Mutex
}

type roundLeaseVerifier interface {
	GetRunningRoundIDs(string) []string
}

type messageTransport interface {
	HandleDirectedMessage(context.Context, string, string, protocol.CreateRoomDirectedMessageRequest) (*protocol.RoomDirectedMessageRecord, error)
	HandlePlatformPublicMessage(context.Context, string, string, protocol.CreateRoomPublicMessageRequest) (protocol.Message, error)
}

type agentExternalSessionGateway interface {
	ListAgentExternalSessions(context.Context, string, string, string) ([]channels.AgentExternalSession, error)
	SendAgentExternalSessionMessage(context.Context, string, string, string, string) (channels.DeliveryResult, error)
}

// NewService 创建平台通讯服务。
func NewService(
	agents *agentsvc.Service,
	rooms *roomsvc.Service,
	realtime messageTransport,
	runtime roundLeaseVerifier,
	external agentExternalSessionGateway,
) *Service {
	return &Service{
		agents: agents, rooms: rooms, realtime: realtime, runtime: runtime, external: external,
	}
}

// ListAddressBook 返回当前 Agent 的好友与所在 Group Room。
func (s *Service) ListAddressBook(ctx context.Context, actor Actor) (*AddressBook, error) {
	scoped, current, err := s.authorize(ctx, actor)
	if err != nil {
		return nil, err
	}
	contacts, err := s.agents.ListAgentContacts(scoped, current.AgentID)
	if err != nil {
		return nil, err
	}
	rooms, err := s.rooms.ListRooms(scoped, addressBookRoomLimit)
	if err != nil {
		return nil, err
	}
	externalSessions, err := s.listExternalSessions(scoped, current.AgentID)
	if err != nil {
		return nil, err
	}
	result := &AddressBook{
		AgentID: current.AgentID, Contacts: contacts, Rooms: make([]RoomContact, 0),
		ExternalSessions: externalSessions,
	}
	for _, roomValue := range rooms {
		if roomValue.Room.RoomType != protocol.RoomTypeGroup ||
			!roomdomain.IsMemberAgent(roomValue.Members, current.AgentID) {
			continue
		}
		contexts, contextErr := s.rooms.GetRoomContexts(scoped, roomValue.Room.ID)
		if contextErr != nil {
			return nil, contextErr
		}
		conversation, ok := mainConversation(contexts)
		if !ok {
			continue
		}
		result.Rooms = append(result.Rooms, RoomContact{
			RoomID: roomValue.Room.ID, ConversationID: conversation.ID,
			Name: roomValue.Room.Name, Avatar: roomValue.Room.Avatar,
			MemberAgentIDs:      roomdomain.ListAgentIDs(roomValue.Members),
			ParticipationPaused: memberParticipationPaused(roomValue.Members, current.AgentID),
		})
	}
	return result, nil
}

// SendMessage 给好友发私域消息，或向当前成员群发布公区消息。
func (s *Service) SendMessage(
	ctx context.Context,
	actor Actor,
	request SendRequest,
) (*SendResult, error) {
	scoped, current, err := s.authorize(ctx, actor)
	if err != nil {
		return nil, err
	}
	if (strings.EqualFold(strings.TrimSpace(request.TargetType), TargetTypeAgent) ||
		strings.EqualFold(strings.TrimSpace(request.TargetType), TargetTypeExternalSession)) &&
		strings.TrimSpace(request.ConversationID) != "" {
		return nil, newInputError("conversation_id 只支持 owner 通讯客户端或 room 目标")
	}
	if strings.EqualFold(strings.TrimSpace(request.TargetType), TargetTypeExternalSession) &&
		strings.TrimSpace(request.TargetID) == strings.TrimSpace(actor.SessionKey) {
		return nil, newInputError("当前外部私聊请直接使用 final reply")
	}
	trusted := sendContext{}
	if actor.ContextKind == ContextKindRoom {
		trusted.RoomID = strings.TrimSpace(actor.RoomID)
		trusted.ConversationID = strings.TrimSpace(actor.ConversationID)
		trusted.RootRoundID = strings.TrimSpace(actor.RoundID)
		if actor.GoalCollaborationBinding != nil {
			trusted.GoalBinding = protocol.NormalizeGoalCollaborationBinding(
				actor.GoalCollaborationBinding(),
			)
		}
	}
	return s.sendMessage(
		scoped, current.AgentID, request, protocol.RoomWakePolicyImmediate, trusted,
	)
}

// OpenContactChannel 为 owner 控制面打开已有好友直聊通道。
func (s *Service) OpenContactChannel(
	ctx context.Context,
	sourceAgentID string,
	targetAgentID string,
) (*protocol.ConversationContextAggregate, error) {
	current, err := s.requireOwnerAgent(ctx, sourceAgentID)
	if err != nil {
		return nil, err
	}
	targetAgentID = strings.TrimSpace(targetAgentID)
	if targetAgentID == "" || targetAgentID == current.AgentID {
		return nil, newInputError("目标 Agent 不可用")
	}
	s.directMu.Lock()
	defer s.directMu.Unlock()
	return s.findDirectRoom(ctx, current.AgentID, targetAgentID)
}

// SendMessageAsAgent 允许 owner 在控制面以自己管理的普通 Agent 身份发消息。
func (s *Service) SendMessageAsAgent(
	ctx context.Context,
	sourceAgentID string,
	request SendRequest,
) (*SendResult, error) {
	current, err := s.requireOwnerAgent(ctx, sourceAgentID)
	if err != nil {
		return nil, err
	}
	return s.sendMessage(
		ctx, current.AgentID, request, protocol.RoomWakePolicyNone, sendContext{},
	)
}

func (s *Service) sendMessage(
	ctx context.Context,
	sourceAgentID string,
	request SendRequest,
	replyWakePolicy protocol.RoomWakePolicy,
	trusted sendContext,
) (*SendResult, error) {
	request.TargetType = strings.ToLower(strings.TrimSpace(request.TargetType))
	request.TargetID = strings.TrimSpace(request.TargetID)
	request.ConversationID = strings.TrimSpace(request.ConversationID)
	request.Content = strings.TrimSpace(request.Content)
	if request.TargetID == "" || request.Content == "" {
		return nil, newInputError("target_id 和 content 不能为空")
	}
	switch request.TargetType {
	case TargetTypeAgent:
		return s.sendToAgent(ctx, sourceAgentID, request, replyWakePolicy, trusted)
	case TargetTypeRoom:
		return s.sendToRoom(ctx, sourceAgentID, request, trusted)
	case TargetTypeExternalSession:
		return s.sendToExternalSession(ctx, sourceAgentID, request)
	default:
		return nil, newInputError("target_type 只支持 agent、room 或 external_session")
	}
}

func (s *Service) listExternalSessions(
	ctx context.Context,
	agentID string,
) ([]channels.AgentExternalSession, error) {
	if s.external == nil {
		return nil, errors.New("外部会话通讯未装配")
	}
	return s.external.ListAgentExternalSessions(
		ctx, authctx.OwnerUserID(ctx), strings.TrimSpace(agentID), "",
	)
}

func (s *Service) sendToExternalSession(
	ctx context.Context,
	sourceAgentID string,
	request SendRequest,
) (*SendResult, error) {
	if s.external == nil {
		return nil, errors.New("外部会话通讯未装配")
	}
	result, err := s.external.SendAgentExternalSessionMessage(
		ctx,
		authctx.OwnerUserID(ctx),
		sourceAgentID,
		request.TargetID,
		request.Content,
	)
	if err != nil {
		return nil, err
	}
	messageID := ""
	if result.Receipt != nil {
		messageID = result.Receipt.PrimaryPlatformMessageID
	}
	return &SendResult{
		MessageID: messageID, Status: "delivered",
		TargetType: TargetTypeExternalSession, TargetID: request.TargetID,
		SessionKey: result.Target.SessionKey, Channel: result.Target.Channel,
		RoutingSource: RoutingSourceExplicit,
	}, nil
}

func (s *Service) sendToAgent(
	ctx context.Context,
	sourceAgentID string,
	request SendRequest,
	replyWakePolicy protocol.RoomWakePolicy,
	trusted sendContext,
) (*SendResult, error) {
	if request.TargetID == sourceAgentID {
		return nil, newInputError("Agent 不能给自己发消息")
	}
	contextValue, err := s.ensureDirectRoom(ctx, sourceAgentID, request.TargetID)
	if err != nil {
		return nil, err
	}
	if request.ConversationID != "" {
		selectedContext, contextErr := s.rooms.GetConversationContext(
			ctx, request.ConversationID,
		)
		if contextErr != nil {
			return nil, contextErr
		}
		if selectedContext.Room.ID != contextValue.Room.ID {
			return nil, newInputError("conversation 不属于联系人直聊")
		}
		contextValue = selectedContext
	}
	message, err := s.realtime.HandleDirectedMessage(
		ctx, contextValue.Room.ID, contextValue.Conversation.ID,
		protocol.CreateRoomDirectedMessageRequest{
			SourceAgentID: sourceAgentID,
			RootRoundID:   trusted.RootRoundID,
			GoalCollaborationBinding: protocol.NormalizeGoalCollaborationBinding(
				trusted.GoalBinding,
			),
			Recipients:  []string{request.TargetID},
			WakeTargets: []string{request.TargetID},
			Content:     request.Content,
			WakePolicy:  protocol.RoomWakePolicyImmediate,
			ReplyRoute: protocol.RoomReplyRoute{
				Mode:       protocol.RoomReplyRoutePrivate,
				Recipients: []string{sourceAgentID},
				WakePolicy: replyWakePolicy,
			},
		},
	)
	if err != nil {
		return nil, err
	}
	return &SendResult{
		MessageID: message.MessageID, Status: "queued",
		TargetType: TargetTypeAgent, TargetID: request.TargetID,
		RoomID: contextValue.Room.ID, ConversationID: contextValue.Conversation.ID,
		RoutingSource: RoutingSourceRoomMain,
	}, nil
}

func (s *Service) requireOwnerAgent(
	ctx context.Context,
	agentID string,
) (*protocol.Agent, error) {
	if s == nil || s.agents == nil || s.rooms == nil || s.realtime == nil {
		return nil, errors.New("平台通讯服务未完整装配")
	}
	current, err := s.agents.GetAgent(ctx, strings.TrimSpace(agentID))
	if err != nil {
		return nil, err
	}
	if current.IsMain {
		return nil, newInputError("主智能体只属于控制面，不能进入 Agent 通讯录")
	}
	return current, nil
}

func (s *Service) sendToRoom(
	ctx context.Context,
	sourceAgentID string,
	request SendRequest,
	trusted sendContext,
) (*SendResult, error) {
	conversationID, routingSource, err := resolveRoomConversationID(request, trusted)
	if err != nil {
		return nil, err
	}
	contextValue, err := s.resolveRoomConversation(ctx, request.TargetID, conversationID)
	if err != nil {
		return nil, err
	}
	if !roomdomain.IsMemberAgent(contextValue.Members, sourceAgentID) {
		return nil, roomsvc.ErrRoomMemberNotFound
	}
	if contextValue.Room.IsContactChannel {
		return nil, newInputError("联系人私信通道不能作为群目标")
	}
	message, err := s.realtime.HandlePlatformPublicMessage(
		ctx, contextValue.Room.ID, contextValue.Conversation.ID,
		protocol.CreateRoomPublicMessageRequest{
			SourceAgentID: sourceAgentID,
			RootRoundID:   trusted.RootRoundID,
			GoalCollaborationBinding: protocol.NormalizeGoalCollaborationBinding(
				trusted.GoalBinding,
			),
			Content: request.Content,
		},
	)
	if err != nil {
		return nil, err
	}
	messageID, _ := message["message_id"].(string)
	return &SendResult{
		MessageID: messageID, Status: "published",
		TargetType: TargetTypeRoom, TargetID: request.TargetID,
		RoomID: contextValue.Room.ID, ConversationID: contextValue.Conversation.ID,
		RoutingSource: routingSource,
	}, nil
}

func resolveRoomConversationID(request SendRequest, trusted sendContext) (string, string, error) {
	if conversationID := strings.TrimSpace(request.ConversationID); conversationID != "" {
		return conversationID, RoutingSourceExplicit, nil
	}
	targetRoomID := strings.TrimSpace(request.TargetID)
	if trustedRoomID := strings.TrimSpace(trusted.RoomID); trustedRoomID != "" {
		if targetRoomID != trustedRoomID {
			return "", "", errors.New("从当前 Room 向其他 Room 发消息时必须显式指定 conversation_id")
		}
		if conversationID := strings.TrimSpace(trusted.ConversationID); conversationID != "" {
			return conversationID, RoutingSourceCurrentContext, nil
		}
		return "", "", errors.New("当前 Room runtime 缺少可信 conversation_id")
	}
	return "", RoutingSourceRoomMain, nil
}

func (s *Service) ensureDirectRoom(
	ctx context.Context,
	sourceAgentID string,
	targetAgentID string,
) (*protocol.ConversationContextAggregate, error) {
	s.directMu.Lock()
	defer s.directMu.Unlock()

	contextValue, err := s.findDirectRoom(ctx, sourceAgentID, targetAgentID)
	if err == nil {
		return contextValue, nil
	}
	if !errors.Is(err, roomsvc.ErrRoomNotFound) {
		return nil, err
	}
	created, err := s.rooms.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs:               []string{sourceAgentID, targetAgentID},
		PrivateMessagesEnabled: true,
		IsContactChannel:       true,
	})
	if err != nil {
		return nil, err
	}
	if err = s.agents.SetAgentContactDirectRoom(
		ctx, sourceAgentID, targetAgentID, created.Room.ID,
	); err != nil {
		return nil, errors.Join(err, s.rooms.DeleteRoom(ctx, created.Room.ID))
	}
	return created, nil
}

func (s *Service) findDirectRoom(
	ctx context.Context,
	sourceAgentID string,
	targetAgentID string,
) (*protocol.ConversationContextAggregate, error) {
	contact, err := s.agents.GetAgentContact(ctx, sourceAgentID, targetAgentID)
	if err != nil {
		return nil, err
	}
	if contact.DirectRoomID != "" {
		contextValue, contextErr := s.directRoomContext(
			ctx, contact.DirectRoomID, sourceAgentID, targetAgentID,
		)
		if contextErr == nil {
			return contextValue, nil
		}
		if !errors.Is(contextErr, roomsvc.ErrRoomNotFound) &&
			!errors.Is(contextErr, roomsvc.ErrRoomMemberNotFound) {
			return nil, contextErr
		}
	}
	contextValue, err := s.rooms.FindContactRoomContext(ctx, sourceAgentID, targetAgentID)
	if err != nil {
		return nil, err
	}
	if err = s.agents.SetAgentContactDirectRoom(
		ctx, sourceAgentID, targetAgentID, contextValue.Room.ID,
	); err != nil {
		return nil, err
	}
	return contextValue, nil
}

func (s *Service) directRoomContext(
	ctx context.Context,
	roomID string,
	sourceAgentID string,
	targetAgentID string,
) (*protocol.ConversationContextAggregate, error) {
	contexts, err := s.rooms.GetRoomContexts(ctx, roomID)
	if err != nil {
		return nil, err
	}
	contextValue, ok := mainContext(contexts)
	if !ok {
		return nil, roomsvc.ErrRoomNotFound
	}
	if contextValue.Room.RoomType != protocol.RoomTypeGroup ||
		!contextValue.Room.IsContactChannel ||
		!contextValue.Room.PrivateMessagesEnabled {
		return nil, errors.New("联系人直聊 Room 已停用私域消息")
	}
	members := roomdomain.ListAgentIDs(contextValue.Members)
	if len(members) != 2 ||
		!slices.Contains(members, sourceAgentID) ||
		!slices.Contains(members, targetAgentID) {
		return nil, roomsvc.ErrRoomMemberNotFound
	}
	return contextValue, nil
}

func (s *Service) resolveRoomConversation(
	ctx context.Context,
	roomID string,
	conversationID string,
) (*protocol.ConversationContextAggregate, error) {
	if conversationID != "" {
		contextValue, err := s.rooms.GetConversationContext(ctx, conversationID)
		if err != nil {
			return nil, err
		}
		if contextValue.Room.ID != roomID || contextValue.Room.RoomType != protocol.RoomTypeGroup {
			return nil, roomsvc.ErrConversationNotFound
		}
		return contextValue, nil
	}
	contexts, err := s.rooms.GetRoomContexts(ctx, roomID)
	if err != nil {
		return nil, err
	}
	contextValue, ok := mainContext(contexts)
	if !ok || contextValue.Room.RoomType != protocol.RoomTypeGroup {
		return nil, roomsvc.ErrConversationNotFound
	}
	return contextValue, nil
}

func (s *Service) authorize(
	ctx context.Context,
	actor Actor,
) (context.Context, *protocol.Agent, error) {
	if s == nil || s.agents == nil || s.rooms == nil || s.realtime == nil || s.runtime == nil {
		return nil, nil, errors.New("平台通讯服务未完整装配")
	}
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
	if actor.OwnerUserID == "" || actor.AgentID == "" ||
		actor.SessionKey == "" || actor.RoundID == "" ||
		actor.LeaseSessionKey == "" || actor.LeaseRoundID == "" {
		return nil, nil, errors.New("平台通讯调用缺少可信 runtime 身份")
	}
	if principal := authctx.PrincipalFromContext(ctx); principal != nil &&
		strings.TrimSpace(principal.UserID) != actor.OwnerUserID {
		return nil, nil, errors.New("平台通讯认证主体与 owner 作用域不匹配")
	}
	if !slices.Contains(s.runtime.GetRunningRoundIDs(actor.LeaseSessionKey), actor.LeaseRoundID) {
		return nil, nil, errors.New("平台通讯调用所属 round 已结束或不再可信")
	}
	scoped := authctx.WithPrincipal(ctx, &authctx.Principal{
		UserID: actor.OwnerUserID, Username: actor.AgentID,
		Role: authctx.RoleMember, AuthMethod: "nexus_communication_runtime",
	})
	current, err := s.agents.GetAgent(scoped, actor.AgentID)
	if err != nil {
		return nil, nil, err
	}
	if current.IsMain {
		return nil, nil, errors.New("主智能体只属于控制面，不能进入 Agent 通讯录")
	}
	if strings.TrimSpace(current.OwnerUserID) != actor.OwnerUserID {
		return nil, nil, errors.New("平台通讯 Agent 与 owner 作用域不匹配")
	}
	switch actor.ContextKind {
	case ContextKindAgent:
		if err = validateAgentActor(actor); err != nil {
			return nil, nil, err
		}
	case ContextKindRoom:
		if err = s.validateRoomActor(scoped, actor); err != nil {
			return nil, nil, err
		}
	default:
		return nil, nil, errors.New("平台通讯只接受 Agent 或 Room runtime 上下文")
	}
	return scoped, current, nil
}

func validateAgentActor(actor Actor) error {
	if actor.ContextID != actor.AgentID ||
		actor.SessionKey != actor.LeaseSessionKey ||
		actor.RoundID != actor.LeaseRoundID {
		return errors.New("平台通讯 Agent 上下文与 runtime lease 不匹配")
	}
	parsed := protocol.ParseSessionKey(actor.SessionKey)
	if !parsed.IsStructured || parsed.Kind != protocol.SessionKeyKindAgent ||
		strings.TrimSpace(parsed.AgentID) != actor.AgentID {
		return errors.New("平台通讯 session 不属于当前 Agent")
	}
	return nil
}

func (s *Service) validateRoomActor(ctx context.Context, actor Actor) error {
	roomID := actor.RoomID
	if roomID == "" {
		roomID = actor.ContextID
	}
	if roomID == "" || actor.ContextID != roomID {
		return errors.New("平台通讯 Room 上下文缺少固定 room_id")
	}
	parsed := protocol.ParseSessionKey(actor.SessionKey)
	if !parsed.IsStructured || parsed.Kind != protocol.SessionKeyKindRoom ||
		strings.TrimSpace(parsed.ConversationID) == "" ||
		strings.TrimSpace(parsed.ConversationID) != actor.ConversationID {
		return errors.New("平台通讯 Room session 与 conversation 不匹配")
	}
	authorization, err := s.rooms.GetRoomAuthorizationSnapshot(ctx, roomID, actor.AgentID)
	if err != nil {
		return fmt.Errorf("重新验证平台通讯 Room 成员身份: %w", err)
	}
	if authorization == nil || !authorization.AgentIsMember {
		return errors.New("当前 Agent 已不是该 Room 成员")
	}
	contextValue, err := s.rooms.GetConversationContext(ctx, actor.ConversationID)
	if err != nil {
		return fmt.Errorf("重新验证平台通讯 conversation: %w", err)
	}
	if contextValue.Room.ID != roomID {
		return errors.New("平台通讯 conversation 不属于当前 Room")
	}
	expectedLeaseSession := protocol.BuildRoomAgentSessionKey(
		actor.ConversationID, actor.AgentID, contextValue.Room.RoomType,
	)
	if actor.LeaseSessionKey != expectedLeaseSession {
		return errors.New("平台通讯 runtime lease 不属于当前 Room Agent slot")
	}
	return nil
}

func mainConversation(contexts []protocol.ConversationContextAggregate) (protocol.ConversationRecord, bool) {
	contextValue, ok := mainContext(contexts)
	if !ok {
		return protocol.ConversationRecord{}, false
	}
	return contextValue.Conversation, true
}

func mainContext(contexts []protocol.ConversationContextAggregate) (*protocol.ConversationContextAggregate, bool) {
	for index := range contexts {
		if contexts[index].Conversation.ConversationType == protocol.ConversationTypeMain {
			return &contexts[index], true
		}
	}
	return nil, false
}

func memberParticipationPaused(members []protocol.MemberRecord, agentID string) bool {
	for _, member := range members {
		if member.MemberType == protocol.MemberTypeAgent && member.MemberAgentID == agentID {
			return member.ParticipationPaused
		}
	}
	return false
}
