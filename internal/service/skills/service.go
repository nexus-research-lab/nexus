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

// Service 提供全局技能库、Agent 启停与外部来源管理能力。
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
	records, enabledNames, isMainAgent, err := s.catalogWithAgentState(ctx, strings.TrimSpace(query.AgentID))
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
		detail.EnabledForAgent = enabledNames[detail.Name]
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
	records, enabledNames, isMainAgent, err := s.catalogWithAgentState(ctx, strings.TrimSpace(query.AgentID))
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
		detail.EnabledForAgent = enabledNames[detail.Name]
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
	records, enabledNames, isMainAgent, err := s.catalogWithAgentState(ctx, strings.TrimSpace(agentID))
	if err != nil {
		return nil, err
	}
	record, ok := findCatalogRecord(records, skillName)
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
	detail.EnabledForAgent = enabledNames[detail.Name]
	return &detail, nil
}

// GetAgentSkills 返回 Agent 可见的技能列表。
func (s *Service) GetAgentSkills(ctx context.Context, agentID string) ([]Info, error) {
	return s.ListSkills(ctx, Query{AgentID: agentID})
}

// ListSkillAgents 返回 Skill 在当前用户各 Agent 上的启用状态。
func (s *Service) ListSkillAgents(ctx context.Context, skillName string) ([]AgentSkillBinding, error) {
	records, err := s.loadCatalogRecords(ctx)
	if err != nil {
		return nil, err
	}
	record, ok := findCatalogRecord(records, skillName)
	if !ok {
		return nil, errors.New("skill not found")
	}
	if s.agents == nil {
		return []AgentSkillBinding{}, nil
	}
	agents, err := s.agents.ListAgentRecords(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]AgentSkillBinding, 0, len(agents))
	for index := range agents {
		agentValue := &agents[index]
		available := skillAvailableForAgent(record, agentValue)
		enabled := skillEnabledForAgent(agentValue, record.Detail.Name)
		if !available && record.Detail.SourceType != sourceTypeSystem {
			enabled = false
		}
		result = append(result, AgentSkillBinding{
			AgentID:   agentValue.AgentID,
			AgentName: agentValue.Name,
			IsMain:    agentValue.IsMain,
			Available: available,
			Enabled:   enabled,
		})
	}
	slices.SortFunc(result, func(left AgentSkillBinding, right AgentSkillBinding) int {
		if left.IsMain != right.IsMain {
			if left.IsMain {
				return -1
			}
			return 1
		}
		return cmp.Compare(left.AgentName, right.AgentName)
	})
	return result, nil
}

func skillAvailableForAgent(record catalogRecord, agentValue *protocol.Agent) bool {
	if agentValue == nil ||
		record.Detail.SourceType == sourceTypeSystem ||
		record.Detail.Scope == scopeRoom {
		return false
	}
	return record.Detail.Scope != scopeMain || agentValue.IsMain
}

