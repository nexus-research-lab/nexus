package skills

import (
	"cmp"
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"slices"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
	workspacesvc "github.com/nexus-research-lab/nexus/internal/service/workspace"
	skillstore "github.com/nexus-research-lab/nexus/internal/storage/skills"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

var (
	// ErrLocalPathImportUnavailable 表示认证服务端不能把宿主绝对路径当作用户能力。
	ErrLocalPathImportUnavailable = errors.New("authenticated deployment requires archive upload instead of local_path")
)

// Service 提供技能目录、安装与卸载能力。
type Service struct {
	config        config.Config
	agents        *agentsvc.Service
	workspaces    *workspacesvc.Service
	skillStore    *skillstore.Repository
	commandRunner commandRunnerFunc
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

func (s *Service) openAgentWorkspace(
	agentValue *protocol.Agent,
) (*confinedfs.Root, error) {
	if agentValue == nil {
		return nil, errors.New("agent 不能为空")
	}
	return workspacestore.New(s.config.WorkspacePath).OpenOwnerWorkspacePath(
		agentValue.OwnerUserID,
		agentValue.WorkspacePath,
		false,
	)
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
	if isPlatformSkill(record) {
		if err = s.setAgentSkillEnabled(ctx, agentValue, skillReference(record), true); err != nil {
			return nil, err
		}
	} else if record.Detail.SourceType == sourceTypeExternal {
		if err = s.setAgentSkillEnabled(ctx, agentValue, skillReference(record), true); err != nil {
			return nil, err
		}
	} else if err = s.deploySkillToWorkspace(agentValue, record); err != nil {
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
		workspaceRoot, openErr := s.openAgentWorkspace(agentValue)
		if openErr != nil {
			return openErr
		}
		defer workspaceRoot.Close()
		return undeployWorkspaceLocalSkillAt(
			workspaceRoot,
			agentValue.WorkspacePath,
			record,
		)
	}
	if isPlatformSkill(record) {
		return s.setAgentSkillEnabled(ctx, agentValue, skillReference(record), false)
	}
	if record.Detail.SourceType == sourceTypeExternal {
		return s.setAgentSkillEnabled(ctx, agentValue, skillReference(record), false)
	}
	workspaceRoot, err := s.openAgentWorkspace(agentValue)
	if err != nil {
		return err
	}
	defer workspaceRoot.Close()
	return workspacesvc.UndeploySkillAt(workspaceRoot, record.Detail.Name)
}

func isPlatformSkill(record catalogRecord) bool {
	if record.Detail.SourceType == sourceTypeSystem {
		return true
	}
	if record.Detail.SourceType != sourceTypeBuiltin {
		return false
	}
	if record.Detail.SourceKind != "" && record.Detail.SourceKind != sourceKindNexusPlatform {
		return false
	}
	platformSourceRoot := filepath.Join(appfs.Root(), "skills")
	relative, err := filepath.Rel(platformSourceRoot, record.SourcePath)
	if err != nil || relative == "." {
		return false
	}
	return relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func (s *Service) setAgentSkillEnabled(ctx context.Context, agentValue *protocol.Agent, skillName string, enabled bool) error {
	if agentValue == nil {
		return errors.New("agent 不能为空")
	}
	name := strings.TrimSpace(skillName)
	if name == "" {
		return errors.New("skill 名称不能为空")
	}
	selected := make([]string, 0, len(agentValue.Options.SkillIDs)+1)
	seen := map[string]struct{}{}
	canonicalName := name
	if externalName, ok := protocol.ParseExternalSkillReference(name); ok {
		canonicalName = externalName
	}
	for _, current := range agentValue.Options.SkillIDs {
		current = strings.TrimSpace(current)
		if current == "" {
			continue
		}
		if enabled && skillReferenceMatches(current, canonicalName) {
			current = name
		}
		key := strings.ToLower(current)
		if !enabled && skillReferenceMatches(current, canonicalName) {
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		selected = append(selected, current)
	}
	if enabled {
		if _, exists := seen[strings.ToLower(name)]; !exists {
			selected = append(selected, name)
		}
	}
	options := agentValue.Options
	options.SkillIDs = selected
	_, err := s.agents.UpdateAgent(ctx, agentValue.AgentID, protocol.UpdateRequest{Options: &options})
	return err
}

// ImportLocalPath 从本地目录导入外部 skill。
func (s *Service) ImportLocalPath(ctx context.Context, localPath string) (*Detail, error) {
	if state, ok := authctx.StateFromContext(ctx); ok && state.AuthRequired {
		return nil, ErrLocalPathImportUnavailable
	}
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
	for index := range agents {
		agentValue := agents[index]
		selected, changed := removeSkillReferences(agentValue.Options.SkillIDs, record.Detail.Name)
		if changed {
			options := agentValue.Options
			options.SkillIDs = selected
			if _, err = s.agents.UpdateAgent(ctx, agentValue.AgentID, protocol.UpdateRequest{Options: &options}); err != nil {
				return err
			}
		}
	}
	if s.skillStore != nil {
		if err = s.skillStore.DeleteImportedSkill(ctx, authctx.OwnerUserID(ctx), record.Detail.Name); err != nil {
			return err
		}
	}
	ownerUserID := authctx.OwnerUserID(ctx)
	boundaryFS, err := workspacestore.New(s.config.WorkspacePath).OpenOwnerWorkspacePath(
		ownerUserID,
		workspacesvc.UserSkillLibraryRoot(s.config, ownerUserID),
		false,
	)
	if err != nil {
		return err
	}
	defer boundaryFS.Close()
	relativePath, err := relativeSkillPath(boundaryFS, record.SourcePath)
	if err != nil {
		return err
	}
	if err = boundaryFS.RemoveAll(relativePath); err != nil {
		return err
	}
	return workspacesvc.RefreshUserSkillLibrary(s.config, ownerUserID)
}
