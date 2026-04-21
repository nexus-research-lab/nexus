// =====================================================
// @File   ：service.go
// @Date   ：2026/04/10 21:22:41
// @Author ：leemysw
// 2026/04/10 21:22:41   Create
// =====================================================

package room

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	agent2 "github.com/nexus-research-lab/nexus/internal/agent"
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

const localUserID = "local-user"

var (
	// ErrAgentNotFound 表示成员 Agent 不存在。
	ErrAgentNotFound = errors.New("agent not found")
	// ErrRoomNotFound 表示房间不存在。
	ErrRoomNotFound = errors.New("room not found")
	// ErrConversationNotFound 表示房间对话不存在。
	ErrConversationNotFound = errors.New("conversation not found")
	// ErrRoomMemberNotFound 表示房间成员不存在。
	ErrRoomMemberNotFound = errors.New("room member not found")
)

// Repository 定义 Room 存储接口。
type Repository interface {
	LoadAgentRuntimeRefs(context.Context, []string) ([]AgentRuntimeRef, error)
	ListRecentRooms(context.Context, int) ([]RoomAggregate, error)
	GetRoom(context.Context, string) (*RoomAggregate, error)
	GetRoomContexts(context.Context, string) ([]ConversationContextAggregate, error)
	GetConversationContext(context.Context, string) (*ConversationContextAggregate, error)
	FindDMRoomContext(context.Context, string) (*ConversationContextAggregate, error)
	CreateRoom(context.Context, CreateRoomBundle) (*ConversationContextAggregate, error)
	UpdateRoom(context.Context, string, *string, *string, *string, *string) (*ConversationContextAggregate, error)
	AddRoomMember(context.Context, string, AgentRuntimeRef) (*ConversationContextAggregate, error)
	RemoveRoomMember(context.Context, string, string) (*ConversationContextAggregate, error)
	DeleteRoom(context.Context, string) (bool, error)
	CreateConversation(context.Context, CreateConversationBundle) (*ConversationContextAggregate, error)
	UpdateConversation(context.Context, string, string, string) (*ConversationContextAggregate, error)
	DeleteConversation(context.Context, string, string) (*ConversationContextAggregate, error)
	UpdateSessionSDKSessionID(context.Context, string, string) error
}

// Service 提供 Room 编排能力。
type Service struct {
	config     config.Config
	agents     *agent2.Service
	repository Repository
	files      *workspacestore.SessionFileStore
	history    *workspacestore.AgentHistoryStore
}

// NewService 创建 Room 服务。
func NewService(cfg config.Config, agents *agent2.Service, repository Repository) *Service {
	return &Service{
		config:     cfg,
		agents:     agents,
		repository: repository,
		files:      workspacestore.NewSessionFileStore(cfg.WorkspacePath),
		history:    workspacestore.NewAgentHistoryStore(cfg.WorkspacePath),
	}
}

// BuildRoomSessionKey 复用统一协议层构造 room key。
func (s *Service) BuildRoomSessionKey(conversationID string) string {
	return "room:group:" + conversationID
}

// ListRooms 列出最近房间。
func (s *Service) ListRooms(ctx context.Context, limit int) ([]RoomAggregate, error) {
	if limit <= 0 {
		limit = 20
	}
	return s.repository.ListRecentRooms(ctx, limit)
}

// GetRoom 读取单个房间。
func (s *Service) GetRoom(ctx context.Context, roomID string) (*RoomAggregate, error) {
	roomValue, err := s.repository.GetRoom(ctx, strings.TrimSpace(roomID))
	if err != nil {
		return nil, err
	}
	if roomValue == nil {
		return nil, ErrRoomNotFound
	}
	return roomValue, nil
}

// GetRoomContexts 读取房间全部上下文。
func (s *Service) GetRoomContexts(ctx context.Context, roomID string) ([]ConversationContextAggregate, error) {
	contexts, err := s.repository.GetRoomContexts(ctx, strings.TrimSpace(roomID))
	if err != nil {
		return nil, err
	}
	if len(contexts) == 0 {
		return nil, ErrRoomNotFound
	}
	return contexts, nil
}