func skillEnabledForAgent(agentValue *protocol.Agent, skillName string) bool {
	if agentValue == nil {
		return false
	}
	for _, reference := range agentValue.Options.SkillIDs {
		if skillReferenceMatches(reference, skillName) {
			return true
		}
	}
	return false
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

// InstallSkill 保留旧调用入口；语义等同于启用全局 Skill。
func (s *Service) InstallSkill(ctx context.Context, agentID string, skillName string) (*Info, error) {
	return s.SetAgentSkillEnabledInScope(
		ctx,
		agentID,
		skillName,
		true,
		AgentSkillTargetGlobalLibrary,
	)
}

// UninstallSkill 保留旧 DELETE 入口；全局 Skill 只解除 Agent 绑定，
// Agent 本地 Skill 才会在显式 DELETE 时删除 workspace 文件。
func (s *Service) UninstallSkill(ctx context.Context, agentID string, skillName string) error {
	agentValue, err := s.ensureAgent(ctx, agentID)
	if err != nil {
		return err
	}
	records, _, _, err := s.catalogWithAgentState(ctx, agentID)
	if err != nil {
		return err
	}
	record, ok := findCatalogRecord(records, skillName)
	if !ok {
		return errors.New("skill not found")
	}
	if record.Detail.SourceType == sourceTypeSystem {
		return errors.New("系统托管 skill 不能删除")
	}
	if record.Detail.SourceType == sourceTypeWorkspace {
		workspaceRoot, openErr := s.openAgentWorkspace(agentValue)
		if openErr != nil {
			return openErr
		}
		defer workspaceRoot.Close()
		if err = undeployWorkspaceLocalSkillAt(
			workspaceRoot,
			agentValue.WorkspacePath,
			record,
		); err != nil {
			return err
		}
		// DELETE 代表移除本地文件；同步清掉仅针对该本地 Skill 的停用项，
		// 避免文件删除后留下无法解释的 Agent 状态。
		disabled, changed := removeSkillReferences(
			agentValue.Options.DisabledSkillIDs,
			record.Detail.Name,
		)
		if !changed {
			return nil
		}
		_, err = s.agents.UpdateAgentSkillSelection(
			ctx,
			agentValue.AgentID,
			agentValue.Options.SkillIDs,
			disabled,
		)
		return err
	}
	_, err = s.SetAgentSkillEnabledInScope(
		ctx,
		agentID,
		record.Detail.Name,
		false,
		AgentSkillTargetGlobalLibrary,
	)
	return err
}

// SetAgentSkillEnabled 更新单个 Agent 的技能开关，不删除工作区文件。
func (s *Service) SetAgentSkillEnabled(
	ctx context.Context,
	agentID string,
	skillName string,
	enabled bool,
) (*Info, error) {
	return s.SetAgentSkillEnabledInScope(ctx, agentID, skillName, enabled, "")
}

// SetAgentSkillEnabledInScope 更新指定来源作用域的 Agent Skill 开关。
//
// 全局详情必须显式传 global_library，避免同名 Agent 本地 Skill 覆盖全局记录；
// Agent 设置页则按卡片来源传入 global_library 或 agent_workspace。
func (s *Service) SetAgentSkillEnabledInScope(
	ctx context.Context,
	agentID string,
	skillName string,
	enabled bool,
	targetScope AgentSkillTargetScope,
) (*Info, error) {
	agentValue, err := s.ensureAgent(ctx, agentID)
	if err != nil {
		return nil, err
	}
	record, err := s.resolveAgentSkillTarget(ctx, agentID, skillName, targetScope)
	if err != nil {
		return nil, err
	}
	if record.Detail.SourceType == sourceTypeSystem {
		return nil, errors.New("系统托管 skill 不能手动切换")
	}
	if record.Detail.Scope == scopeMain && !agentValue.IsMain {
		return nil, errors.New("该 skill 仅允许主智能体使用")
	}
	if record.Detail.Scope == scopeRoom {
		return nil, errors.New("room scope skill 不能绑定到 agent")
	}
	workspaceLocal := record.Detail.SourceType == sourceTypeWorkspace
	reference := record.Detail.Name
	if !workspaceLocal {
		reference = skillReference(record)
	}
	if err = s.setAgentSkillEnabled(ctx, agentValue, reference, enabled, workspaceLocal); err != nil {
		return nil, err
	}
	if targetScope == AgentSkillTargetGlobalLibrary {
		info := record.Detail.Info
		info.EnabledForAgent = enabled
		return &info, nil
	}
	detail, err := s.GetSkillDetail(ctx, record.Detail.Name, agentID)
	if err != nil {
		return nil, err
	}
	return &detail.Info, nil
}

func (s *Service) resolveAgentSkillTarget(
	ctx context.Context,
	agentID string,
	skillName string,
	targetScope AgentSkillTargetScope,
) (catalogRecord, error) {
	var (
		records map[string]catalogRecord
		err     error
	)
	switch targetScope {
	case AgentSkillTargetGlobalLibrary:
		records, err = s.loadCatalogRecords(ctx)
	case AgentSkillTargetWorkspace, "":
		records, _, _, err = s.catalogWithAgentState(ctx, agentID)
	default:
		return catalogRecord{}, errors.New("skill target_scope 无效")
	}
	if err != nil {
		return catalogRecord{}, err
	}
	record, ok := findCatalogRecord(records, skillName)
	if !ok {
		return catalogRecord{}, errors.New("skill not found")
	}
	if targetScope == AgentSkillTargetWorkspace &&
		record.Detail.SourceType != sourceTypeWorkspace {
		return catalogRecord{}, errors.New("Agent 本地 skill 不存在")
	}
	if targetScope == AgentSkillTargetGlobalLibrary &&
		record.Detail.SourceType == sourceTypeWorkspace {
		return catalogRecord{}, errors.New("全局 skill 不存在")
	}
	return record, nil
}

func (s *Service) setAgentSkillEnabled(
	ctx context.Context,
	agentValue *protocol.Agent,
	skillName string,
	enabled bool,
	workspaceLocal bool,
) error {
	if agentValue == nil {
		return errors.New("agent 不能为空")
	}
	if s.agents == nil {
		return errors.New("agent service 未初始化")
	}
	name := strings.TrimSpace(skillName)
	if name == "" {
		return errors.New("skill 名称不能为空")
	}
	selected := make([]string, 0, len(agentValue.Options.SkillIDs)+1)
	disabled := make([]string, 0, len(agentValue.Options.DisabledSkillIDs)+1)
	seen := map[string]struct{}{}
	disabledSeen := map[string]struct{}{}
	canonicalName := name
	if externalName, ok := protocol.ParseExternalSkillReference(name); ok {
		canonicalName = externalName
	}
	for _, current := range agentValue.Options.SkillIDs {
		current = strings.TrimSpace(current)
		if current == "" {
			continue
		}
		if !workspaceLocal && skillReferenceMatches(current, canonicalName) {
			if !enabled {
				continue
			}
			current = name
		}
		key := strings.ToLower(current)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		selected = append(selected, current)
	}
	for _, current := range agentValue.Options.DisabledSkillIDs {
		current = strings.TrimSpace(current)
		if current == "" {
			continue
		}
		if workspaceLocal && skillReferenceMatches(current, canonicalName) {
			continue
		}
		key := strings.ToLower(current)
		if _, exists := disabledSeen[key]; exists {
			continue
		}
		disabledSeen[key] = struct{}{}
		disabled = append(disabled, current)
	}
	if enabled && !workspaceLocal {
		if _, exists := seen[strings.ToLower(name)]; !exists {
			selected = append(selected, name)
		}
	}
	if !enabled && workspaceLocal {
		if _, exists := disabledSeen[strings.ToLower(canonicalName)]; !exists {
			disabled = append(disabled, canonicalName)
		}
	}
	_, err := s.agents.UpdateAgentSkillSelection(ctx, agentValue.AgentID, selected, disabled)
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
	skillName = strings.TrimSpace(skillName)
	records, _, _, err := s.catalogWithAgentState(ctx, "")
	if err != nil {
		return err
	}
	record, ok := findCatalogRecord(records, skillName)
	if !ok {
		return errors.New("skill not found")
	}
	if record.Detail.SourceType != sourceTypeExternal || !record.Detail.Deletable {
		return errors.New("该 skill 不允许删除")
	}
	return s.applySkillDeletion(ctx, record.Detail.Name, record.SourcePath)
}
