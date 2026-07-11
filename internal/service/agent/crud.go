package agent

import (
	"context"
	"errors"
	"os"
	"slices"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/storage/agentrepo"
)

type nameInvalidError struct {
	reason string
}

func (e nameInvalidError) Error() string {
	return e.reason
}

func (e nameInvalidError) Is(target error) bool {
	return target == ErrAgentNameInvalid
}

func fmtAgentNameInvalid(reason string) error {
	if strings.TrimSpace(reason) == "" {
		reason = "名称不合法"
	}
	return nameInvalidError{reason: reason}
}

// ListAgents 返回所有活跃 Agent。
func (s *Service) ListAgents(ctx context.Context) ([]protocol.Agent, error) {
	return s.listAgents(ctx, true)
}

// ListAgentRecords 返回所有活跃 Agent 的落库基础记录。
func (s *Service) ListAgentRecords(ctx context.Context) ([]protocol.Agent, error) {
	return s.listAgents(ctx, false)
}

// ListAllAgentRecordsForMaintenance 返回全局活跃 Agent 记录，仅供维护任务跨 owner 迁移使用。
func (s *Service) ListAllAgentRecordsForMaintenance(ctx context.Context) ([]protocol.Agent, error) {
	if err := s.EnsureReady(ctx); err != nil {
		return nil, err
	}
	return s.repository.ListActiveAgents(ctx, "")
}

func (s *Service) listAgents(ctx context.Context, includeSkillsCount bool) ([]protocol.Agent, error) {
	if err := s.EnsureReady(ctx); err != nil {
		return nil, err
	}
	ownerUserID, _ := scopedOwnerUserID(ctx)
	agents, err := s.repository.ListActiveAgents(ctx, ownerUserID)
	if err != nil {
		return nil, err
	}
	if err = ensureAgentRuntimeEmotionStates(agents); err != nil {
		return nil, err
	}
	if err = ensureAgentRuntimeSettings(agents); err != nil {
		return nil, err
	}
	if includeSkillsCount {
		err = enrichAgentsWithSkillsCount(agents)
	}
	if err != nil {
		return nil, err
	}
	return agents, nil
}

// GetAgent 获取指定 Agent。
func (s *Service) GetAgent(ctx context.Context, agentID string) (*protocol.Agent, error) {
	if err := s.EnsureReady(ctx); err != nil {
		return nil, err
	}
	ownerUserID, _ := scopedOwnerUserID(ctx)
	agent, err := s.repository.GetAgent(ctx, agentID, ownerUserID)
	if err != nil {
		return nil, err
	}
	if agent == nil || agent.Status != "active" {
		return nil, ErrAgentNotFound
	}
	if err = EnsureRuntimeEmotionState(agent.WorkspacePath); err != nil {
		return nil, err
	}
	if err = EnsureRuntimeSettingsProjection(*agent); err != nil {
		return nil, err
	}
	if err = enrichAgentWithSkillsCount(agent); err != nil {
		return nil, err
	}
	return agent, nil
}

// GetAgentsByIDs 批量获取指定 ID 列表的活跃 Agent。
func (s *Service) GetAgentsByIDs(ctx context.Context, agentIDs []string) ([]protocol.Agent, error) {
	if err := s.EnsureReady(ctx); err != nil {
		return nil, err
	}
	ownerUserID, _ := scopedOwnerUserID(ctx)
	agents, err := s.repository.ListAgentsByIDs(ctx, ownerUserID, agentIDs)
	if err != nil {
		return nil, err
	}
	if err = ensureAgentRuntimeEmotionStates(agents); err != nil {
		return nil, err
	}
	if err = ensureAgentRuntimeSettings(agents); err != nil {
		return nil, err
	}
	return agents, nil
}

// GetDefaultAgent 返回当前作用域下的主智能体。
func (s *Service) GetDefaultAgent(ctx context.Context) (*protocol.Agent, error) {
	if err := s.EnsureReady(ctx); err != nil {
		return nil, err
	}
	ownerUserID := effectiveOwnerUserID(ctx)
	agent, err := s.repository.GetMainAgent(ctx, ownerUserID)
	if err != nil {
		return nil, err
	}
	if agent == nil || agent.Status != "active" {
		return nil, ErrAgentNotFound
	}
	if err = EnsureRuntimeEmotionState(agent.WorkspacePath); err != nil {
		return nil, err
	}
	if err = EnsureRuntimeSettingsProjection(*agent); err != nil {
		return nil, err
	}
	if err = enrichAgentWithSkillsCount(agent); err != nil {
		return nil, err
	}
	return agent, nil
}

