// INPUT: owner-scoped Room 创建请求、资料 patch 与内部联系人通道用途。
// OUTPUT: 校验成员/配置后持久化的 Room 与主 conversation 聚合。
// POS: Room 创建和资料变更业务主链；内部用途不能由 HTTP JSON 伪造。
package room

import (
	"context"
	"errors"
	"strings"

	roomdomain "github.com/nexus-research-lab/nexus/internal/chat/room"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/storage/roomrepo"
)

// EnsureDirectRoom 获取或创建直聊房间，并返回最近活跃的对话上下文。
func (s *Service) EnsureDirectRoom(ctx context.Context, agentID string) (*protocol.ConversationContextAggregate, error) {
	agentValue, err := s.resolveRoomAgent(ctx, agentID)
	if err != nil {
		return nil, err
	}
	normalizedAgentID := agentValue.AgentID
	existing, err := s.repository.FindDMRoomContext(ctx, authctx.OwnerUserID(ctx), normalizedAgentID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return existing, nil
	}

	return s.createRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs: []string{normalizedAgentID},
	}, protocol.RoomTypeDM)
}

// CreateRoom 创建房间。
func (s *Service) CreateRoom(ctx context.Context, request protocol.CreateRoomRequest) (*protocol.ConversationContextAggregate, error) {
	return s.createRoom(ctx, request, protocol.RoomTypeGroup)
}

func (s *Service) createRoom(ctx context.Context, request protocol.CreateRoomRequest, roomType string) (*protocol.ConversationContextAggregate, error) {
	ownerUserID := authctx.OwnerUserID(ctx)
	normalizedRoomType, err := s.normalizeRoomType(roomType)
	if err != nil {
		return nil, err
	}
	var normalizedAgentIDs []string
	// DM 与 group 的成员语义不同，不能共用“普通成员”归一化。
	// DM 允许主智能体，group 仍然禁止主智能体进入房间成员列表。
	switch normalizedRoomType {
	case protocol.RoomTypeDM:
		normalizedAgentIDs, err = s.normalizeDirectAgentIDs(ctx, request.AgentIDs)
	default:
		normalizedAgentIDs, err = s.normalizeGroupAgentIDs(ctx, request.AgentIDs)
	}
	if err != nil {
		return nil, err
	}
	agentRefs, err := s.loadAgentRefs(ctx, normalizedAgentIDs)
	if err != nil {
		return nil, err
	}
	roomID := roomdomain.NewEntityID()
	roomName := roomdomain.NormalizeOptionalText(request.Name)
	if roomName == "" {
		roomName = roomdomain.BuildRoomName(agentRefs, normalizedRoomType)
	}
	conversationTitle := roomdomain.NormalizeOptionalText(request.Title)
	if conversationTitle == "" {
		conversationTitle = roomName
	}

	conversationID := roomdomain.NewEntityID()
	skillNames, err := s.normalizeRoomSkillNames(ctx, request.SkillNames)
	if err != nil {
		return nil, err
	}
	if normalizedRoomType == protocol.RoomTypeDM && len(skillNames) > 0 {
		return nil, errors.New("DM room 不支持启用 room skill")
	}
	hostAgentID, hostAutoReplyEnabled, err := s.normalizeRoomHostSettings(normalizedRoomType, normalizedAgentIDs, request.HostAgentID, request.HostAutoReplyEnabled)
	if err != nil {
		return nil, err
	}
	bundle := roomrepo.CreateRoomBundle{
		Room: protocol.RoomRecord{
			ID:                     roomID,
			OwnerUserID:            ownerUserID,
			RoomType:               normalizedRoomType,
			Name:                   roomName,
			Description:            roomdomain.NormalizeOptionalText(request.Description),
			Avatar:                 roomdomain.NormalizeOptionalText(request.Avatar),
			SkillNames:             skillNames,
			HostAgentID:            hostAgentID,
			HostAutoReplyEnabled:   hostAutoReplyEnabled,
			PrivateMessagesEnabled: normalizedRoomType == protocol.RoomTypeGroup && request.PrivateMessagesEnabled,
			IsContactChannel:       normalizedRoomType == protocol.RoomTypeGroup && request.IsContactChannel,
		},
		Members: roomdomain.BuildMembers(roomID, ownerUserID, normalizedAgentIDs),
		Conversation: protocol.ConversationRecord{
			ID:               conversationID,
			RoomID:           roomID,
			ConversationType: roomdomain.PickMainConversationType(normalizedRoomType),
			Title:            conversationTitle,
			IsDraft:          true,
		},
		Sessions: roomdomain.BuildSessions(conversationID, agentRefs),
	}

	return s.repository.CreateRoom(ctx, bundle)
}

