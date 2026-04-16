// # !/usr/bin/env go
// -*- coding: utf-8 -*-
// =====================================================
// @File   ：service.go
// @Date   ：2026/04/11 00:18:00
// @Author ：leemysw
// 2026/04/11 00:18:00   Create
// =====================================================

package session

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	agent2 "github.com/nexus-research-lab/nexus/internal/agent"
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

var (
	// ErrSessionNotFound 表示 session 不存在。
	ErrSessionNotFound = errors.New("session not found")
	// ErrSessionMutationUnsupported 表示该 session 只能通过更高层语义操作。
	ErrSessionMutationUnsupported = errors.New("session mutation is not supported")
)

// CreateRequest 表示创建会话请求。
type CreateRequest struct {
	SessionKey string `json:"session_key"`
	AgentID    string `json:"agent_id,omitempty"`
	Title      string `json:"title,omitempty"`
}

// UpdateRequest 表示更新会话请求。
type UpdateRequest struct {
	Title *string `json:"title,omitempty"`
}

// Service 负责编排文件会话与 Room SQL 会话视图。
type Service struct {
	config       config.Config
	agentService *agent2.Service
	repository   SQLRepository
	files        *workspacestore.SessionFileStore
	runtime      *runtimectx.Manager
}

// SetRuntimeManager 注入运行时管理器，用于历史读取时识别活跃轮次。
func (s *Service) SetRuntimeManager(runtimeManager *runtimectx.Manager) {
	s.runtime = runtimeManager
}

// NewService 使用已注入的依赖创建 Session 服务。
func NewService(cfg config.Config, agentService *agent2.Service, repository SQLRepository) *Service {
	return &Service{
		config:       cfg,
		agentService: agentService,
		repository:   repository,
		files:        workspacestore.NewSessionFileStore(cfg.WorkspacePath),
	}
}

// ListSessions 列出全部会话视图。
func (s *Service) ListSessions(ctx context.Context) ([]Session, error) {
	fileSessions, err := s.listWorkspaceSessions(ctx, "")
	if err != nil {
		return nil, err
	}
	roomSessions, err := s.repository.ListRoomSessions(ctx)
	if err != nil {
		return nil, err
	}
	return mergeSessions(fileSessions, roomSessions), nil
}

// ListAgentSessions 列出指定 Agent 的全部会话。
func (s *Service) ListAgentSessions(ctx context.Context, agentID string) ([]Session, error) {
	agentValue, err := s.agentService.GetAgent(ctx, agentID)
	if err != nil {
		return nil, err
	}

	fileSessions, err := s.files.ListSessions(agentValue.WorkspacePath)
	if err != nil {
		return nil, err
	}
	filteredFileSessions := make([]Session, 0, len(fileSessions))
	for _, item := range fileSessions {
		if item.AgentID != agentID {
			continue
		}
		if shouldHideWorkspaceSession(item) {
			continue
		}
		filteredFileSessions = append(filteredFileSessions, normalizeSession(item))
	}

	roomSessions, err := s.repository.ListRoomSessionsByAgent(ctx, agentID)
	if err != nil {
		return nil, err
	}
	return mergeSessions(filteredFileSessions, roomSessions), nil
}

// GetSession 读取指定 session。
func (s *Service) GetSession(ctx context.Context, rawSessionKey string) (*Session, error) {
	sessionKey, parsed, err := s.requireSessionKey(rawSessionKey)
	if err != nil {
		return nil, err
	}
	if parsed.Kind == protocol.SessionKeyKindRoom {
		return nil, ErrSessionNotFound
	}

	roomSession, err := s.repository.GetRoomSessionByKey(ctx, parsed)
	if err != nil {
		return nil, err
	}
	if roomSession != nil {
		return roomSession, nil
	}

	workspacePaths, err := s.resolveWorkspacePaths(ctx, parsed.AgentID)
	if err != nil {
		return nil, err
	}
	item, _, err := s.files.FindSession(workspacePaths, sessionKey)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, ErrSessionNotFound
	}
	normalized := normalizeSession(*item)
	return &normalized, nil
}