// GetConversationContext 按 conversation_id 读取单条房间上下文。
func (s *Service) GetConversationContext(ctx context.Context, conversationID string) (*ConversationContextAggregate, error) {
	contextValue, err := s.repository.GetConversationContext(ctx, strings.TrimSpace(conversationID))
	if err != nil {
		return nil, err
	}
	if contextValue == nil {
		return nil, ErrConversationNotFound
	}
	return contextValue, nil
}

// EnsureDirectRoom 获取或创建直聊房间。
func (s *Service) EnsureDirectRoom(ctx context.Context, agentID string) (*ConversationContextAggregate, error) {
	normalizedAgentID := strings.TrimSpace(agentID)
	if err := s.ensureDirectAgentID(normalizedAgentID); err != nil {
		return nil, err
	}
	if normalizedAgentID == s.config.DefaultAgentID {
		if err := s.agents.EnsureReady(ctx); err != nil {
			return nil, err
		}
	}

	existing, err := s.repository.FindDMRoomContext(ctx, normalizedAgentID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return existing, nil
	}

	return s.createRoom(ctx, CreateRoomRequest{
		AgentIDs: []string{normalizedAgentID},
	}, RoomTypeDM)
}

// CreateRoom 创建房间。
func (s *Service) CreateRoom(ctx context.Context, request CreateRoomRequest) (*ConversationContextAggregate, error) {
	return s.createRoom(ctx, request, RoomTypeGroup)
}

func (s *Service) createRoom(ctx context.Context, request CreateRoomRequest, roomType string) (*ConversationContextAggregate, error) {
	normalizedRoomType, err := s.normalizeRoomType(roomType)
	if err != nil {
		return nil, err
	}
	var normalizedAgentIDs []string
	// DM 与 group 的成员语义不同，不能共用“普通成员”归一化。
	// DM 允许主智能体，group 仍然禁止主智能体进入房间成员列表。
	switch normalizedRoomType {
	case RoomTypeDM:
		normalizedAgentIDs, err = s.normalizeDirectAgentIDs(request.AgentIDs)
	default:
		normalizedAgentIDs, err = s.normalizeGroupAgentIDs(request.AgentIDs)
	}
	if err != nil {
		return nil, err
	}
	agentRefs, err := s.loadAgentRefs(ctx, normalizedAgentIDs)
	if err != nil {
		return nil, err
	}
	roomID := newEntityID()
	roomName := normalizeOptionalText(request.Name)
	if roomName == "" {
		roomName = buildRoomName(agentRefs, normalizedRoomType)
	}
	conversationTitle := normalizeOptionalText(request.Title)
	if conversationTitle == "" {
		conversationTitle = roomName
	}

	conversationID := newEntityID()
	bundle := CreateRoomBundle{
		Room: RoomRecord{
			ID:          roomID,
			RoomType:    normalizedRoomType,
			Name:        roomName,
			Description: normalizeDescription(request.Description),
			Avatar:      normalizeOptionalText(request.Avatar),
		},
		Members: buildMembers(roomID, normalizedAgentIDs),
		Conversation: ConversationRecord{
			ID:               conversationID,
			RoomID:           roomID,
			ConversationType: pickMainConversationType(normalizedRoomType),
			Title:            conversationTitle,
		},
		Sessions: buildSessions(conversationID, agentRefs),
	}

	return s.repository.CreateRoom(ctx, bundle)
}

// UpdateRoom 更新房间信息。
func (s *Service) UpdateRoom(ctx context.Context, roomID string, request UpdateRoomRequest) (*ConversationContextAggregate, error) {
	nameValue, hasName := normalizeOptionalPatch(request.Name)
	descriptionValue, hasDescription := normalizeOptionalPatch(request.Description)
	titleValue, hasTitle := normalizeOptionalPatch(request.Title)

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
		avatarValue := normalizeOptionalText(*request.Avatar)
		avatarPtr = &avatarValue
	}

	if _, err := s.GetRoom(ctx, roomID); err != nil {
		return nil, err
	}

	contextValue, err := s.repository.UpdateRoom(ctx, strings.TrimSpace(roomID), namePtr, descriptionPtr, titlePtr, avatarPtr)
	if err != nil {
		return nil, err
	}
	if contextValue == nil {
		return nil, ErrRoomNotFound
	}
	return contextValue, nil
}

