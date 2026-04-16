// =====================================================
// @File   ：service.go
// @Date   ：2026/04/10 21:22:41
// @Author ：leemysw
// 2026/04/10 21:22:41   Create
// =====================================================

package agent

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/nexus-research-lab/nexus-core/internal/config"
	"github.com/nexus-research-lab/nexus-core/internal/storage"
	postgresrepo "github.com/nexus-research-lab/nexus-core/internal/storage/postgres"
	sqliterepo "github.com/nexus-research-lab/nexus-core/internal/storage/sqlite"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var (
	// ErrAgentNotFound 表示 Agent 不存在。
	ErrAgentNotFound = errors.New("agent not found")
)

// Service 提供 Agent 业务能力。
type Service struct {
	config     config.Config
	repository Repository
	once       sync.Once
	readyErr   error
}

// NewService 创建 Agent 服务。
func NewService(cfg config.Config) (*Service, error) {
	db, err := storage.OpenDB(cfg)
	if err != nil {
		return nil, err
	}
	return NewServiceWithDB(cfg, db), nil
}

// NewServiceWithDB 使用共享 DB 创建 Agent 服务。
func NewServiceWithDB(cfg config.Config, db *sql.DB) *Service {
	var repository Repository
	switch strings.ToLower(cfg.DatabaseDriver) {
	case "postgres", "postgresql", "pg":
		repository = postgresrepo.NewAgentRepository(db)
	default:
		repository = sqliterepo.NewAgentRepository(db)
	}

	return &Service{
		config:     cfg,
		repository: repository,
	}
}

// EnsureReady 确保主智能体和 workspace 根目录存在。
func (s *Service) EnsureReady(ctx context.Context) error {
	s.once.Do(func() {
		s.readyErr = s.ensureReady(ctx)
	})
	return s.readyErr
}

// ListAgents 返回所有活跃 Agent。
func (s *Service) ListAgents(ctx context.Context) ([]Agent, error) {
	if err := s.EnsureReady(ctx); err != nil {
		return nil, err
	}
	agents, err := s.repository.ListActiveAgents(ctx)
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
	agent, err := s.repository.GetAgent(ctx, agentID)
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
	response := ValidateNameResponse{
		Name:           name,
		NormalizedName: normalized,
	}

	if reason := ValidateName(name); reason != "" {
		response.Reason = reason
		return response, nil
	}

	workspacePath := ResolveWorkspacePath(s.config, normalized)
	response.WorkspacePath = workspacePath
	response.IsValid = true

	exists, err := s.repository.ExistsActiveAgentName(ctx, normalized, excludeAgentID)
	if err != nil {
		return response, err
	}
	if exists {
		response.Reason = "名称已存在，请更换一个名称"
		return response, nil
	}

	if _, err := os.Stat(workspacePath); err == nil {
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
	record := BuildCreateRecord(s.config, request, validation.NormalizedName, agentID, validation.WorkspacePath, "active")
	if err := os.MkdirAll(validation.WorkspacePath, 0o755); err != nil {
		return nil, err
	}
	return s.repository.CreateAgent(ctx, record)
}

// UpdateAgent 更新 Agent 配置。
func (s *Service) UpdateAgent(ctx context.Context, agentID string, request UpdateRequest) (*Agent, error) {
	if err := s.EnsureReady(ctx); err != nil {
		return nil, err
	}

	existing, err := s.repository.GetAgent(ctx, strings.TrimSpace(agentID))
	if err != nil {
		return nil, err
	}
	if existing == nil || existing.Status != "active" {
		return nil, ErrAgentNotFound
	}

	normalizedName := existing.Name
	workspacePath := existing.WorkspacePath
	if request.Name != nil {
		candidate := NormalizeName(*request.Name)
		if candidate != existing.Name {
			if existing.AgentID == s.config.DefaultAgentID {
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
		Slug:                BuildWorkspaceDirName(normalizedName),
		Name:                normalizedName,
		WorkspacePath:       workspacePath,
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

	existing, err := s.repository.GetAgent(ctx, strings.TrimSpace(agentID))
	if err != nil {
		return err
	}
	if existing == nil || existing.Status != "active" {
		return ErrAgentNotFound
	}
	if existing.AgentID == s.config.DefaultAgentID {
		return errors.New("主智能体不可删除")
	}
	if err = os.RemoveAll(existing.WorkspacePath); err != nil {
		return err
	}
	return s.repository.ArchiveAgent(ctx, existing.AgentID)
}

func (s *Service) ensureReady(ctx context.Context) error {
	workspaceBase := WorkspaceBasePath(s.config)
	if err := os.MkdirAll(workspaceBase, 0o755); err != nil {
		return err
	}

	agent, err := s.repository.GetAgent(ctx, s.config.DefaultAgentID)
	if err != nil {
		return err
	}
	if agent != nil && agent.Status == "active" {
		return os.MkdirAll(agent.WorkspacePath, 0o755)
	}

	record := BuildDefaultMainAgentRecord(s.config)
	if err := os.MkdirAll(record.WorkspacePath, 0o755); err != nil {
		return err
	}

	created, err := s.repository.CreateAgent(ctx, record)
	if err == nil {
		return os.MkdirAll(created.WorkspacePath, 0o755)
	}
	if !strings.Contains(err.Error(), "UNIQUE") && !strings.Contains(strings.ToLower(err.Error()), "duplicate") {
		return err
	}

	agent, err = s.repository.GetAgent(ctx, s.config.DefaultAgentID)
	if err != nil {
		return err
	}
	if agent == nil {
		return fmt.Errorf("主智能体初始化失败: %s", s.config.DefaultAgentID)
	}
	return os.MkdirAll(agent.WorkspacePath, 0o755)
}

func enrichAgentsWithSkillsCount(agents []Agent) error {
	for index := range agents {
		if err := enrichAgentWithSkillsCount(&agents[index]); err != nil {
			return err
		}
	}
	return nil
}

func enrichAgentWithSkillsCount(agent *Agent) error {
	if agent == nil {
		return nil
	}
	count, err := countDeployedSkills(agent.WorkspacePath)
	if err != nil {
		return err
	}
	agent.SkillsCount = count
	return nil
}

func countDeployedSkills(workspacePath string) (int, error) {
	skillRoot := filepath.Join(strings.TrimSpace(workspacePath), ".agents", "skills")
	entries, err := os.ReadDir(skillRoot)
	if os.IsNotExist(err) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	count := 0
	for _, entry := range entries {
		if entry.IsDir() {
			count++
		}
	}
	return count, nil
}

func (s *Service) syncWorkspacePath(currentPath string, targetPath string) error {
	source := strings.TrimSpace(currentPath)
	target := strings.TrimSpace(targetPath)
	if source == "" || target == "" || source == target {
		if target == "" {
			return nil
		}
		return os.MkdirAll(target, 0o755)
	}
	if _, err := os.Stat(source); os.IsNotExist(err) {
		return os.MkdirAll(target, 0o755)
	} else if err != nil {
		return err
	}
	if _, err := os.Stat(target); err == nil {
		return errors.New("目标工作区目录已存在")
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	return os.Rename(source, target)
}