// CreateSession 创建或幂等返回普通 Agent 会话。
func (s *Service) CreateSession(ctx context.Context, request CreateRequest) (*Session, error) {
	sessionKey, parsed, err := s.requireSessionKey(request.SessionKey)
	if err != nil {
		return nil, err
	}
	if parsed.Kind != protocol.SessionKeyKindAgent {
		return nil, fmt.Errorf("%w: 共享 room session 不支持通过 Session API 创建", ErrSessionMutationUnsupported)
	}
	if request.AgentID != "" && request.AgentID != parsed.AgentID {
		return nil, errors.New("agent_id 与 session_key 不一致")
	}

	existing, err := s.GetSession(ctx, sessionKey)
	if err == nil && existing != nil {
		return existing, nil
	}
	if err != nil && !errors.Is(err, ErrSessionNotFound) {
		return nil, err
	}

	agentValue, err := s.agentService.GetAgent(ctx, parsed.AgentID)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	created, err := s.files.UpsertSession(agentValue.WorkspacePath, normalizeSession(Session{
		SessionKey:   sessionKey,
		AgentID:      parsed.AgentID,
		ChannelType:  protocol.NormalizeStoredChannelType(parsed.Channel),
		ChatType:     protocol.NormalizeSessionChatType(parsed.ChatType),
		Status:       "active",
		CreatedAt:    now,
		LastActivity: now,
		Title:        firstNonEmpty(strings.TrimSpace(request.Title), "New Chat"),
		MessageCount: 0,
		Options:      map[string]any{},
		IsActive:     true,
	}))
	if err != nil {
		return nil, err
	}
	return created, nil
}

// UpdateSession 更新普通 Agent 会话标题。
func (s *Service) UpdateSession(ctx context.Context, rawSessionKey string, request UpdateRequest) (*Session, error) {
	item, workspacePath, parsed, err := s.loadMutableWorkspaceSession(ctx, rawSessionKey)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, ErrSessionNotFound
	}
	if request.Title != nil {
		item.Title = firstNonEmpty(strings.TrimSpace(*request.Title), "New Chat")
	}
	if parsed.AgentID != "" {
		item.AgentID = parsed.AgentID
	}
	item.IsActive = item.Status == "active"
	return s.files.UpsertSession(workspacePath, *item)
}

// DeleteSession 删除普通 Agent 会话目录。
func (s *Service) DeleteSession(ctx context.Context, rawSessionKey string) error {
	sessionKey, _, err := s.requireSessionKey(rawSessionKey)
	if err != nil {
		return err
	}
	_, workspacePath, _, err := s.loadMutableWorkspaceSession(ctx, sessionKey)
	if err != nil {
		return err
	}
	if workspacePath == "" {
		return ErrSessionNotFound
	}
	deleted, err := s.files.DeleteSession(workspacePath, sessionKey)
	if err != nil {
		return err
	}
	if !deleted {
		return ErrSessionNotFound
	}
	return nil
}

// GetSessionMessages 读取 session 历史消息。
func (s *Service) GetSessionMessages(ctx context.Context, rawSessionKey string) ([]Message, error) {
	sessionKey, parsed, err := s.requireSessionKey(rawSessionKey)
	if err != nil {
		return nil, err
	}
	if parsed.Kind == protocol.SessionKeyKindRoom {
		logPath, err := s.repository.GetConversationLogPath(ctx, parsed.ConversationID)
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(logPath) == "" {
			logPath = s.files.RoomConversationMessagePath(parsed.ConversationID)
		}
		return s.files.ReadRoomMessagesWithActiveRounds(logPath, s.activeRoundIDs(sessionKey))
	}

	workspacePaths, err := s.resolveWorkspacePaths(ctx, parsed.AgentID)
	if err != nil {
		return nil, err
	}
	return s.files.ReadSessionMessagesWithActiveRounds(workspacePaths, sessionKey, s.activeRoundIDs(sessionKey))
}