// AddRoomMember 向房间追加成员。
func (s *Service) AddRoomMember(ctx context.Context, roomID string, request AddRoomMemberRequest) (*ConversationContextAggregate, error) {
	normalizedAgentID := strings.TrimSpace(request.AgentID)
	if err := s.ensureGroupMemberAgentID(normalizedAgentID); err != nil {
		return nil, err
	}
	roomValue, err := s.GetRoom(ctx, roomID)
	if err != nil {
		return nil, err
	}
	if roomValue.Room.RoomType != RoomTypeGroup {
		return nil, errors.New("DM room does not support adding members")
	}
	for _, member := range roomValue.Members {
		if member.MemberType == MemberTypeAgent && member.MemberAgentID == normalizedAgentID {
			return nil, errors.New("Agent already exists in room")
		}
	}

	agentRefs, err := s.loadAgentRefs(ctx, []string{normalizedAgentID})
	if err != nil {
		return nil, err
	}
	contextValue, err := s.repository.AddRoomMember(ctx, strings.TrimSpace(roomID), agentRefs[0])
	if err != nil {
		return nil, err
	}
	if contextValue == nil {
		return nil, ErrRoomNotFound
	}
	return contextValue, nil
}

// RemoveRoomMember 从房间移除成员。
func (s *Service) RemoveRoomMember(ctx context.Context, roomID string, agentID string) (*ConversationContextAggregate, error) {
	normalizedAgentID := strings.TrimSpace(agentID)
	if err := s.ensureGroupMemberAgentID(normalizedAgentID); err != nil {
		return nil, err
	}

	roomContexts, err := s.GetRoomContexts(ctx, roomID)
	if err != nil {
		return nil, err
	}
	roomValue, err := s.GetRoom(ctx, roomID)
	if err != nil {
		return nil, err
	}
	if roomValue.Room.RoomType != RoomTypeGroup {
		return nil, errors.New("DM room does not support removing members")
	}
	agentCount := 0
	memberFound := false
	for _, member := range roomValue.Members {
		if member.MemberType == MemberTypeAgent && member.MemberAgentID != "" {
			agentCount++
		}
		if member.MemberType == MemberTypeAgent && member.MemberAgentID == normalizedAgentID {
			memberFound = true
		}
	}
	if !memberFound {
		return nil, ErrRoomMemberNotFound
	}
	if agentCount <= 1 {
		return nil, errors.New("Room 至少保留一个 agent 成员")
	}

	contextValue, err := s.repository.RemoveRoomMember(ctx, strings.TrimSpace(roomID), normalizedAgentID)
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
	deleted, err := s.repository.DeleteRoom(ctx, strings.TrimSpace(roomID))
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
func (s *Service) CreateConversation(ctx context.Context, roomID string, request CreateConversationRequest) (*ConversationContextAggregate, error) {
	roomValue, err := s.GetRoom(ctx, roomID)
	if err != nil {
		return nil, err
	}

	agentIDs := listAgentIDs(roomValue.Members)
	agentRefs, err := s.loadAgentRefs(ctx, agentIDs)
	if err != nil {
		return nil, err
	}

	contexts, err := s.repository.GetRoomContexts(ctx, roomValue.Room.ID)
	if err != nil {
		return nil, err
	}
	nextTitle := normalizeOptionalText(request.Title)
	if nextTitle == "" {
		nextTitle = buildNextConversationTitle(roomValue.Room.Name, contexts)
	}

	conversationID := newEntityID()
	contextValue, err := s.repository.CreateConversation(ctx, CreateConversationBundle{
		RoomID: roomValue.Room.ID,
		Conversation: ConversationRecord{
			ID:               conversationID,
			RoomID:           roomValue.Room.ID,
			ConversationType: ConversationTypeTopic,
			Title:            nextTitle,
		},
		Sessions: buildSessions(conversationID, agentRefs),
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
func (s *Service) UpdateConversation(ctx context.Context, roomID string, conversationID string, request UpdateConversationRequest) (*ConversationContextAggregate, error) {
	title := normalizeOptionalText(request.Title)
	if title == "" {
		return nil, errors.New("对话标题不能为空")
	}
	contexts, err := s.GetRoomContexts(ctx, roomID)
	if err != nil {
		return nil, err
	}
	if !hasConversation(contexts, conversationID) {
		return nil, ErrConversationNotFound
	}
	contextValue, err := s.repository.UpdateConversation(ctx, strings.TrimSpace(roomID), strings.TrimSpace(conversationID), title)
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
) (*ConversationContextAggregate, error) {
	return s.UpdateConversation(ctx, roomID, conversationID, UpdateConversationRequest{Title: title})
}

// DeleteConversation 删除 room 话题并返回回退上下文。
func (s *Service) DeleteConversation(ctx context.Context, roomID string, conversationID string) (*ConversationContextAggregate, error) {
	contexts, err := s.GetRoomContexts(ctx, roomID)
	if err != nil {
		return nil, err
	}
	if len(contexts) <= 1 {
		return nil, errors.New("room 至少保留一个对话")
	}
	target, ok := findConversation(contexts, conversationID)
	if !ok {
		return nil, ErrConversationNotFound
	}
	if target.ConversationType != ConversationTypeTopic {
		return nil, errors.New("主对话不支持删除")
	}
	targetContext, ok := findConversationContext(contexts, conversationID)
	if !ok {
		return nil, ErrConversationNotFound
	}
	contextValue, err := s.repository.DeleteConversation(ctx, strings.TrimSpace(roomID), strings.TrimSpace(conversationID))
	if err != nil {
		return nil, err
	}
	if contextValue == nil {
		return nil, ErrConversationNotFound
	}
	if err = s.cleanupConversationArtifacts(ctx, []ConversationContextAggregate{targetContext}, true, nil); err != nil {
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
	contexts []ConversationContextAggregate,
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
	return agent2.ResolveWorkspacePath(s.config, agentValue.Name), nil
}

func (s *Service) ensureDirectAgentID(agentID string) error {
	if agentID == "" {
		return errors.New("agent_id 不能为空")
	}
	return nil
}

func (s *Service) ensureGroupMemberAgentID(agentID string) error {
	if err := s.ensureDirectAgentID(agentID); err != nil {
		return err
	}
	if agentID == s.config.DefaultAgentID {
		return fmt.Errorf("主智能体（%s）不能作为 room 成员", s.config.DefaultAgentID)
	}
	return nil
}

func (s *Service) normalizeDirectAgentIDs(agentIDs []string) ([]string, error) {
	normalizedIDs := make([]string, 0, len(agentIDs))
	for _, agentID := range agentIDs {
		cleaned := strings.TrimSpace(agentID)
		if cleaned == "" {
			continue
		}
		if !containsString(normalizedIDs, cleaned) {
			normalizedIDs = append(normalizedIDs, cleaned)
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

func (s *Service) normalizeGroupAgentIDs(agentIDs []string) ([]string, error) {
	normalizedIDs := make([]string, 0, len(agentIDs))
	hasDefaultAgent := false
	for _, agentID := range agentIDs {
		cleaned := strings.TrimSpace(agentID)
		if cleaned == "" {
			continue
		}
		if cleaned == s.config.DefaultAgentID {
			hasDefaultAgent = true
			continue
		}
		if !containsString(normalizedIDs, cleaned) {
			normalizedIDs = append(normalizedIDs, cleaned)
		}
	}
	if hasDefaultAgent {
		return nil, fmt.Errorf("主智能体（%s）不能作为 room 成员", s.config.DefaultAgentID)
	}
	if len(normalizedIDs) == 0 {
		return nil, fmt.Errorf("room 至少需要一个普通成员 agent，主智能体（%s）不能作为 room 成员", s.config.DefaultAgentID)
	}
	return normalizedIDs, nil
}

func (s *Service) loadAgentRefs(ctx context.Context, agentIDs []string) ([]AgentRuntimeRef, error) {
	refs, err := s.repository.LoadAgentRuntimeRefs(ctx, agentIDs)
	if err != nil {
		return nil, err
	}
	refByID := make(map[string]AgentRuntimeRef, len(refs))
	for _, ref := range refs {
		refByID[ref.AgentID] = ref
	}

	result := make([]AgentRuntimeRef, 0, len(agentIDs))
	for _, agentID := range agentIDs {
		ref, ok := refByID[agentID]
		if !ok || ref.Status != "active" || strings.TrimSpace(ref.RuntimeID) == "" {
			return nil, fmt.Errorf("%w: %s", ErrAgentNotFound, agentID)
		}
		result = append(result, ref)
	}
	return result, nil
}

func (s *Service) normalizeRoomType(roomType string) (string, error) {
	normalized := strings.TrimSpace(strings.ToLower(roomType))
	if normalized == "" {
		normalized = RoomTypeGroup
	}
	switch normalized {
	case RoomTypeDM, RoomTypeGroup:
		return normalized, nil
	default:
		return "", errors.New("room_type 仅支持 room 或 dm")
	}
}

func buildMembers(roomID string, agentIDs []string) []MemberRecord {
	members := []MemberRecord{
		{
			ID:           newEntityID(),
			RoomID:       roomID,
			MemberType:   MemberTypeUser,
			MemberUserID: localUserID,
		},
	}
	for _, agentID := range agentIDs {
		members = append(members, MemberRecord{
			ID:            newEntityID(),
			RoomID:        roomID,
			MemberType:    MemberTypeAgent,
			MemberAgentID: agentID,
		})
	}
	return members
}

func buildSessions(conversationID string, refs []AgentRuntimeRef) []SessionRecord {
	sessions := make([]SessionRecord, 0, len(refs))
	for _, ref := range refs {
		sessions = append(sessions, SessionRecord{
			ID:             newEntityID(),
			ConversationID: conversationID,
			AgentID:        ref.AgentID,
			RuntimeID:      ref.RuntimeID,
			VersionNo:      1,
			BranchKey:      "main",
			IsPrimary:      true,
			Status:         "active",
		})
	}
	return sessions
}

func buildRoomName(refs []AgentRuntimeRef, roomType string) string {
	if len(refs) == 0 {
		return ""
	}
	if roomType == RoomTypeDM {
		return pickDisplayName(refs[0])
	}
	names := make([]string, 0, len(refs))
	for _, ref := range refs {
		names = append(names, pickDisplayName(ref))
	}
	return strings.Join(names, "、")
}

func buildNextConversationTitle(roomName string, contexts []ConversationContextAggregate) string {
	baseName := normalizeOptionalText(roomName)
	if baseName == "" {
		baseName = "未命名 room"
	}
	topicCount := 0
	for _, contextValue := range contexts {
		if contextValue.Conversation.ConversationType == ConversationTypeTopic {
			topicCount++
		}
	}
	return fmt.Sprintf("%s · 对话 %d", baseName, topicCount+1)
}

func pickMainConversationType(roomType string) string {
	if roomType == RoomTypeDM {
		return ConversationTypeDM
	}
	return ConversationTypeMain
}

func pickDisplayName(ref AgentRuntimeRef) string {
	if strings.TrimSpace(ref.DisplayName) != "" {
		return ref.DisplayName
	}
	return ref.Name
}

func listAgentIDs(members []MemberRecord) []string {
	agentIDs := make([]string, 0)
	for _, member := range members {
		if member.MemberType == MemberTypeAgent && member.MemberAgentID != "" {
			agentIDs = append(agentIDs, member.MemberAgentID)
		}
	}
	return agentIDs
}

func normalizeOptionalText(value string) string {
	return strings.TrimSpace(value)
}

func normalizeDescription(value string) string {
	return strings.TrimSpace(value)
}

func normalizeOptionalPatch(value string) (string, bool) {
	if value == "" {
		return "", false
	}
	return strings.TrimSpace(value), true
}

func containsString(items []string, value string) bool {
	for _, item := range items {
		if item == value {
			return true
		}
	}
	return false
}

func hasConversation(contexts []ConversationContextAggregate, conversationID string) bool {
	for _, contextValue := range contexts {
		if contextValue.Conversation.ID == conversationID {
			return true
		}
	}
	return false
}

func findConversation(contexts []ConversationContextAggregate, conversationID string) (ConversationRecord, bool) {
	for _, contextValue := range contexts {
		if contextValue.Conversation.ID == conversationID {
			return contextValue.Conversation, true
		}
	}
	return ConversationRecord{}, false
}

func findConversationContext(contexts []ConversationContextAggregate, conversationID string) (ConversationContextAggregate, bool) {
	for _, contextValue := range contexts {
		if contextValue.Conversation.ID == conversationID {
			return contextValue, true
		}
	}
	return ConversationContextAggregate{}, false
}

func newEntityID() string {
	buffer := make([]byte, 6)
	if _, err := rand.Read(buffer); err == nil {
		return hex.EncodeToString(buffer)
	}
	return fmt.Sprintf("%d", len(buffer))
}
