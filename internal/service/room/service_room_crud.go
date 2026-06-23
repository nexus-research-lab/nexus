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
		},
		Members: roomdomain.BuildMembers(roomID, ownerUserID, normalizedAgentIDs),
		Conversation: protocol.ConversationRecord{
			ID:               conversationID,
			RoomID:           roomID,
			ConversationType: roomdomain.PickMainConversationType(normalizedRoomType),
			Title:            conversationTitle,
		},
		Sessions: roomdomain.BuildSessions(conversationID, agentRefs),
	}

	return s.repository.CreateRoom(ctx, bundle)
}

// UpdateRoom 更新房间信息。
func (s *Service) UpdateRoom(ctx context.Context, roomID string, request protocol.UpdateRoomRequest) (*protocol.ConversationContextAggregate, error) {
	nameValue, hasName := roomdomain.NormalizeOptionalPatch(request.Name)
	descriptionValue, hasDescription := roomdomain.NormalizeOptionalPatch(request.Description)
	titleValue, hasTitle := roomdomain.NormalizeOptionalPatch(request.Title)

	var (
		namePtr        *string
		descriptionPtr *string
		titlePtr       *string
		avatarPtr      *string
	)
	if hasName {
		namePtr = &nameValue
	}
	if hasDescription {
		descriptionPtr = &descriptionValue
	}
	if hasTitle {
		if titleValue == "" {
			return nil, errors.New("对话标题不能为空")
		}
		titlePtr = &titleValue
	}
	if request.Avatar != nil {
		avatarValue := roomdomain.NormalizeOptionalText(*request.Avatar)
		avatarPtr = &avatarValue
	}
	var skillNamesPtr *[]string
	var existingRoom *protocol.RoomAggregate
	loadExistingRoom := func() (*protocol.RoomAggregate, error) {
		if existingRoom != nil {
			return existingRoom, nil
		}
		value, err := s.GetRoom(ctx, roomID)
		if err != nil {
			return nil, err
		}
		existingRoom = value
		return existingRoom, nil
	}
	if request.SkillNames != nil {
		existing, err := loadExistingRoom()
		if err != nil {
			return nil, err
		}
		if existing.Room.RoomType == protocol.RoomTypeDM && len(*request.SkillNames) > 0 {
			return nil, errors.New("DM room 不支持启用 room skill")
		}
		skillNames, err := s.normalizeRoomSkillNames(ctx, *request.SkillNames)
		if err != nil {
			return nil, err
		}
		skillNamesPtr = &skillNames
	}
	var hostAgentIDPtr *string
	var hostAutoReplyEnabledPtr *bool
	if request.HostAgentID != nil || request.HostAutoReplyEnabled != nil {
		existing, err := loadExistingRoom()
		if err != nil {
			return nil, err
		}
		hostAgentID, hostAutoReplyEnabled, err := s.normalizeRoomHostSettingsPatch(existing, request.HostAgentID, request.HostAutoReplyEnabled)
		if err != nil {
			return nil, err
		}
		hostAgentIDPtr = &hostAgentID
		hostAutoReplyEnabledPtr = &hostAutoReplyEnabled
	}
	var privateMessagesEnabledPtr *bool
	if request.PrivateMessagesEnabled != nil {
		existing, err := loadExistingRoom()
		if err != nil {
			return nil, err
		}
		if existing.Room.RoomType == protocol.RoomTypeDM && *request.PrivateMessagesEnabled {
			return nil, errors.New("DM room 不支持启用私聊消息")
		}
		privateMessagesEnabledValue := *request.PrivateMessagesEnabled
		privateMessagesEnabledPtr = &privateMessagesEnabledValue
	}

	contextValue, err := s.repository.UpdateRoom(
		ctx,
		authctx.OwnerUserID(ctx),
		strings.TrimSpace(roomID),
		roomrepo.UpdateRoomPatch{
			Name:                   namePtr,
			Description:            descriptionPtr,
			Title:                  titlePtr,
			Avatar:                 avatarPtr,
			SkillNames:             skillNamesPtr,
			HostAgentID:            hostAgentIDPtr,
			HostAutoReplyEnabled:   hostAutoReplyEnabledPtr,
			PrivateMessagesEnabled: privateMessagesEnabledPtr,
		},
	)
	if err != nil {
		return nil, err
	}
	if contextValue == nil {
		return nil, ErrRoomNotFound
	}
	return contextValue, nil
}

// DeleteRoom 删除房间。
func (s *Service) DeleteRoom(ctx context.Context, roomID string) error {
	roomContexts, err := s.GetRoomContexts(ctx, roomID)
	if err != nil {
		return err
	}
	deleted, err := s.repository.DeleteRoom(ctx, authctx.OwnerUserID(ctx), strings.TrimSpace(roomID))
	if err != nil {
		return err
	}
	if !deleted {
		return ErrRoomNotFound
	}
	runtimeErr := s.closeConversationRuntimeSessions(ctx, roomContexts, true, nil)
	artifactErr := s.cleanupConversationArtifacts(ctx, roomContexts, true, nil)
	goalErr := s.cleanupGoalsForRoomContexts(ctx, roomContexts)
	return errors.Join(runtimeErr, artifactErr, goalErr)
}
