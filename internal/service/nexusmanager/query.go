// INPUT: 已绑定 Actor、资源 ID、列表/内容上限。
// OUTPUT: 每次重校验后的脱敏 Agent/Room/conversation/session/workspace 视图。
// POS: nexus_manager 只读业务入口。
package nexusmanager

import (
	"context"
	"errors"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

const (
	defaultListLimit = 100
	maxListLimit     = 1000
	defaultReadBytes = 32 * 1024
	maxReadBytes     = 128 * 1024
)

// InspectCapabilities 返回当前 runtime 身份可见的实际能力边界。
func (s *Service) InspectCapabilities(ctx context.Context, actor Actor) (*CapabilitySnapshot, error) {
	resolved, err := s.resolveActor(ctx, actor)
	if err != nil {
		return nil, err
	}
	result := &CapabilitySnapshot{
		Authority: resolved.Authority, ContextKind: resolved.ContextKind,
		ContextID: resolved.ContextID,
		Excluded: []string{
			"auth_and_users", "public_providers", "channel_credentials",
			"connector_credentials", "automation", "destructive_delete",
			"raw_nexusctl", "arbitrary_workspace_write",
		},
	}
	switch resolved.Authority {
	case AuthorityOwnerMain:
		result.ReadOperations = []string{
			"agents", "rooms", "conversations", "sessions", "owned_agent_workspaces",
		}
	case AuthorityAgentSelf:
		result.ReadOperations = []string{"own_workspace"}
	case AuthorityRoomMember:
		result.ReadOperations = []string{"current_room", "current_conversation"}
	}
	return result, nil
}

// ListAgents 返回当前 owner 的脱敏 Agent 目录。
func (s *Service) ListAgents(ctx context.Context, actor Actor) ([]AgentView, error) {
	resolved, err := s.resolveActor(ctx, actor)
	if err != nil {
		return nil, err
	}
	if err = requireOwnerMain(resolved); err != nil {
		return nil, err
	}
	items, err := s.agents.ListAgents(resolved.Context)
	if err != nil {
		return nil, err
	}
	result := make([]AgentView, 0, len(items))
	for _, item := range items {
		result = append(result, agentView(item))
	}
	return result, nil
}

// GetAgent 返回当前 owner 下指定 Agent 的脱敏视图。
func (s *Service) GetAgent(ctx context.Context, actor Actor, agentID string) (*AgentView, error) {
	resolved, err := s.resolveActor(ctx, actor)
	if err != nil {
		return nil, err
	}
	if err = requireOwnerMain(resolved); err != nil {
		return nil, err
	}
	item, err := s.agents.GetAgent(resolved.Context, strings.TrimSpace(agentID))
	if err != nil {
		return nil, err
	}
	if item == nil || strings.TrimSpace(item.OwnerUserID) != resolved.OwnerUserID {
		return nil, errors.New("目标 Agent 不属于当前 owner")
	}
	result := agentView(*item)
	return &result, nil
}

// ListRooms 返回当前 owner 最近 Room 的脱敏目录。
func (s *Service) ListRooms(ctx context.Context, actor Actor, limit int) ([]RoomView, error) {
	resolved, err := s.resolveActor(ctx, actor)
	if err != nil {
		return nil, err
	}
	if err = requireOwnerMain(resolved); err != nil {
		return nil, err
	}
	if err = s.requireRooms(); err != nil {
		return nil, err
	}
	items, err := s.rooms.ListRooms(resolved.Context, boundedLimit(limit))
	if err != nil {
		return nil, err
	}
	result := make([]RoomView, 0, len(items))
	for _, item := range items {
		result = append(result, roomView(item))
	}
	return result, nil
}

// GetRoom 返回 owner 指定 Room，或 Room runtime 固定的当前 Room。
func (s *Service) GetRoom(ctx context.Context, actor Actor, requestedRoomID string) (*RoomView, error) {
	resolved, err := s.resolveActor(ctx, actor)
	if err != nil {
		return nil, err
	}
	roomID := strings.TrimSpace(requestedRoomID)
	switch resolved.Authority {
	case AuthorityOwnerMain:
		if roomID == "" {
			return nil, errors.New("主智能体读取 Room 时必须指定 room_id")
		}
	case AuthorityRoomMember:
		if roomID != "" && roomID != resolved.RoomID {
			return nil, errors.New("Room 成员只能读取当前 Room")
		}
		roomID = resolved.RoomID
	default:
		return nil, errors.New("普通 Agent 私有 DM 不能读取 Room 目录")
	}
	if err = s.requireRooms(); err != nil {
		return nil, err
	}
	item, err := s.rooms.GetRoom(resolved.Context, roomID)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, errors.New("目标 Room 不存在")
	}
	result := roomView(*item)
	return &result, nil
}

