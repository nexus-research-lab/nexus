package skills

import (
	"cmp"
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
	workspacesvc "github.com/nexus-research-lab/nexus/internal/service/workspace"
	skillstore "github.com/nexus-research-lab/nexus/internal/storage/skills"
)

// Service 提供技能目录、安装与卸载能力。
type Service struct {
	config           config.Config
	agents           *agentsvc.Service
	workspaces       *workspacesvc.Service
	skillStore       *skillstore.Repository
	commandRunner    commandRunnerFunc
	legacyRegistryMu sync.Mutex
}

// NewService 创建技能服务。
func NewService(cfg config.Config, agents *agentsvc.Service, workspaces *workspacesvc.Service) *Service {
	return &Service{
		config:     cfg,
		agents:     agents,
		workspaces: workspaces,
	}
}

// NewServiceWithDB 创建带数据库状态仓储的技能服务。
func NewServiceWithDB(cfg config.Config, db *sql.DB, agents *agentsvc.Service, workspaces *workspacesvc.Service) *Service {
	service := NewService(cfg, agents, workspaces)
	if db != nil {
		service.skillStore = skillstore.NewRepository(cfg, db)
	}
	return service
}

// ListSkills 返回公开 skill 目录。
func (s *Service) ListSkills(ctx context.Context, query Query) ([]Info, error) {
	records, installedNames, isMainAgent, err := s.catalogWithAgentState(ctx, strings.TrimSpace(query.AgentID))
	if err != nil {
		return nil, err
	}
	items := make([]Info, 0, len(records))
	needle := strings.ToLower(strings.TrimSpace(query.Q))
	for _, record := range records {
		detail := record.Detail
		if !skillVisibleForQuery(detail.Scope, query.Scope, query.AgentID, isMainAgent) {
			continue
		}
		detail.Installed = installedNames[detail.Name]
		if query.CategoryKey != "" && detail.CategoryKey != query.CategoryKey {
			continue
		}
		if query.SourceType != "" && detail.SourceType != query.SourceType {
			continue
		}
		if needle != "" && !matchSkillQuery(detail, needle) {
			continue
		}
		items = append(items, detail.Info)
	}
	slices.SortFunc(items, func(left Info, right Info) int {
		if result := cmp.Compare(left.CategoryName, right.CategoryName); result != 0 {
			return result
		}
		return cmp.Compare(left.Title, right.Title)
	})
	return items, nil
}

// CountSkills 返回符合查询条件的技能数量。
func (s *Service) CountSkills(ctx context.Context, query Query) (int, error) {
	records, installedNames, isMainAgent, err := s.catalogWithAgentState(ctx, strings.TrimSpace(query.AgentID))
	if err != nil {
		return 0, err
	}
	needle := strings.ToLower(strings.TrimSpace(query.Q))
	count := 0
	for _, record := range records {
		detail := record.Detail
		if !skillVisibleForQuery(detail.Scope, query.Scope, query.AgentID, isMainAgent) {
			continue
		}
		detail.Installed = installedNames[detail.Name]
		if query.CategoryKey != "" && detail.CategoryKey != query.CategoryKey {
			continue
		}
		if query.SourceType != "" && detail.SourceType != query.SourceType {
			continue
		}
		if needle != "" && !matchSkillQuery(detail, needle) {
			continue
		}
		count += 1
	}
	return count, nil
}

// GetSkillDetail 返回单个 skill 详情。
func (s *Service) GetSkillDetail(ctx context.Context, skillName string, agentID string) (*Detail, error) {
	records, installedNames, isMainAgent, err := s.catalogWithAgentState(ctx, strings.TrimSpace(agentID))
	if err != nil {
		return nil, err
	}
	record, ok := records[strings.TrimSpace(skillName)]
	if !ok {
		return nil, errors.New("skill not found")
	}
	detail := record.Detail
	if detail.Scope == scopeMain && agentID != "" && !isMainAgent {
		return nil, errors.New("skill not found")
	}
	if detail.Scope == scopeRoom && agentID != "" {
		return nil, errors.New("skill not found")
	}
	detail.Installed = installedNames[detail.Name]
	return &detail, nil
}

// GetAgentSkills 返回 Agent 可见的技能列表。
func (s *Service) GetAgentSkills(ctx context.Context, agentID string) ([]Info, error) {
	return s.ListSkills(ctx, Query{AgentID: agentID})
}

