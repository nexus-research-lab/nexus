// =====================================================
// @File   ：service_crud.go
// @Date   ：2026/04/16 13:44:49
// @Author ：leemysw
// 2026/04/16 13:44:49   Create
// =====================================================

package agent

import (
	"context"
	"errors"
	"os"
	"strings"
)

// ListAgents 返回所有活跃 Agent。
func (s *Service) ListAgents(ctx context.Context) ([]Agent, error) {
	if err := s.EnsureReady(ctx); err != nil {
		return nil, err
	}
	ownerUserID, _ := scopedOwnerUserID(ctx)
	agents, err := s.repository.ListActiveAgents(ctx, ownerUserID)
	if err != nil {
		return nil, err
	}
	if err = enrichAgentsWithSkillsCount(agents); err != nil {
		return nil, err
	}
	return agents, nil
}

// GetAgent 获取指定 Agent。
func (s *Service) GetAgent(ctx context.Context, agentID string) (*Agent, error) {
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
	if err = enrichAgentWithSkillsCount(agent); err != nil {
		return nil, err
	}
	return agent, nil
}

// GetDefaultAgent 返回当前作用域下的主智能体。
func (s *Service) GetDefaultAgent(ctx context.Context) (*Agent, error) {
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
	if err = enrichAgentWithSkillsCount(agent); err != nil {
		return nil, err
	}
	return agent, nil
}

// ValidateName 校验名称是否可用。
func (s *Service) ValidateName(ctx context.Context, name string, excludeAgentID string) (ValidateNameResponse, error) {
	if err := s.EnsureReady(ctx); err != nil {
		return ValidateNameResponse{}, err
	}

	normalized := NormalizeName(name)
	ownerUserID := effectiveOwnerUserID(ctx)
	response := ValidateNameResponse{
		Name:           name,
		NormalizedName: normalized,
	}

	if reason := ValidateName(name); reason != "" {
		response.Reason = reason
		return response, nil
	}

	workspacePath := ResolveWorkspacePath(s.config, ownerUserID, normalized)
	response.WorkspacePath = workspacePath
	response.IsValid = true

	exists, err := s.repository.ExistsActiveAgentName(ctx, ownerUserID, normalized, excludeAgentID)
	if err != nil {
		return response, err
	}
	if exists {
		response.Reason = "名称已存在，请更换一个名称"
		return response, nil
	}

	if _, err = os.Stat(workspacePath); err == nil {
		response.Reason = "同名工作区目录已存在，请更换名称"
		return response, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return response, err
	}

	response.IsAvailable = true
	return response, nil
}

// CreateAgent 创建普通 Agent。
func (s *Service) CreateAgent(ctx context.Context, request CreateRequest) (*Agent, error) {
	if err := s.EnsureReady(ctx); err != nil {
		return nil, err
	}

	validation, err := s.ValidateName(ctx, request.Name, "")
	if err != nil {
		return nil, err
	}
	if !validation.IsValid || !validation.IsAvailable {
		return nil, errors.New(validation.Reason)
	}

	agentID := NewAgentID()
	record := BuildCreateRecord(
		s.config,
		request,
		effectiveOwnerUserID(ctx),
		validation.NormalizedName,
		agentID,
		validation.WorkspacePath,
		"active",
		false,
	)
	if err = os.MkdirAll(validation.WorkspacePath, 0o755); err != nil {
		return nil, err
	}
	return s.repository.CreateAgent(ctx, record)
}

// UpdateAgent 更新 Agent 配置。
func (s *Service) UpdateAgent(ctx context.Context, agentID string, request UpdateRequest) (*Agent, error) {
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
	workspacePath := existing.WorkspacePath
	if request.Name != nil {
		candidate := NormalizeName(*request.Name)
		if candidate != existing.Name {
			if existing.IsMain {
				return nil, errors.New("主智能体名称不可修改")
			}
			validation, validateErr := s.ValidateName(ctx, candidate, existing.AgentID)
			if validateErr != nil {
				return nil, validateErr
			}
			if !validation.IsValid || !validation.IsAvailable {
				return nil, errors.New(validation.Reason)
			}
			normalizedName = validation.NormalizedName
			workspacePath = validation.WorkspacePath
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

	if err = s.syncWorkspacePath(existing.WorkspacePath, workspacePath); err != nil {
		return nil, err
	}

	updated, err := s.repository.UpdateAgent(ctx, UpdateRecord{
		AgentID:             existing.AgentID,
		OwnerUserID:         updateOwnerUserID,
		Slug:                BuildWorkspaceDirName(normalizedName),
		Name:                normalizedName,
		WorkspacePath:       workspacePath,
		Avatar:              avatar,
		Description:         description,
		VibeTagsJSON:        mustJSONString(vibeTags, "[]"),
		Provider:            nextOptions.Provider,
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