// ListRoomContexts 返回 owner 指定 Room 的 conversation 目录。
func (s *Service) ListRoomContexts(
	ctx context.Context,
	actor Actor,
	roomID string,
	limit int,
) ([]ConversationContextView, error) {
	resolved, err := s.resolveActor(ctx, actor)
	if err != nil {
		return nil, err
	}
	if err = requireOwnerMain(resolved); err != nil {
		return nil, err
	}
	if err = s.requireRooms(); err != nil {
		return nil, err
	}
	roomID = strings.TrimSpace(roomID)
	if roomID == "" {
		return nil, errors.New("读取 conversation 目录必须指定 room_id")
	}
	roomValue, err := s.rooms.GetRoom(resolved.Context, roomID)
	if err != nil {
		return nil, err
	}
	if roomValue == nil {
		return nil, errors.New("目标 Room 不存在")
	}
	items, err := s.rooms.GetRoomContexts(resolved.Context, roomID)
	if err != nil {
		return nil, err
	}
	limit = boundedLimit(limit)
	if len(items) > limit {
		items = items[:limit]
	}
	result := make([]ConversationContextView, 0, len(items))
	for _, item := range items {
		result = append(result, contextView(item))
	}
	return result, nil
}

// GetConversation 返回 owner 指定 conversation，或 Room runtime 固定的当前 conversation。
func (s *Service) GetConversation(
	ctx context.Context,
	actor Actor,
	conversationID string,
) (*ConversationContextView, error) {
	resolved, err := s.resolveActor(ctx, actor)
	if err != nil {
		return nil, err
	}
	conversationID = strings.TrimSpace(conversationID)
	switch resolved.Authority {
	case AuthorityOwnerMain:
		if conversationID == "" {
			return nil, errors.New("主智能体读取 conversation 时必须指定 conversation_id")
		}
	case AuthorityRoomMember:
		if conversationID != "" && conversationID != resolved.ConversationID {
			return nil, errors.New("Room 成员只能读取当前 conversation")
		}
		result := contextView(*resolved.RoomContext)
		return &result, nil
	default:
		return nil, errors.New("普通 Agent 私有 DM 不能读取 Room conversation")
	}
	if err = s.requireRooms(); err != nil {
		return nil, err
	}
	item, err := s.rooms.GetConversationContext(resolved.Context, conversationID)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, errors.New("目标 conversation 不存在")
	}
	result := contextView(*item)
	return &result, nil
}