func skillVisibleForQuery(scope string, queryScope string, agentID string, isMainAgent bool) bool {
	normalizedScope := strings.TrimSpace(scope)
	normalizedQueryScope := strings.TrimSpace(queryScope)
	if normalizedQueryScope != "" {
		return normalizedScope == normalizedQueryScope
	}
	if agentID == "" {
		return true
	}
	if normalizedScope == scopeRoom {
		return false
	}
	return normalizedScope != scopeMain || isMainAgent
}

// InstallSkill 为 Agent 部署 skill。
func (s *Service) InstallSkill(ctx context.Context, agentID string, skillName string) (*Info, error) {
	agentValue, err := s.ensureAgent(ctx, agentID)
	if err != nil {
		return nil, err
	}
	records, _, isMainAgent, err := s.catalogWithAgentState(ctx, agentID)
	if err != nil {
		return nil, err
	}
	record, ok := records[strings.TrimSpace(skillName)]
	if !ok {
		return nil, errors.New("skill not found")
	}
	if record.Detail.SourceType == sourceTypeSystem {
		return nil, errors.New("系统托管 skill 不能手动安装")
	}
	if record.Detail.SourceType == sourceTypeWorkspace {
		return nil, errors.New("智能体工作区内 skill 不能从技能市场安装")
	}
	if record.Detail.Scope == scopeMain && !isMainAgent {
		return nil, errors.New("该 skill 仅允许主智能体安装")
	}
	if record.Detail.Scope == scopeRoom {
		return nil, errors.New("room scope skill 不能安装到 agent")
	}
	if err = s.deploySkillToWorkspace(agentValue, record); err != nil {
		return nil, err
	}
	detail, err := s.GetSkillDetail(ctx, skillName, agentID)
	if err != nil {
		return nil, err
	}
	return &detail.Info, nil
}

// UninstallSkill 从 Agent 卸载 skill。
func (s *Service) UninstallSkill(ctx context.Context, agentID string, skillName string) error {
	agentValue, err := s.ensureAgent(ctx, agentID)
	if err != nil {
		return err
	}
	records, _, _, err := s.catalogWithAgentState(ctx, agentID)
	if err != nil {
		return err
	}
	record, ok := records[strings.TrimSpace(skillName)]
	if !ok {
		return errors.New("skill not found")
	}
	if record.Detail.SourceType == sourceTypeSystem {
		return errors.New("系统托管 skill 不能手动卸载")
	}
	if record.Detail.SourceType == sourceTypeWorkspace {
		return undeployWorkspaceLocalSkill(agentValue.WorkspacePath, record)
	}
	return workspacesvc.UndeploySkill(agentValue.WorkspacePath, record.Detail.Name)
}

// ImportLocalPath 从本地目录导入外部 skill。
func (s *Service) ImportLocalPath(ctx context.Context, localPath string) (*Detail, error) {
	if strings.TrimSpace(localPath) == "" {
		return nil, errors.New("请提供本地 zip 上传文件或 local_path")
	}
	sourceDir := filepath.Clean(strings.TrimSpace(localPath))
	return s.importSourceDir(ctx, sourceDir, externalManifest{
		SourceType:     sourceTypeExternal,
		SourceRef:      sourceDir,
		SourceKind:     externalSourceKindLocalPath,
		SourceName:     "本地路径",
		SourceTrust:    externalSourceTrustPrivate,
		ImportMode:     externalSourceKindLocalPath,
		Version:        "local",
		Recommendation: "来自本地路径导入。",
	})
}

// DeleteSkill 删除外部导入 skill。
func (s *Service) DeleteSkill(ctx context.Context, skillName string) error {
	records, _, _, err := s.catalogWithAgentState(ctx, "")
	if err != nil {
		return err
	}
	record, ok := records[strings.TrimSpace(skillName)]
	if !ok {
		return errors.New("skill not found")
	}
	if record.Detail.SourceType != sourceTypeExternal || !record.Detail.Deletable {
		return errors.New("该 skill 不允许删除")
	}
	agents, err := s.agents.ListAgentRecords(ctx)
	if err != nil {
		return err
	}
	for _, agentValue := range agents {
		if err = workspacesvc.UndeploySkill(agentValue.WorkspacePath, record.Detail.Name); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	if s.skillStore != nil {
		if err = s.skillStore.DeleteImportedSkill(ctx, authctx.OwnerUserID(ctx), record.Detail.Name); err != nil {
			return err
		}
	}
	return os.RemoveAll(record.SourcePath)
}
