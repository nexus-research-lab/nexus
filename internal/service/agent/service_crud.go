package agent

import (
	"context"
	"errors"
	"os"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/storage/agentrepo"
)

// ListAgents 返回所有活跃 Agent。
func (s *Service) ListAgents(ctx context.Context) ([]protocol.Agent, error) {
	return s.listAgents(ctx, true)
}

// ListAgentRecords 返回所有活跃 Agent 的落库基础记录。
func (s *Service) ListAgentRecords(ctx context.Context) ([]protocol.Agent, error) {
	return s.listAgents(ctx, false)
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

// ValidateName 校验名称是否可用。
func (s *Service) ValidateName(ctx context.Context, name string, excludeAgentID string) (protocol.ValidateNameResponse, error) {
	if err := s.EnsureReady(ctx); err != nil {
		return protocol.ValidateNameResponse{}, err
	}
	return s.validateName(ctx, effectiveOwnerUserID(ctx), name, excludeAgentID)
}

func (s *Service) validateName(
	ctx context.Context,
	ownerUserID string,
	name string,
	excludeAgentID string,
) (protocol.ValidateNameResponse, error) {
	normalized := NormalizeName(name)
	response := protocol.ValidateNameResponse{
		Name:           name,
		NormalizedName: normalized,
	}

	if reason := ValidateName(name); reason != "" {
		response.Reason = reason
		return response, nil
	}

	response.IsValid = true

	exists, err := s.repository.ExistsActiveAgentName(ctx, ownerUserID, normalized, excludeAgentID)
	if err != nil {
		return response, err
	}
	if exists {
		response.Reason = "名称已存在，请更换一个名称"
		return response, nil
	}

	response.IsAvailable = true
	return response, nil
}

// CreateAgent 创建普通 Agent。
func (s *Service) CreateAgent(ctx context.Context, request protocol.CreateRequest) (*protocol.Agent, error) {
	if err := s.EnsureReady(ctx); err != nil {
		return nil, err
	}

	ownerUserID := effectiveOwnerUserID(ctx)
	validation, err := s.validateName(ctx, ownerUserID, request.Name, "")
	if err != nil {
		return nil, err
	}
	if !validation.IsValid || !validation.IsAvailable {
		return nil, errors.New(validation.Reason)
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
	return created, nil
}

// UpdateAgent 更新 Agent 配置。
func (s *Service) UpdateAgent(ctx context.Context, agentID string, request protocol.UpdateRequest) (*protocol.Agent, error) {
	if err := s.EnsureReady(ctx); err != nil {
		return nil, err
	}

	ownerUserID, _ := scopedOwnerUserID(ctx)
	existing, err := s.repository.GetAgent(ctx, strings.TrimSpace(agentID), ownerUserID)
	if err != nil {
		return nil, err
	}
	if existing == nil || existing.Status != "active" {
		return nil, ErrAgentNotFound
	}
	updateOwnerUserID := existing.OwnerUserID
	if ownerUserID != "" {
		updateOwnerUserID = ownerUserID
	}

	normalizedName := existing.Name
	if request.Name != nil {
		candidate := NormalizeName(*request.Name)
		if candidate != existing.Name {
			if existing.IsMain {
				return nil, errors.New("主智能体名称不可修改")
			}
			validation, validateErr := s.validateName(ctx, updateOwnerUserID, candidate, existing.AgentID)
			if validateErr != nil {
				return nil, validateErr
			}
			if !validation.IsValid || !validation.IsAvailable {
				return nil, errors.New(validation.Reason)
			}
			normalizedName = validation.NormalizedName
		}
	}

	nextOptions := existing.Options
	if request.Options != nil {
		nextOptions = mergeOptions(existing.Options, *request.Options)
	}

	avatar := existing.Avatar
	if request.Avatar != nil {
		avatar = strings.TrimSpace(*request.Avatar)
	}
	description := existing.Description
	if request.Description != nil {
		description = strings.TrimSpace(*request.Description)
	}
	vibeTags := existing.VibeTags
	if request.VibeTags != nil {
		vibeTags = append([]string(nil), request.VibeTags...)
	}

	updated, err := s.repository.UpdateAgent(ctx, agentrepo.UpdateRecord{
		AgentID:             existing.AgentID,
		OwnerUserID:         updateOwnerUserID,
		Slug:                BuildWorkspaceDirName(normalizedName),
		Name:                normalizedName,
		WorkspacePath:       existing.WorkspacePath,
		Avatar:              avatar,
		Description:         description,
		VibeTagsJSON:        mustJSONString(vibeTags, "[]"),
		Provider:            nextOptions.Provider,
		Model:               nextOptions.Model,
		PermissionMode:      nextOptions.PermissionMode,
		AllowedToolsJSON:    mustJSONString(nextOptions.AllowedTools, "[]"),
		DisallowedToolsJSON: mustJSONString(nextOptions.DisallowedTools, "[]"),
		MCPServersJSON:      mustJSONString(nextOptions.MCPServers, "{}"),
		MaxTurns:            nextOptions.MaxTurns,
		MaxThinkingTokens:   nextOptions.MaxThinkingTokens,
		SettingSourcesJSON:  mustJSONString(nextOptions.SettingSources, "[]"),
	})
	if err != nil {
		return nil, err
	}
	if updated == nil {
		return nil, ErrAgentNotFound
	}
	if err = os.MkdirAll(updated.WorkspacePath, 0o755); err != nil {
		return nil, err
	}
	if err = enrichAgentWithSkillsCount(updated); err != nil {
		return nil, err
	}
	return updated, nil
}

// DeleteAgent 软删除 Agent，并清理 workspace 目录。
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
	if s.history != nil {
		if _, err = s.history.DeleteTranscriptProject(existing.WorkspacePath); err != nil {
			return err
		}
	}
	if err = os.RemoveAll(existing.WorkspacePath); err != nil {
		return err
	}
	archiveOwnerUserID := existing.OwnerUserID
	if ownerUserID != "" {
		archiveOwnerUserID = ownerUserID
	}
	return s.repository.ArchiveAgent(ctx, existing.AgentID, archiveOwnerUserID)
}