// GetSessionCostSummary 读取或回算 session 成本汇总。
func (s *Service) GetSessionCostSummary(ctx context.Context, rawSessionKey string) (CostSummary, error) {
	sessionKey, parsed, err := s.requireSessionKey(rawSessionKey)
	if err != nil {
		return CostSummary{}, err
	}
	if parsed.Kind == protocol.SessionKeyKindRoom {
		return CostSummary{
			AgentID:    s.config.DefaultAgentID,
			SessionKey: sessionKey,
			SessionID:  "",
			UpdatedAt:  time.Now().UTC(),
		}, nil
	}

	workspacePaths, err := s.resolveWorkspacePaths(ctx, parsed.AgentID)
	if err != nil {
		return CostSummary{}, err
	}
	return s.files.ReadSessionCostSummary(workspacePaths, sessionKey, parsed.AgentID)
}

// GetAgentCostSummary 汇总指定 Agent 的成本视图。
func (s *Service) GetAgentCostSummary(ctx context.Context, agentID string) (AgentCostSummary, error) {
	sessions, err := s.ListAgentSessions(ctx, agentID)
	if err != nil {
		return AgentCostSummary{}, err
	}

	summary := AgentCostSummary{
		AgentID:   agentID,
		UpdatedAt: time.Now().UTC(),
	}
	for _, item := range sessions {
		cost, costErr := s.GetSessionCostSummary(ctx, item.SessionKey)
		if costErr != nil {
			return AgentCostSummary{}, costErr
		}
		if cost.TotalTokens == 0 && cost.TotalCostUSD == 0 && cost.CompletedRounds == 0 {
			continue
		}
		summary.TotalInputTokens += cost.TotalInputTokens
		summary.TotalOutputTokens += cost.TotalOutputTokens
		summary.TotalCacheCreationInputTokens += cost.TotalCacheCreationInputTokens
		summary.TotalCacheReadInputTokens += cost.TotalCacheReadInputTokens
		summary.TotalCostUSD += cost.TotalCostUSD
		summary.CompletedRounds += cost.CompletedRounds
		summary.ErrorRounds += cost.ErrorRounds
		summary.CostSessions++
	}
	summary.TotalTokens = summary.TotalInputTokens + summary.TotalOutputTokens
	return summary, nil
}

func (s *Service) loadMutableWorkspaceSession(ctx context.Context, rawSessionKey string) (*Session, string, protocol.SessionKey, error) {
	sessionKey, parsed, err := s.requireSessionKey(rawSessionKey)
	if err != nil {
		return nil, "", protocol.SessionKey{}, err
	}
	if parsed.Kind != protocol.SessionKeyKindAgent {
		return nil, "", parsed, fmt.Errorf("%w: 共享 room session 不支持通过 Session API 修改", ErrSessionMutationUnsupported)
	}

	roomSession, err := s.repository.GetRoomSessionByKey(ctx, parsed)
	if err != nil {
		return nil, "", parsed, err
	}
	if roomSession != nil {
		return nil, "", parsed, fmt.Errorf("%w: Room 成员会话必须通过 room/conversation 语义修改", ErrSessionMutationUnsupported)
	}

	workspacePaths, err := s.resolveWorkspacePaths(ctx, parsed.AgentID)
	if err != nil {
		return nil, "", parsed, err
	}
	item, workspacePath, err := s.files.FindSession(workspacePaths, sessionKey)
	if err != nil {
		return nil, "", parsed, err
	}
	return item, workspacePath, parsed, nil
}

func (s *Service) listWorkspaceSessions(ctx context.Context, agentID string) ([]Session, error) {
	workspacePaths, err := s.resolveWorkspacePaths(ctx, agentID)
	if err != nil {
		return nil, err
	}
	result := make([]Session, 0)
	for _, workspacePath := range workspacePaths {
		items, listErr := s.files.ListSessions(workspacePath)
		if listErr != nil {
			return nil, listErr
		}
		for _, item := range items {
			if shouldHideWorkspaceSession(item) {
				continue
			}
			result = append(result, normalizeSession(item))
		}
	}
	sort.Slice(result, func(i int, j int) bool {
		return result[i].LastActivity.After(result[j].LastActivity)
	})
	return result, nil
}