func ensureAgentRuntimeEmotionStates(agents []protocol.Agent) error {
	for _, agentValue := range agents {
		if err := EnsureRuntimeEmotionState(agentValue.WorkspacePath); err != nil {
			return err
		}
	}
	return nil
}

func ensureAgentRuntimeSettings(agents []protocol.Agent) error {
	for _, agentValue := range agents {
		if err := EnsureRuntimeSettingsProjection(agentValue); err != nil {
			return err
		}
	}
	return nil
}

// ValidateName 校验名称格式。
func (s *Service) ValidateName(ctx context.Context, name string, excludeAgentID string) (protocol.ValidateNameResponse, error) {
	if err := s.EnsureReady(ctx); err != nil {
		return protocol.ValidateNameResponse{}, err
	}
	return validateName(name), nil
}

func validateName(name string) protocol.ValidateNameResponse {
	normalized := NormalizeName(name)
	response := protocol.ValidateNameResponse{
		Name:           name,
		NormalizedName: normalized,
	}

	if reason := ValidateName(name); reason != "" {
		response.Reason = reason
		return response
	}

	response.IsValid = true
	response.IsAvailable = true
	return response
}

// CreateAgent 创建普通 Agent。
func (s *Service) CreateAgent(ctx context.Context, request protocol.CreateRequest) (*protocol.Agent, error) {
	if err := s.EnsureReady(ctx); err != nil {
		return nil, err
	}

	ownerUserID := effectiveOwnerUserID(ctx)
	validation := validateName(request.Name)
	if !validation.IsValid || !validation.IsAvailable {
		return nil, fmtAgentNameInvalid(validation.Reason)
	}

	agentID, workspacePath, err := s.createAgentWorkspacePath(ownerUserID)
	if err != nil {
		return nil, err
	}
	if err = EnsureRuntimeEmotionState(workspacePath); err != nil {
		_ = os.RemoveAll(workspacePath)
		return nil, err
	}
	record := BuildCreateRecord(
		s.config,
		request,
		ownerUserID,
		validation.NormalizedName,
		agentID,
		workspacePath,
		"active",
		false,
	)
	created, err := s.repository.CreateAgent(ctx, record)
	if err != nil {
		_ = os.RemoveAll(workspacePath)
		return nil, err
	}
	if err = EnsureRuntimeSettingsProjection(*created); err != nil {
		return nil, err
	}
	return created, nil
}

// UpdateAgent 更新 Agent 配置。
func (s *Service) UpdateAgent(ctx context.Context, agentID string, request protocol.UpdateRequest) (*protocol.Agent, error) {
	if err := s.EnsureReady(ctx); err != nil {
		return nil, err
	}
	update := agentUpdate{
		service: s,
		ctx:     ctx,
		agentID: strings.TrimSpace(agentID),
		request: request,
	}
	return update.run()
}

type agentUpdate struct {
	service     *Service
	ctx         context.Context
	agentID     string
	request     protocol.UpdateRequest
	existing    *protocol.Agent
	ownerUserID string
}

func (u *agentUpdate) run() (*protocol.Agent, error) {
	if err := u.load(); err != nil {
		return nil, err
	}
	record, err := u.record()
	if err != nil {
		return nil, err
	}
	updated, err := u.service.repository.UpdateAgent(u.ctx, record)
	if err != nil {
		return nil, err
	}
	if updated == nil {
		return nil, ErrAgentNotFound
	}
	if err = u.finalize(updated); err != nil {
		return nil, err
	}
	return updated, nil
}

func (u *agentUpdate) load() error {
	scopedOwnerID, _ := scopedOwnerUserID(u.ctx)
	existing, err := u.service.repository.GetAgent(u.ctx, u.agentID, scopedOwnerID)
	if err != nil {
		return err
	}
	if existing == nil || existing.Status != "active" {
		return ErrAgentNotFound
	}
	u.existing = existing
	u.ownerUserID = existing.OwnerUserID
	if scopedOwnerID != "" {
		u.ownerUserID = scopedOwnerID
	}
	return nil
}