// ListSessions 返回主智能体可见的 owner session 目录。
func (s *Service) ListSessions(
	ctx context.Context,
	actor Actor,
	agentID string,
	limit int,
) ([]SessionView, error) {
	resolved, err := s.resolveActor(ctx, actor)
	if err != nil {
		return nil, err
	}
	if err = requireOwnerMain(resolved); err != nil {
		return nil, err
	}
	if err = s.requireSessions(); err != nil {
		return nil, err
	}
	agentID = strings.TrimSpace(agentID)
	var items []protocol.Session
	if agentID == "" {
		items, err = s.sessions.ListSessions(resolved.Context)
	} else {
		target, getErr := s.agents.GetAgent(resolved.Context, agentID)
		if getErr != nil {
			return nil, getErr
		}
		if target == nil || strings.TrimSpace(target.OwnerUserID) != resolved.OwnerUserID {
			return nil, errors.New("目标 Agent 不属于当前 owner")
		}
		items, err = s.sessions.ListAgentSessions(resolved.Context, agentID)
	}
	if err != nil {
		return nil, err
	}
	limit = boundedLimit(limit)
	if len(items) > limit {
		items = items[:limit]
	}
	result := make([]SessionView, 0, len(items))
	for _, item := range items {
		result = append(result, sessionView(item))
	}
	return result, nil
}

// GetSession 返回主智能体指定的非 Room 统一 session 视图。
func (s *Service) GetSession(ctx context.Context, actor Actor, sessionKey string) (*SessionView, error) {
	resolved, err := s.resolveActor(ctx, actor)
	if err != nil {
		return nil, err
	}
	if err = requireOwnerMain(resolved); err != nil {
		return nil, err
	}
	if err = s.requireSessions(); err != nil {
		return nil, err
	}
	item, err := s.sessions.GetSession(resolved.Context, strings.TrimSpace(sessionKey))
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, errors.New("目标 Session 不存在")
	}
	result := sessionView(*item)
	return &result, nil
}

// ListWorkspaceFiles 返回主智能体指定 Agent 或普通 Agent 自身的有界文件目录。
func (s *Service) ListWorkspaceFiles(
	ctx context.Context,
	actor Actor,
	agentID string,
	limit int,
) (*WorkspaceListing, error) {
	resolved, err := s.resolveActor(ctx, actor)
	if err != nil {
		return nil, err
	}
	targetAgentID, err := s.workspaceAgent(resolved, agentID)
	if err != nil {
		return nil, err
	}
	if err = s.requireWorkspaces(); err != nil {
		return nil, err
	}
	items, err := s.workspaces.ListFiles(resolved.Context, targetAgentID)
	if err != nil {
		return nil, err
	}
	limit = boundedLimit(limit)
	truncated := len(items) > limit
	if truncated {
		items = items[:limit]
	}
	result := &WorkspaceListing{
		AgentID: targetAgentID, Items: make([]WorkspaceEntry, 0, len(items)),
		Truncated: truncated,
	}
	for _, item := range items {
		result.Items = append(result.Items, workspaceEntry(item))
	}
	return result, nil
}

// ReadWorkspaceFile 返回主智能体指定 Agent 或普通 Agent 自身的有界文件内容。
func (s *Service) ReadWorkspaceFile(
	ctx context.Context,
	actor Actor,
	agentID string,
	path string,
	maxBytes int,
) (*WorkspaceFileView, error) {
	resolved, err := s.resolveActor(ctx, actor)
	if err != nil {
		return nil, err
	}
	targetAgentID, err := s.workspaceAgent(resolved, agentID)
	if err != nil {
		return nil, err
	}
	if err = s.requireWorkspaces(); err != nil {
		return nil, err
	}
	item, err := s.workspaces.GetFile(resolved.Context, targetAgentID, strings.TrimSpace(path))
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, errors.New("目标 Workspace 文件不存在")
	}
	if maxBytes <= 0 {
		maxBytes = defaultReadBytes
	}
	if maxBytes > maxReadBytes {
		maxBytes = maxReadBytes
	}
	content, truncated := boundedUTF8(item.Content, maxBytes)
	return &WorkspaceFileView{
		AgentID: targetAgentID, Path: item.Path, Content: content,
		TotalBytes: len(item.Content), Truncated: truncated,
	}, nil
}

func boundedLimit(limit int) int {
	if limit <= 0 {
		return defaultListLimit
	}
	if limit > maxListLimit {
		return maxListLimit
	}
	return limit
}