func (s *Service) listAgents(ctx context.Context, agentID string) ([]*agent2.Agent, error) {
	if strings.TrimSpace(agentID) != "" {
		agentValue, err := s.agentService.GetAgent(ctx, agentID)
		if err != nil {
			return nil, err
		}
		return []*agent2.Agent{agentValue}, nil
	}

	items, err := s.agentService.ListAgents(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]*agent2.Agent, 0, len(items))
	for index := range items {
		item := items[index]
		copyItem := item
		result = append(result, &copyItem)
	}
	return result, nil
}

func (s *Service) resolveWorkspacePaths(ctx context.Context, agentID string) ([]string, error) {
	agents, err := s.listAgents(ctx, agentID)
	if err != nil {
		return nil, err
	}
	result := make([]string, 0, len(agents))
	seen := make(map[string]struct{}, len(agents))
	for _, agentValue := range agents {
		workspacePath := strings.TrimSpace(agentValue.WorkspacePath)
		if workspacePath == "" {
			workspacePath = filepath.Join(agent2.WorkspaceBasePath(s.config), agentValue.AgentID)
		}
		if _, exists := seen[workspacePath]; exists {
			continue
		}
		seen[workspacePath] = struct{}{}
		result = append(result, workspacePath)
	}
	if strings.TrimSpace(agentID) == "" {
		workspaceBase := agent2.WorkspaceBasePath(s.config)
		entries, readErr := os.ReadDir(workspaceBase)
		if readErr == nil {
			for _, entry := range entries {
				if !entry.IsDir() {
					continue
				}
				workspacePath := filepath.Join(workspaceBase, entry.Name())
				if _, exists := seen[workspacePath]; exists {
					continue
				}
				seen[workspacePath] = struct{}{}
				result = append(result, workspacePath)
			}
		} else if !errors.Is(readErr, os.ErrNotExist) {
			return nil, readErr
		}
	}
	return result, nil
}

func (s *Service) requireSessionKey(raw string) (string, protocol.SessionKey, error) {
	sessionKey, err := protocol.RequireStructuredSessionKey(raw)
	if err != nil {
		return "", protocol.SessionKey{}, err
	}
	return sessionKey, protocol.ParseSessionKey(sessionKey), nil
}

func (s *Service) activeRoundIDs(sessionKey string) []string {
	if s.runtime == nil {
		return nil
	}
	return s.runtime.GetRunningRoundIDs(sessionKey)
}

func mergeSessions(fileSessions []Session, roomSessions []Session) []Session {
	merged := make(map[string]Session, len(fileSessions)+len(roomSessions))
	for _, item := range fileSessions {
		merged[item.SessionKey] = normalizeSession(item)
	}
	for _, item := range roomSessions {
		// Room SQL 视图必须覆盖文件侧同 key 残留，避免前端渲染重复会话。
		merged[item.SessionKey] = normalizeSession(item)
	}

	result := make([]Session, 0, len(merged))
	for _, item := range merged {
		result = append(result, item)
	}
	sort.Slice(result, func(i int, j int) bool {
		return result[i].LastActivity.After(result[j].LastActivity)
	})
	return result
}

func shouldHideWorkspaceSession(item Session) bool {
	if protocol.IsRoomSharedSessionKey(item.SessionKey) {
		return true
	}
	return item.RoomSessionID != nil && strings.TrimSpace(*item.RoomSessionID) != ""
}

func normalizeSession(item Session) Session {
	if item.Options == nil {
		item.Options = map[string]any{}
	}
	if item.Title == "" {
		item.Title = "New Chat"
	}
	if item.Status == "" {
		item.Status = "active"
	}
	if item.ChannelType == "" {
		item.ChannelType = "websocket"
	}
	if item.ChatType == "" {
		item.ChatType = "dm"
	}
	if item.LastActivity.IsZero() {
		item.LastActivity = item.CreatedAt
	}
	item.CreatedAt = item.CreatedAt.UTC()
	item.LastActivity = item.LastActivity.UTC()
	item.IsActive = item.Status == "active"
	return item
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
