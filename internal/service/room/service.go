package room

import (
	"context"
	"errors"
	"fmt"
	"strings"

	roomdomain "github.com/nexus-research-lab/nexus/internal/chat/room"
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
	"github.com/nexus-research-lab/nexus/internal/storage/roomrepo"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

var (
	// ErrRoomNotFound 表示房间不存在。
	ErrRoomNotFound = errors.New("room not found")
	// ErrConversationNotFound 表示房间对话不存在。
	ErrConversationNotFound = errors.New("conversation not found")
	// ErrRoomMemberNotFound 表示房间成员不存在。
	ErrRoomMemberNotFound = errors.New("room member not found")
)

// Repository 定义 Room 存储接口。
type Repository interface {
	LoadAgentRuntimeRefs(context.Context, string, []string) ([]roomrepo.AgentRuntimeRef, error)
	ListRecentRooms(context.Context, string, int) ([]protocol.RoomAggregate, error)
	GetRoom(context.Context, string, string) (*protocol.RoomAggregate, error)
	GetRoomContexts(context.Context, string, string) ([]protocol.ConversationContextAggregate, error)
	GetConversationContext(context.Context, string, string) (*protocol.ConversationContextAggregate, error)
	FindDMRoomContext(context.Context, string, string) (*protocol.ConversationContextAggregate, error)
	CreateRoom(context.Context, roomrepo.CreateRoomBundle) (*protocol.ConversationContextAggregate, error)
	UpdateRoom(context.Context, string, string, *string, *string, *string, *string) (*protocol.ConversationContextAggregate, error)
	AddRoomMember(context.Context, string, string, roomrepo.AgentRuntimeRef) (*protocol.ConversationContextAggregate, error)
	RemoveRoomMember(context.Context, string, string, string) (*protocol.ConversationContextAggregate, error)
	DeleteRoom(context.Context, string, string) (bool, error)
	CreateConversation(context.Context, roomrepo.CreateConversationBundle) (*protocol.ConversationContextAggregate, error)
	UpdateConversation(context.Context, string, string, string, string) (*protocol.ConversationContextAggregate, error)
	DeleteConversation(context.Context, string, string, string) (*protocol.ConversationContextAggregate, error)
	UpdateSessionSDKSessionID(context.Context, string, string) error
}

// Service 提供 Room 编排能力。
type Service struct {
	config     config.Config
	agents     *agentsvc.Service
	repository Repository
	files      *workspacestore.SessionFileStore
	history    *workspacestore.AgentHistoryStore
}

// NewService 创建 Room 服务。
func NewService(cfg config.Config, agents *agentsvc.Service, repository Repository) *Service {
	return &Service{
		config:     cfg,
		agents:     agents,
		repository: repository,
		files:      workspacestore.NewSessionFileStore(cfg.WorkspacePath),
		history:    workspacestore.NewAgentHistoryStore(cfg.WorkspacePath),
	}
}

// ListRooms 列出最近房间。
func (s *Service) ListRooms(ctx context.Context, limit int) ([]protocol.RoomAggregate, error) {
	if limit <= 0 {
		limit = 20
	}
	return s.repository.ListRecentRooms(ctx, authctx.OwnerUserID(ctx), limit)
}

// GetRoom 读取单个房间。
func (s *Service) GetRoom(ctx context.Context, roomID string) (*protocol.RoomAggregate, error) {
	roomValue, err := s.repository.GetRoom(ctx, authctx.OwnerUserID(ctx), strings.TrimSpace(roomID))
	if err != nil {
		return nil, err
	}
	if roomValue == nil {
		return nil, ErrRoomNotFound
	}
	return roomValue, nil
}

// GetRoomContexts 读取房间全部上下文。
func (s *Service) GetRoomContexts(ctx context.Context, roomID string) ([]protocol.ConversationContextAggregate, error) {
	contexts, err := s.repository.GetRoomContexts(ctx, authctx.OwnerUserID(ctx), strings.TrimSpace(roomID))
	if err != nil {
		return nil, err
	}
	if len(contexts) == 0 {
		return nil, ErrRoomNotFound
	}
	return contexts, nil
}

// GetConversationContext 按 conversation_id 读取单条房间上下文。
func (s *Service) GetConversationContext(ctx context.Context, conversationID string) (*protocol.ConversationContextAggregate, error) {
	contextValue, err := s.repository.GetConversationContext(ctx, authctx.OwnerUserID(ctx), strings.TrimSpace(conversationID))
	if err != nil {
		return nil, err
	}
	if contextValue == nil {
		return nil, ErrConversationNotFound
	}
	return contextValue, nil
}

// EnsureDirectRoom 获取或创建直聊房间。
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
	bundle := roomrepo.CreateRoomBundle{
		Room: protocol.RoomRecord{
			ID:          roomID,
			OwnerUserID: ownerUserID,
			RoomType:    normalizedRoomType,
			Name:        roomName,
			Description: roomdomain.NormalizeOptionalText(request.Description),
			Avatar:      roomdomain.NormalizeOptionalText(request.Avatar),
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

	contextValue, err := s.repository.UpdateRoom(
		ctx,
		authctx.OwnerUserID(ctx),
		strings.TrimSpace(roomID),
		namePtr,
		descriptionPtr,
		titlePtr,
		avatarPtr,
	)
	if err != nil {
		return nil, err
	}
	if contextValue == nil {
		return nil, ErrRoomNotFound
	}
	return contextValue, nil
}

// AddRoomMember 向房间追加成员。
func (s *Service) AddRoomMember(ctx context.Context, roomID string, request protocol.AddRoomMemberRequest) (*protocol.ConversationContextAggregate, error) {
	agentValue, err := s.ensureGroupMemberAgent(ctx, request.AgentID)
	if err != nil {
		return nil, err
	}
	normalizedAgentID := agentValue.AgentID

	agentRefs, err := s.loadAgentRefs(ctx, []string{normalizedAgentID})
	if err != nil {
		return nil, err
	}
	contextValue, err := s.repository.AddRoomMember(ctx, authctx.OwnerUserID(ctx), strings.TrimSpace(roomID), agentRefs[0])
	if err != nil {
		return nil, err
	}
	if contextValue == nil {
		return nil, ErrRoomNotFound
	}
	return contextValue, nil
}

// RemoveRoomMember 从房间移除成员。
func (s *Service) RemoveRoomMember(ctx context.Context, roomID string, agentID string) (*protocol.ConversationContextAggregate, error) {
	agentValue, err := s.ensureGroupMemberAgent(ctx, agentID)
	if err != nil {
		return nil, err
	}
	normalizedAgentID := agentValue.AgentID

	roomContexts, err := s.GetRoomContexts(ctx, roomID)
	if err != nil {
		return nil, err
	}
	roomValue := roomContexts[0].Room
	if roomValue.RoomType != protocol.RoomTypeGroup {
		return nil, errors.New("DM room does not support removing members")
	}
	agentCount := 0
	memberFound := false
	for _, member := range roomContexts[0].Members {
		if member.MemberType == protocol.MemberTypeAgent && member.MemberAgentID != "" {
			agentCount++
		}
		if member.MemberType == protocol.MemberTypeAgent && member.MemberAgentID == normalizedAgentID {
			memberFound = true
		}
	}
	if !memberFound {
		return nil, ErrRoomMemberNotFound
	}
	if agentCount <= 1 {
		return nil, errors.New("Room 至少保留一个 agent 成员")
	}

	contextValue, err := s.repository.RemoveRoomMember(ctx, authctx.OwnerUserID(ctx), strings.TrimSpace(roomID), normalizedAgentID)
	if err != nil {
		return nil, err
	}
	if contextValue == nil {
		return nil, ErrRoomNotFound
	}
	if err = s.cleanupConversationArtifacts(ctx, roomContexts, false, map[string]struct{}{normalizedAgentID: {}}); err != nil {
		return nil, err
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
	if err = s.cleanupConversationArtifacts(ctx, roomContexts, true, nil); err != nil {
		return err
	}
	return nil
}

// CreateConversation 创建 room 话题。
func (s *Service) CreateConversation(ctx context.Context, roomID string, request protocol.CreateConversationRequest) (*protocol.ConversationContextAggregate, error) {
	contexts, err := s.GetRoomContexts(ctx, roomID)
	if err != nil {
		return nil, err
	}
	roomValue := contexts[0].Room

	agentIDs := roomdomain.ListAgentIDs(contexts[0].Members)
	agentRefs, err := s.loadAgentRefs(ctx, agentIDs)
	if err != nil {
		return nil, err
	}

	nextTitle := roomdomain.NormalizeOptionalText(request.Title)
	if nextTitle == "" {
		nextTitle = roomdomain.BuildNextConversationTitle(roomValue.Name, contexts)
	}

	conversationID := roomdomain.NewEntityID()
	contextValue, err := s.repository.CreateConversation(ctx, roomrepo.CreateConversationBundle{
		RoomID: roomValue.ID,
		Conversation: protocol.ConversationRecord{
			ID:               conversationID,
			RoomID:           roomValue.ID,
			ConversationType: protocol.ConversationTypeTopic,
			Title:            nextTitle,
		},
		Sessions: roomdomain.BuildSessions(conversationID, agentRefs),
	})
	if err != nil {
		return nil, err
	}
	if contextValue == nil {
		return nil, ErrRoomNotFound
	}
	return contextValue, nil
}

// UpdateConversation 更新 room 话题标题。
func (s *Service) UpdateConversation(ctx context.Context, roomID string, conversationID string, request protocol.UpdateConversationRequest) (*protocol.ConversationContextAggregate, error) {
	title := roomdomain.NormalizeOptionalText(request.Title)
	if title == "" {
		return nil, errors.New("对话标题不能为空")
	}
	contexts, err := s.GetRoomContexts(ctx, roomID)
	if err != nil {
		return nil, err
	}
	if !roomdomain.HasConversation(contexts, conversationID) {
		return nil, ErrConversationNotFound
	}
	contextValue, err := s.repository.UpdateConversation(
		ctx,
		authctx.OwnerUserID(ctx),
		strings.TrimSpace(roomID),
		strings.TrimSpace(conversationID),
		title,
	)
	if err != nil {
		return nil, err
	}
	if contextValue == nil {
		return nil, ErrConversationNotFound
	}
	return contextValue, nil
}

// UpdateConversationTitle 以最小输入更新对话标题，供跨领域服务复用。
func (s *Service) UpdateConversationTitle(
	ctx context.Context,
	roomID string,
	conversationID string,
	title string,
) (*protocol.ConversationContextAggregate, error) {
	return s.UpdateConversation(ctx, roomID, conversationID, protocol.UpdateConversationRequest{Title: title})
}

// DeleteConversation 删除 room 话题并返回回退上下文。
func (s *Service) DeleteConversation(ctx context.Context, roomID string, conversationID string) (*protocol.ConversationContextAggregate, error) {
	contexts, err := s.GetRoomContexts(ctx, roomID)
	if err != nil {
		return nil, err
	}
	if len(contexts) <= 1 {
		return nil, errors.New("Room 至少保留一个对话")
	}
	target, ok := roomdomain.FindConversation(contexts, conversationID)
	if !ok {
		return nil, ErrConversationNotFound
	}
	if target.ConversationType != protocol.ConversationTypeTopic {
		return nil, errors.New("主对话不支持删除")
	}
	targetContext, ok := roomdomain.FindConversationContext(contexts, conversationID)
	if !ok {
		return nil, ErrConversationNotFound
	}
	contextValue, err := s.repository.DeleteConversation(
		ctx,
		authctx.OwnerUserID(ctx),
		strings.TrimSpace(roomID),
		strings.TrimSpace(conversationID),
	)
	if err != nil {
		return nil, err
	}
	if contextValue == nil {
		return nil, ErrConversationNotFound
	}
	if err = s.cleanupConversationArtifacts(ctx, []protocol.ConversationContextAggregate{targetContext}, true, nil); err != nil {
		return nil, err
	}
	return contextValue, nil
}

// UpdateSessionSDKSessionID 更新房间会话记录中的 Claude session_id。
func (s *Service) UpdateSessionSDKSessionID(ctx context.Context, sessionID string, sdkSessionID string) error {
	if strings.TrimSpace(sessionID) == "" || strings.TrimSpace(sdkSessionID) == "" {
		return nil
	}
	return s.repository.UpdateSessionSDKSessionID(ctx, strings.TrimSpace(sessionID), strings.TrimSpace(sdkSessionID))
}

func (s *Service) cleanupConversationArtifacts(
	ctx context.Context,
	contexts []protocol.ConversationContextAggregate,
	deleteSharedLog bool,
	agentFilter map[string]struct{},
) error {
	errs := make([]error, 0)
	workspaceByAgentID := make(map[string]string)
	for _, contextValue := range contexts {
		if deleteSharedLog {
			if _, err := s.files.DeleteRoomConversation(contextValue.Conversation.ID); err != nil {
				errs = append(errs, err)
			}
		}

		seenSessionKeys := make(map[string]struct{})
		for _, sessionValue := range contextValue.Sessions {
			if len(agentFilter) > 0 {
				if _, ok := agentFilter[sessionValue.AgentID]; !ok {
					continue
				}
			}

			sessionKey := protocol.BuildRoomAgentSessionKey(
				contextValue.Conversation.ID,
				sessionValue.AgentID,
				contextValue.Room.RoomType,
			)
			if _, exists := seenSessionKeys[sessionKey]; exists {
				continue
			}
			seenSessionKeys[sessionKey] = struct{}{}

			workspacePath := workspaceByAgentID[sessionValue.AgentID]
			if workspacePath == "" {
				resolvedPath, err := s.resolveAgentWorkspacePath(ctx, sessionValue.AgentID)
				if err != nil {
					errs = append(errs, err)
					continue
				}
				workspacePath = resolvedPath
				workspaceByAgentID[sessionValue.AgentID] = workspacePath
			}

			if _, err := s.files.DeleteSession(workspacePath, sessionKey); err != nil {
				errs = append(errs, err)
			}
			if s.history != nil && strings.TrimSpace(sessionValue.SDKSessionID) != "" {
				if _, err := s.history.DeleteTranscriptSession(workspacePath, sessionValue.SDKSessionID); err != nil {
					errs = append(errs, err)
				}
			}
		}
	}
	return errors.Join(errs...)
}

func (s *Service) resolveAgentWorkspacePath(ctx context.Context, agentID string) (string, error) {
	agentValue, err := s.agents.GetAgent(ctx, agentID)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(agentValue.WorkspacePath) != "" {
		return strings.TrimSpace(agentValue.WorkspacePath), nil
	}
	return agentsvc.ResolveWorkspacePath(s.config, agentValue.OwnerUserID, agentValue.Name), nil
}

func (s *Service) normalizeDirectAgentIDs(ctx context.Context, agentIDs []string) ([]string, error) {
	normalizedIDs := make([]string, 0, len(agentIDs))
	for _, agentID := range agentIDs {
		agentValue, err := s.resolveRoomAgent(ctx, agentID)
		if err != nil {
			return nil, err
		}
		if !roomdomain.ContainsString(normalizedIDs, agentValue.AgentID) {
			normalizedIDs = append(normalizedIDs, agentValue.AgentID)
		}
	}
	if len(normalizedIDs) == 0 {
		return nil, errors.New("DM room 需要一个 agent 成员")
	}
	if len(normalizedIDs) > 1 {
		return nil, errors.New("DM room 仅支持一个 agent 成员")
	}
	return normalizedIDs, nil
}

func (s *Service) normalizeGroupAgentIDs(ctx context.Context, agentIDs []string) ([]string, error) {
	normalizedIDs := make([]string, 0, len(agentIDs))
	for _, agentID := range agentIDs {
		agentValue, err := s.ensureGroupMemberAgent(ctx, agentID)
		if err != nil {
			return nil, err
		}
		if !roomdomain.ContainsString(normalizedIDs, agentValue.AgentID) {
			normalizedIDs = append(normalizedIDs, agentValue.AgentID)
		}
	}
	if len(normalizedIDs) == 0 {
		return nil, errors.New("room 至少需要一个普通成员 agent，主智能体不能作为 room 成员")
	}
	return normalizedIDs, nil
}

func (s *Service) loadAgentRefs(ctx context.Context, agentIDs []string) ([]roomrepo.AgentRuntimeRef, error) {
	refs, err := s.repository.LoadAgentRuntimeRefs(ctx, authctx.OwnerUserID(ctx), agentIDs)
	if err != nil {
		return nil, err
	}
	refByID := make(map[string]roomrepo.AgentRuntimeRef, len(refs))
	for _, ref := range refs {
		refByID[ref.AgentID] = ref
	}

	result := make([]roomrepo.AgentRuntimeRef, 0, len(agentIDs))
	for _, agentID := range agentIDs {
		ref, ok := refByID[agentID]
		if !ok || ref.Status != "active" || strings.TrimSpace(ref.RuntimeID) == "" {
			return nil, fmt.Errorf("%w: %s", agentsvc.ErrAgentNotFound, agentID)
		}
		result = append(result, ref)
	}
	return result, nil
}

func (s *Service) resolveRoomAgent(ctx context.Context, agentID string) (*protocol.Agent, error) {
	cleaned := strings.TrimSpace(agentID)
	if cleaned == "" {
		return nil, errors.New("agent_id 不能为空")
	}
	if cleaned == strings.TrimSpace(s.config.DefaultAgentID) {
		agentValue, err := s.agents.GetDefaultAgent(ctx)
		if err == nil {
			return agentValue, nil
		}
		if !errors.Is(err, agentsvc.ErrAgentNotFound) {
			return nil, err
		}
	}
	return s.agents.GetAgent(ctx, cleaned)
}

func (s *Service) ensureGroupMemberAgent(ctx context.Context, agentID string) (*protocol.Agent, error) {
	agentValue, err := s.resolveRoomAgent(ctx, agentID)
	if err != nil {
		return nil, err
	}
	if agentValue.IsMain {
		return nil, fmt.Errorf("主智能体（%s）不能作为 room 成员", agentValue.Name)
	}
	return agentValue, nil
}

func (s *Service) normalizeRoomType(roomType string) (string, error) {
	normalized := strings.TrimSpace(strings.ToLower(roomType))
	if normalized == "" {
		normalized = protocol.RoomTypeGroup
	}
	switch normalized {
	case protocol.RoomTypeDM, protocol.RoomTypeGroup:
		return normalized, nil
	default:
		return "", errors.New("room_type 仅支持 room 或 dm")
	}
}