func (u *agentUpdate) record() (agentrepo.UpdateRecord, error) {
	name, err := u.normalizedName()
	if err != nil {
		return agentrepo.UpdateRecord{}, err
	}
	options := u.updatedOptions()
	return agentrepo.UpdateRecord{
		AgentID:             u.existing.AgentID,
		OwnerUserID:         u.ownerUserID,
		Name:                name,
		WorkspacePath:       u.existing.WorkspacePath,
		Avatar:              updatedAgentText(u.existing.Avatar, u.request.Avatar),
		Description:         updatedAgentText(u.existing.Description, u.request.Description),
		VibeTagsJSON:        mustJSONString(u.updatedVibeTags()),
		Provider:            options.Provider,
		Model:               options.Model,
		PermissionMode:      options.PermissionMode,
		AllowedToolsJSON:    mustJSONString(options.AllowedTools),
		DisallowedToolsJSON: mustJSONString(options.DisallowedTools),
		MCPServersJSON:      mustJSONString(options.MCPServers),
		MaxTurns:            options.MaxTurns,
		MaxThinkingTokens:   options.MaxThinkingTokens,
		SettingSourcesJSON:  mustJSONString(options.SettingSources),
	}, nil
}

func (u *agentUpdate) normalizedName() (string, error) {
	if u.request.Name == nil {
		return u.existing.Name, nil
	}
	candidate := NormalizeName(*u.request.Name)
	if candidate == u.existing.Name {
		return u.existing.Name, nil
	}
	if u.existing.IsMain {
		return "", errors.New("主智能体名称不可修改")
	}
	validation := validateName(candidate)
	if !validation.IsValid || !validation.IsAvailable {
		return "", fmtAgentNameInvalid(validation.Reason)
	}
	return validation.NormalizedName, nil
}

func (u *agentUpdate) updatedOptions() protocol.Options {
	if u.request.Options == nil {
		return u.existing.Options
	}
	return mergeOptions(u.existing.Options, *u.request.Options)
}

func (u *agentUpdate) updatedVibeTags() []string {
	if u.request.VibeTags == nil {
		return u.existing.VibeTags
	}
	return slices.Clone(u.request.VibeTags)
}

func updatedAgentText(current string, requested *string) string {
	if requested == nil {
		return current
	}
	return strings.TrimSpace(*requested)
}

func (u *agentUpdate) finalize(updated *protocol.Agent) error {
	if err := os.MkdirAll(updated.WorkspacePath, 0o755); err != nil {
		return err
	}
	if err := EnsureRuntimeSettingsProjection(*updated); err != nil {
		return err
	}
	return enrichAgentWithSkillsCount(updated)
}

// DeleteAgent 删除 Agent，并清理 workspace 目录与数据库记录。
func (s *Service) DeleteAgent(ctx context.Context, agentID string) error {
	if err := s.EnsureReady(ctx); err != nil {
		return err
	}

	ownerUserID, _ := scopedOwnerUserID(ctx)
	existing, err := s.repository.GetAgent(ctx, strings.TrimSpace(agentID), ownerUserID)
	if err != nil {
		return err
	}
	if existing == nil || existing.Status != "active" {
		return ErrAgentNotFound
	}
	if existing.IsMain {
		return errors.New("主智能体不可删除")
	}
	if s.goals != nil {
		if _, err = s.goals.DeleteGoalsForAgent(ctx, existing.AgentID); err != nil {
			return err
		}
	}
	if s.history != nil {
		if _, err = s.history.DeleteTranscriptProject(existing.WorkspacePath); err != nil {
			return err
		}
	}
	if err = os.RemoveAll(existing.WorkspacePath); err != nil {
		return err
	}
	deleteOwnerUserID := existing.OwnerUserID
	if ownerUserID != "" {
		deleteOwnerUserID = ownerUserID
	}
	return s.repository.DeleteAgent(ctx, existing.AgentID, deleteOwnerUserID)
}