// UpdateRoom 更新房间信息。
func (s *Service) UpdateRoom(ctx context.Context, roomID string, request protocol.UpdateRoomRequest) (*protocol.ConversationContextAggregate, error) {
	builder := roomUpdateBuilder{service: s, ctx: ctx, roomID: roomID, request: request}
	patch, err := builder.build()
	if err != nil {
		return nil, err
	}
	contextValue, err := s.repository.UpdateRoom(
		ctx,
		authctx.OwnerUserID(ctx),
		strings.TrimSpace(roomID),
		patch,
	)
	if err != nil {
		return nil, err
	}
	if contextValue == nil {
		return nil, ErrRoomNotFound
	}
	return contextValue, nil
}

type roomUpdateBuilder struct {
	service  *Service
	ctx      context.Context
	roomID   string
	request  protocol.UpdateRoomRequest
	existing *protocol.RoomAggregate
	patch    roomrepo.UpdateRoomPatch
}

func (b *roomUpdateBuilder) build() (roomrepo.UpdateRoomPatch, error) {
	b.patch.ExpectedConfigurationVersion = b.request.ExpectedConfigurationVersion
	stages := []func() error{
		b.applyTextFields,
		b.applySkillNames,
		b.applyHostSettings,
		b.applyPrivateMessages,
	}
	for _, stage := range stages {
		if err := stage(); err != nil {
			return roomrepo.UpdateRoomPatch{}, err
		}
	}
	return b.patch, nil
}

func (b *roomUpdateBuilder) applyTextFields() error {
	if b.request.Name != nil {
		name := roomdomain.NormalizeOptionalText(*b.request.Name)
		if name == "" {
			return errors.New("Room 名称不能为空")
		}
		b.patch.Name = &name
	}
	if b.request.Description != nil {
		description := roomdomain.NormalizeOptionalText(*b.request.Description)
		b.patch.Description = &description
	}
	if b.request.Title != nil {
		title := roomdomain.NormalizeOptionalText(*b.request.Title)
		if title == "" {
			return errors.New("对话标题不能为空")
		}
		b.patch.Title = &title
	}
	if b.request.Avatar != nil {
		avatar := roomdomain.NormalizeOptionalText(*b.request.Avatar)
		b.patch.Avatar = &avatar
	}
	return nil
}

func (b *roomUpdateBuilder) applySkillNames() error {
	if b.request.SkillNames == nil {
		return nil
	}
	existing, err := b.loadExisting()
	if err != nil {
		return err
	}
	if existing.Room.RoomType == protocol.RoomTypeDM && len(*b.request.SkillNames) > 0 {
		return errors.New("DM room 不支持启用 room skill")
	}
	skillNames, err := b.service.normalizeRoomSkillNames(b.ctx, *b.request.SkillNames)
	if err != nil {
		return err
	}
	b.patch.SkillNames = &skillNames
	return nil
}

func (b *roomUpdateBuilder) applyHostSettings() error {
	if b.request.HostAgentID == nil && b.request.HostAutoReplyEnabled == nil {
		return nil
	}
	existing, err := b.loadExisting()
	if err != nil {
		return err
	}
	hostAgentID, hostAutoReplyEnabled, err := b.service.normalizeRoomHostSettingsPatch(
		existing,
		b.request.HostAgentID,
		b.request.HostAutoReplyEnabled,
	)
	if err != nil {
		return err
	}
	b.patch.HostAgentID = &hostAgentID
	b.patch.HostAutoReplyEnabled = &hostAutoReplyEnabled
	return nil
}

func (b *roomUpdateBuilder) applyPrivateMessages() error {
	if b.request.PrivateMessagesEnabled == nil {
		return nil
	}
	existing, err := b.loadExisting()
	if err != nil {
		return err
	}
	if existing.Room.RoomType == protocol.RoomTypeDM && *b.request.PrivateMessagesEnabled {
		return errors.New("DM room 不支持启用私聊消息")
	}
	value := *b.request.PrivateMessagesEnabled
	b.patch.PrivateMessagesEnabled = &value
	return nil
}

func (b *roomUpdateBuilder) loadExisting() (*protocol.RoomAggregate, error) {
	if b.existing != nil {
		return b.existing, nil
	}
	existing, err := b.service.GetRoom(b.ctx, b.roomID)
	if err != nil {
		return nil, err
	}
	b.existing = existing
	return existing, nil
}
