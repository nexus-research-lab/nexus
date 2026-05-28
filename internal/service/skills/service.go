package skills

import (
	"context"
	"crypto/sha256"
	"database/sql"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
	workspacesvc "github.com/nexus-research-lab/nexus/internal/service/workspace"
	"github.com/nexus-research-lab/nexus/internal/storage/jsoncodec"
	skillstore "github.com/nexus-research-lab/nexus/internal/storage/skills"
)

const (
	sourceTypeSystem    = "system"
	sourceTypeBuiltin   = "builtin"
	sourceTypeExternal  = "external"
	sourceTypeWorkspace = "workspace"
	scopeMain           = "main"
	scopeAny            = "any"
	scopeRoom           = "room"

	registryUsersDirName            = "users"
	registryLegacyMigratedDirName   = "legacy-migrated"
	registryLegacyUnassignedDirName = "legacy-unassigned"
)

// ScopeRoom 表示 Room 级 skill，只能由房间启用，不能安装到单个 Agent。
const ScopeRoom = scopeRoom

var (
	systemSkillNames   = map[string]struct{}{"imagegen": {}, "memory-manager": {}, "scheduled-task-manager": {}}
	internalSkillNames = map[string]struct{}{"nexus-manager": {}}
	curatedEntriesOnce sync.Once
	curatedEntriesData map[string]map[string]string
	curatedEntriesErr  error
)

// 中文注释：catalog 元数据直接编进二进制，避免运行时容器再依赖源码目录。
//
//go:embed data/curated_skill_catalog.json
var curatedCatalogPayload []byte

// Info 表示 skill 列表项。
type Info struct {
	Name         string   `json:"name"`
	Title        string   `json:"title"`
	Description  string   `json:"description"`
	Scope        string   `json:"scope"`
	Tags         []string `json:"tags"`
	CategoryKey  string   `json:"category_key"`
	CategoryName string   `json:"category_name"`
	SourceType   string   `json:"source_type"`
	SourceRef    string   `json:"source_ref"`
	Version      string   `json:"version"`
	Installed    bool     `json:"installed"`
	Locked       bool     `json:"locked"`
	HasUpdate    bool     `json:"has_update"`
	Deletable    bool     `json:"deletable"`
	SourceKind   string   `json:"source_kind,omitempty"`
	SourceName   string   `json:"source_name,omitempty"`
	SourceTrust  string   `json:"source_trust,omitempty"`
	ImportMode   string   `json:"import_mode,omitempty"`
	LastError    string   `json:"last_error,omitempty"`
}

// Detail 表示 skill 详情。
type Detail struct {
	Info
	ReadmeMarkdown string `json:"readme_markdown"`
	Recommendation string `json:"recommendation"`
}

// Query 表示技能查询参数。
type Query struct {
	AgentID     string
	CategoryKey string
	SourceType  string
	Scope       string
	Q           string
}

type curatedCatalog struct {
	Skills []struct {
		Name           string `json:"name"`
		CategoryKey    string `json:"category_key"`
		CategoryName   string `json:"category_name"`
		Recommendation string `json:"recommendation"`
	} `json:"skills"`
}

type externalManifest struct {
	Name           string   `json:"name"`
	Title          string   `json:"title"`
	Description    string   `json:"description"`
	Scope          string   `json:"scope"`
	Tags           []string `json:"tags"`
	CategoryKey    string   `json:"category_key"`
	CategoryName   string   `json:"category_name"`
	Version        string   `json:"version"`
	SourceType     string   `json:"source_type"`
	SourceRef      string   `json:"source_ref"`
	SourceKind     string   `json:"source_kind"`
	SourceKey      string   `json:"source_key"`
	SourceName     string   `json:"source_name"`
	SourceTrust    string   `json:"source_trust"`
	ImportMode     string   `json:"import_mode"`
	Recommendation string   `json:"recommendation"`
	GitURL         string   `json:"git_url"`
	GitBranch      string   `json:"git_branch"`
	GitPath        string   `json:"git_path"`
	GitCommit      string   `json:"git_commit"`
	RawURL         string   `json:"raw_url"`
	DetailURL      string   `json:"detail_url"`
}

type catalogRecord struct {
	Detail     Detail
	SourcePath string
}

type commandRunnerFunc func(ctx context.Context, workDir string, extraEnv []string, command ...string) (string, error)

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
	sort.Slice(items, func(i int, j int) bool {
		if items[i].CategoryName != items[j].CategoryName {
			return items[i].CategoryName < items[j].CategoryName
		}
		return items[i].Title < items[j].Title
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

func (s *Service) catalogWithAgentState(ctx context.Context, agentID string) (map[string]catalogRecord, map[string]bool, bool, error) {
	records, err := s.loadCatalogRecords(ctx)
	if err != nil {
		return nil, nil, false, err
	}
	installedNames := map[string]bool{}
	isMainAgent := false
	if strings.TrimSpace(agentID) != "" {
		agentValue, err := s.ensureAgent(ctx, agentID)
		if err != nil {
			return nil, nil, false, err
		}
		isMainAgent = agentValue.IsMain
		names, err := workspacesvc.ListDeployedSkills(agentValue.WorkspacePath)
		if err != nil {
			return nil, nil, false, err
		}
		for _, name := range names {
			installedNames[name] = true
		}
		s.addWorkspaceLocalRecords(agentValue.WorkspacePath, records, installedNames)
	}
	return records, installedNames, isMainAgent, nil
}

func (s *Service) addWorkspaceLocalRecords(workspacePath string, records map[string]catalogRecord, installedNames map[string]bool) {
	skillDirs := discoverWorkspaceSkillDirs(workspacePath)
	skillNames := make([]string, 0, len(skillDirs))
	for skillName := range skillDirs {
		skillNames = append(skillNames, skillName)
	}
	sort.Strings(skillNames)
	for _, skillName := range skillNames {
		if _, ok := records[skillName]; ok {
			installedNames[skillName] = true
			continue
		}
		record, err := buildWorkspaceRecord(skillDirs[skillName])
		if err != nil {
			continue
		}
		if _, ok := records[record.Detail.Name]; ok {
			installedNames[record.Detail.Name] = true
			continue
		}
		records[record.Detail.Name] = record
		installedNames[record.Detail.Name] = true
	}
}

func discoverWorkspaceSkillDirs(workspacePath string) map[string]string {
	root := strings.TrimSpace(workspacePath)
	result := map[string]string{}
	addSkillDirs := func(parent string) {
		entries, err := os.ReadDir(parent)
		if err != nil {
			return
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			skillDir := filepath.Join(parent, entry.Name())
			if _, err := os.Stat(filepath.Join(skillDir, "SKILL.md")); err != nil {
				continue
			}
			if _, exists := result[entry.Name()]; !exists {
				result[entry.Name()] = skillDir
			}
		}
	}
	addSkillDirs(filepath.Join(root, ".agents", "skills"))
	addSkillDirs(filepath.Join(root, ".agents"))
	return result
}

func (s *Service) ensureAgent(ctx context.Context, agentID string) (*protocol.Agent, error) {
	agentValue, err := s.agents.GetAgent(ctx, strings.TrimSpace(agentID))
	if err != nil {
		return nil, err
	}
	if err = workspacesvc.EnsureInitialized(
		agentValue.AgentID,
		agentValue.Name,
		agentValue.WorkspacePath,
		agentValue.IsMain,
		agentValue.CreatedAt,
	); err != nil {
		return nil, err
	}
	return agentValue, nil
}

func (s *Service) deploySkillToWorkspace(agentValue *protocol.Agent, record catalogRecord) error {
	context := workspacesvc.BuildSkillRenderContext(agentValue.AgentID, agentValue.Name, agentValue.WorkspacePath, agentValue.CreatedAt)
	return workspacesvc.DeploySkill(record.Detail.Name, record.SourcePath, agentValue.WorkspacePath, context)
}

func (s *Service) loadCatalogRecords(ctx context.Context) (map[string]catalogRecord, error) {
	records := map[string]catalogRecord{}
	curatedEntries, err := s.loadCuratedEntries()
	if err != nil {
		return nil, err
	}
	for skillName := range systemSkillNames {
		record, err := s.buildSystemRecord(skillName)
		if err != nil {
			return nil, err
		}
		records[skillName] = record
	}
	for _, root := range builtinSearchRoots(projectRoot()) {
		entries, err := os.ReadDir(root)
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			skillName := entry.Name()
			if _, ok := systemSkillNames[skillName]; ok {
				continue
			}
			if _, ok := internalSkillNames[skillName]; ok {
				continue
			}
			if _, ok := records[skillName]; ok {
				continue
			}
			record, buildErr := s.buildBuiltinRecord(filepath.Join(root, skillName), curatedEntries[skillName])
			if buildErr != nil {
				continue
			}
			records[skillName] = record
		}
	}
	externalRecords, err := s.loadExternalRecords(ctx)
	if err != nil {
		return nil, err
	}
	for name, record := range externalRecords {
		records[name] = record
	}
	return records, nil
}

func (s *Service) buildSystemRecord(skillName string) (catalogRecord, error) {
	sourceDir := filepath.Join(projectRoot(), "skills", skillName)
	content, _, _, err := readSkillSource(sourceDir)
	if err != nil {
		return catalogRecord{}, err
	}
	parsed := parseSkillFrontmatter(content, skillName)
	detail := Detail{
		Info: Info{
			Name:         parsed.Name,
			Title:        firstNonEmpty(parsed.Title, parsed.Name),
			Description:  parsed.Description,
			Scope:        defaultSkillScope(parsed.Scope),
			Tags:         parsed.Tags,
			CategoryKey:  "system-builtins",
			CategoryName: "系统内置",
			SourceType:   sourceTypeSystem,
			SourceRef:    sourceDir,
			Version:      "system",
			Locked:       true,
		},
		ReadmeMarkdown: parsed.ReadmeMarkdown,
		Recommendation: "系统内置能力，安装状态由平台托管。",
	}
	return catalogRecord{Detail: detail, SourcePath: sourceDir}, nil
}

func (s *Service) buildBuiltinRecord(sourceDir string, curated map[string]string) (catalogRecord, error) {
	content, _, skillName, err := readSkillSource(sourceDir)
	if err != nil {
		return catalogRecord{}, err
	}
	parsed := parseSkillFrontmatter(content, skillName)
	detail := Detail{
		Info: Info{
			Name:         parsed.Name,
			Title:        firstNonEmpty(parsed.Title, parsed.Name),
			Description:  parsed.Description,
			Scope:        defaultSkillScope(parsed.Scope),
			Tags:         parsed.Tags,
			CategoryKey:  firstNonEmpty(curated["category_key"], parsed.CategoryKey, "builtin-misc"),
			CategoryName: firstNonEmpty(curated["category_name"], parsed.CategoryName, "扩展能力"),
			SourceType:   sourceTypeBuiltin,
			SourceRef:    sourceDir,
			Version:      firstNonEmpty(parsed.Version, "builtin"),
			Locked:       false,
			Deletable:    false,
		},
		ReadmeMarkdown: parsed.ReadmeMarkdown,
		Recommendation: firstNonEmpty(curated["recommendation"], parsed.Recommendation, "自动收录的本地可用能力。"),
	}
	return catalogRecord{Detail: detail, SourcePath: sourceDir}, nil
}

func buildWorkspaceRecord(sourceDir string) (catalogRecord, error) {
	content, _, skillName, err := readSkillSource(sourceDir)
	if err != nil {
		return catalogRecord{}, err
	}
	parsed := parseSkillFrontmatter(content, skillName)
	detail := Detail{
		Info: Info{
			Name:         parsed.Name,
			Title:        firstNonEmpty(parsed.Title, parsed.Name),
			Description:  parsed.Description,
			Scope:        defaultSkillScope(parsed.Scope),
			Tags:         parsed.Tags,
			CategoryKey:  firstNonEmpty(parsed.CategoryKey, "agent-workspace"),
			CategoryName: firstNonEmpty(parsed.CategoryName, "智能体工作区"),
			SourceType:   sourceTypeWorkspace,
			SourceRef:    sourceDir,
			Version:      firstNonEmpty(parsed.Version, "workspace"),
			Installed:    true,
			Locked:       false,
			Deletable:    true,
		},
		ReadmeMarkdown: parsed.ReadmeMarkdown,
		Recommendation: firstNonEmpty(parsed.Recommendation, "仅在该智能体工作区内可用。"),
	}
	return catalogRecord{Detail: detail, SourcePath: sourceDir}, nil
}

func undeployWorkspaceLocalSkill(workspacePath string, record catalogRecord) error {
	workspaceRoot := filepath.Clean(strings.TrimSpace(workspacePath))
	sourcePath := filepath.Clean(strings.TrimSpace(record.SourcePath))
	if workspaceRoot == "." || sourcePath == "." {
		return errors.New("workspace skill path is empty")
	}
	agentsRoot := filepath.Join(workspaceRoot, ".agents")
	relativePath, err := filepath.Rel(agentsRoot, sourcePath)
	if err != nil || relativePath == "." || relativePath == ".." || strings.HasPrefix(relativePath, ".."+string(os.PathSeparator)) {
		return errors.New("workspace skill path is outside .agents")
	}
	if err = os.RemoveAll(sourcePath); err != nil {
		return err
	}
	skillNames := []string{record.Detail.Name, filepath.Base(sourcePath)}
	seen := map[string]struct{}{}
	for _, skillName := range skillNames {
		trimmedName := strings.TrimSpace(skillName)
		if trimmedName == "" {
			continue
		}
		if _, ok := seen[trimmedName]; ok {
			continue
		}
		seen[trimmedName] = struct{}{}
		linkPath := filepath.Join(workspaceRoot, ".claude", "skills", trimmedName)
		if err = os.Remove(linkPath); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func (s *Service) loadExternalRecords(ctx context.Context) (map[string]catalogRecord, error) {
	if err := s.ensureLegacyRegistryMigrated(ctx); err != nil {
		return nil, err
	}
	root := s.registryRoot(ctx)
	if s.skillStore != nil {
		if err := s.backfillImportedSkillRecords(ctx, root); err != nil {
			return nil, err
		}
		return s.loadExternalRecordsFromDB(ctx, root)
	}
	return s.loadExternalRecordsFromRoot(root)
}

func (s *Service) loadExternalRecordsFromDB(ctx context.Context, root string) (map[string]catalogRecord, error) {
	records, err := s.skillStore.ListImportedSkills(ctx, authctx.OwnerUserID(ctx))
	if err != nil {
		return nil, err
	}
	result := map[string]catalogRecord{}
	for _, record := range records {
		item := s.buildExternalRecordFromEntity(root, record)
		result[item.Detail.Name] = item
	}
	return result, nil
}

func (s *Service) buildExternalRecordFromEntity(root string, record skillstore.ImportedSkillEntity) catalogRecord {
	skillDir := filepath.Join(root, record.SkillName)
	content, _, fallbackName, err := readSkillSource(skillDir)
	parsed := parseSkillFrontmatter("", record.SkillName)
	if err == nil {
		parsed = parseSkillFrontmatter(content, fallbackName)
	}
	tags := jsoncodec.ParseStringSlice(record.TagsJSON)
	if tags == nil {
		tags = []string{}
	}
	detail := Detail{
		Info: Info{
			Name:         firstNonEmpty(record.SkillName, parsed.Name),
			Title:        firstNonEmpty(record.Title, parsed.Title, record.SkillName),
			Description:  firstNonEmpty(record.Description, parsed.Description),
			Scope:        defaultSkillScope(firstNonEmpty(record.Scope, parsed.Scope)),
			Tags:         firstNonEmptySlice(tags, parsed.Tags),
			CategoryKey:  firstNonEmpty(record.CategoryKey, parsed.CategoryKey, "custom-imports"),
			CategoryName: firstNonEmpty(record.CategoryName, parsed.CategoryName, "自定义导入"),
			SourceType:   sourceTypeExternal,
			SourceRef:    firstNonEmpty(record.SourceRef, skillDir),
			Version:      firstNonEmpty(record.Version, parsed.Version, "external"),
			Locked:       false,
			HasUpdate:    record.ImportMode == "git" || record.ImportMode == "skills_sh" || record.ImportMode == "url",
			Deletable:    true,
			SourceKind:   record.SourceKind,
			SourceName:   record.SourceName,
			SourceTrust:  record.SourceTrust,
			ImportMode:   record.ImportMode,
			LastError:    record.LastError,
		},
		ReadmeMarkdown: parsed.ReadmeMarkdown,
		Recommendation: firstNonEmpty(record.Recommendation, parsed.Recommendation, "外部导入能力。"),
	}
	return catalogRecord{Detail: detail, SourcePath: skillDir}
}

func (s *Service) backfillImportedSkillRecords(ctx context.Context, root string) error {
	fileRecords, err := s.loadExternalRecordsFromRoot(root)
	if err != nil {
		return err
	}
	for _, record := range fileRecords {
		if existing, getErr := s.skillStore.GetImportedSkill(ctx, authctx.OwnerUserID(ctx), record.Detail.Name); getErr != nil {
			return getErr
		} else if existing != nil {
			continue
		}
		manifest, readErr := s.readManifest(record.SourcePath)
		if readErr != nil {
			continue
		}
		parsed := frontmatterData{
			Name:           record.Detail.Name,
			Title:          record.Detail.Title,
			Description:    record.Detail.Description,
			Scope:          record.Detail.Scope,
			Tags:           record.Detail.Tags,
			Version:        record.Detail.Version,
			CategoryKey:    record.Detail.CategoryKey,
			CategoryName:   record.Detail.CategoryName,
			Recommendation: record.Detail.Recommendation,
			ReadmeMarkdown: record.Detail.ReadmeMarkdown,
		}
		if err = s.upsertImportedSkillRecord(ctx, record.SourcePath, manifest, parsed); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) upsertImportedSkillRecord(ctx context.Context, skillDir string, manifest externalManifest, parsed frontmatterData) error {
	if s.skillStore == nil {
		return nil
	}
	ownerUserID := authctx.OwnerUserID(ctx)
	now := time.Now().UTC()
	entity := skillstore.ImportedSkillEntity{
		OwnerUserID:    ownerUserID,
		SkillName:      firstNonEmpty(manifest.Name, parsed.Name, filepath.Base(skillDir)),
		Title:          firstNonEmpty(manifest.Title, parsed.Title, parsed.Name),
		Description:    firstNonEmpty(manifest.Description, parsed.Description),
		Scope:          defaultSkillScope(firstNonEmpty(manifest.Scope, parsed.Scope)),
		TagsJSON:       jsoncodec.MarshalStringSlice(firstNonEmptySlice(manifest.Tags, parsed.Tags)),
		CategoryKey:    firstNonEmpty(manifest.CategoryKey, parsed.CategoryKey, "custom-imports"),
		CategoryName:   firstNonEmpty(manifest.CategoryName, parsed.CategoryName, "自定义导入"),
		Recommendation: firstNonEmpty(manifest.Recommendation, parsed.Recommendation, "外部导入能力。"),
		Version:        firstNonEmpty(manifest.Version, parsed.Version, "external"),
		SourceID:       s.importedSkillSourceID(manifest),
		SourceKind:     manifest.SourceKind,
		SourceRef:      manifest.SourceRef,
		SourceName:     manifest.SourceName,
		SourceTrust:    firstNonEmpty(manifest.SourceTrust, externalSourceTrustCommunity),
		ImportMode:     manifest.ImportMode,
		GitURL:         manifest.GitURL,
		GitBranch:      manifest.GitBranch,
		GitPath:        manifest.GitPath,
		GitCommit:      manifest.GitCommit,
		RawURL:         manifest.RawURL,
		DetailURL:      manifest.DetailURL,
		ContentHash:    hashSkillContent(skillDir),
		LastImportedAt: &now,
	}
	return s.skillStore.UpsertImportedSkill(ctx, entity)
}

func (s *Service) importedSkillSourceID(manifest externalManifest) string {
	sourceKey := strings.TrimSpace(manifest.SourceKey)
	if strings.HasPrefix(sourceKey, "skill_src_") {
		return sourceKey
	}
	sourceURL := firstNonEmpty(manifest.GitURL, manifest.RawURL, manifest.DetailURL)
	if sourceURL == "" && strings.HasPrefix(strings.TrimSpace(manifest.SourceRef), "http") {
		sourceURL = manifest.SourceRef
	}
	if sourceURL == "" && manifest.ImportMode == "skills_sh" {
		sourceURL = firstNonEmpty(strings.TrimSpace(s.config.SkillsAPIURL), "https://skills.sh")
	}
	if sourceURL == "" {
		return ""
	}
	return buildSkillSourceID(firstNonEmpty(manifest.SourceKind, manifest.ImportMode), sourceURL)
}

func hashSkillContent(skillDir string) string {
	payload, err := os.ReadFile(filepath.Join(skillDir, "SKILL.md"))
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func buildSkillSourceID(kind string, sourceURL string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(kind) + "\x00" + strings.TrimSpace(sourceURL)))
	return "skill_src_" + hex.EncodeToString(sum[:10])
}

func (s *Service) loadExternalRecordsFromRoot(root string) (map[string]catalogRecord, error) {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, err
	}
	result := map[string]catalogRecord{}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		skillDir := filepath.Join(root, entry.Name())
		payload, readErr := os.ReadFile(filepath.Join(skillDir, ".nexus-skill.json"))
		if readErr != nil {
			continue
		}
		var manifest externalManifest
		if json.Unmarshal(payload, &manifest) != nil {
			continue
		}
		content, _, skillName, sourceErr := readSkillSource(skillDir)
		if sourceErr != nil {
			continue
		}
		parsed := parseSkillFrontmatter(content, skillName)
		detail := Detail{
			Info: Info{
				Name:         firstNonEmpty(manifest.Name, parsed.Name),
				Title:        firstNonEmpty(manifest.Title, parsed.Title, skillName),
				Description:  firstNonEmpty(manifest.Description, parsed.Description),
				Scope:        defaultSkillScope(firstNonEmpty(manifest.Scope, parsed.Scope)),
				Tags:         firstNonEmptySlice(manifest.Tags, parsed.Tags),
				CategoryKey:  firstNonEmpty(manifest.CategoryKey, parsed.CategoryKey, "custom-imports"),
				CategoryName: firstNonEmpty(manifest.CategoryName, parsed.CategoryName, "自定义导入"),
				SourceType:   sourceTypeExternal,
				SourceRef:    firstNonEmpty(manifest.SourceRef, skillDir),
				Version:      firstNonEmpty(manifest.Version, parsed.Version, "external"),
				Locked:       false,
				HasUpdate:    manifest.ImportMode == "git" || manifest.ImportMode == "skills_sh" || manifest.ImportMode == "url",
				Deletable:    true,
			},
			ReadmeMarkdown: parsed.ReadmeMarkdown,
			Recommendation: firstNonEmpty(manifest.Recommendation, parsed.Recommendation, "外部导入能力。"),
		}
		result[detail.Name] = catalogRecord{Detail: detail, SourcePath: skillDir}
	}
	return result, nil
}

func (s *Service) ensureLegacyRegistryMigrated(ctx context.Context) error {
	// TODO(skill-legacy-registry): 这是旧全局 registry 的一次性兼容迁移逻辑，存量数据完成迁移后移除。
	s.legacyRegistryMu.Lock()
	defer s.legacyRegistryMu.Unlock()

	baseRoot := s.registryBaseRoot()
	entries, err := os.ReadDir(baseRoot)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	legacyDirs := map[string]string{}
	for _, entry := range entries {
		if !entry.IsDir() || isReservedRegistryDir(entry.Name()) {
			continue
		}
		skillDir := filepath.Join(baseRoot, entry.Name())
		skillName, ok := legacyExternalSkillName(skillDir)
		if !ok {
			continue
		}
		legacyDirs[skillName] = skillDir
	}
	if len(legacyDirs) == 0 {
		return nil
	}
	usageOwners, err := s.legacySkillUsageOwners(ctx)
	if err != nil {
		return err
	}
	for skillName, skillDir := range legacyDirs {
		owners := sortedOwnerSet(usageOwners[skillName])
		if len(owners) == 0 {
			if err = s.archiveLegacySkillDir(skillName, skillDir, registryLegacyUnassignedDirName); err != nil {
				return err
			}
			continue
		}
		for _, ownerUserID := range owners {
			targetDir := filepath.Join(s.registryRootForOwner(ownerUserID), skillName)
			if _, statErr := os.Stat(targetDir); statErr == nil {
				continue
			} else if statErr != nil && !os.IsNotExist(statErr) {
				return statErr
			}
			if err = copyDirectory(skillDir, targetDir); err != nil {
				return err
			}
		}
		if err = s.archiveLegacySkillDir(skillName, skillDir, registryLegacyMigratedDirName); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) legacySkillUsageOwners(ctx context.Context) (map[string]map[string]struct{}, error) {
	if s.agents == nil {
		return map[string]map[string]struct{}{}, nil
	}
	agents, err := s.agents.ListAllAgentRecordsForMaintenance(ctx)
	if err != nil {
		return nil, err
	}
	result := map[string]map[string]struct{}{}
	for _, agentValue := range agents {
		ownerUserID := strings.TrimSpace(agentValue.OwnerUserID)
		if ownerUserID == "" {
			continue
		}
		names, err := workspacesvc.ListDeployedSkills(agentValue.WorkspacePath)
		if err != nil {
			return nil, err
		}
		for _, name := range names {
			normalizedName := strings.TrimSpace(name)
			if normalizedName == "" {
				continue
			}
			if _, ok := result[normalizedName]; !ok {
				result[normalizedName] = map[string]struct{}{}
			}
			result[normalizedName][ownerUserID] = struct{}{}
		}
	}
	return result, nil
}

func (s *Service) archiveLegacySkillDir(skillName string, sourceDir string, bucket string) error {
	targetDir := filepath.Join(s.registryBaseRoot(), bucket, skillName)
	if err := os.MkdirAll(filepath.Dir(targetDir), 0o755); err != nil {
		return err
	}
	if err := os.RemoveAll(targetDir); err != nil {
		return err
	}
	if err := os.Rename(sourceDir, targetDir); err == nil {
		return nil
	}
	if err := copyDirectory(sourceDir, targetDir); err != nil {
		return err
	}
	return os.RemoveAll(sourceDir)
}

func legacyExternalSkillName(skillDir string) (string, bool) {
	payload, err := os.ReadFile(filepath.Join(skillDir, ".nexus-skill.json"))
	if err != nil {
		return "", false
	}
	var manifest externalManifest
	if json.Unmarshal(payload, &manifest) != nil {
		return "", false
	}
	content, _, fallbackName, err := readSkillSource(skillDir)
	if err != nil {
		return "", false
	}
	parsed := parseSkillFrontmatter(content, fallbackName)
	skillName := firstNonEmpty(manifest.Name, parsed.Name, fallbackName)
	return skillName, skillName != ""
}

func isReservedRegistryDir(name string) bool {
	switch strings.TrimSpace(name) {
	case registryUsersDirName, registryLegacyMigratedDirName, registryLegacyUnassignedDirName:
		return true
	default:
		return false
	}
}

func sortedOwnerSet(owners map[string]struct{}) []string {
	result := make([]string, 0, len(owners))
	for ownerUserID := range owners {
		if strings.TrimSpace(ownerUserID) != "" {
			result = append(result, strings.TrimSpace(ownerUserID))
		}
	}
	sort.Strings(result)
	return result
}

func (s *Service) loadCuratedEntries() (map[string]map[string]string, error) {
	curatedEntriesOnce.Do(func() {
		var catalog curatedCatalog
		if err := json.Unmarshal(curatedCatalogPayload, &catalog); err != nil {
			curatedEntriesErr = err
			return
		}
		curatedEntriesData = make(map[string]map[string]string, len(catalog.Skills))
		for _, item := range catalog.Skills {
			curatedEntriesData[item.Name] = map[string]string{
				"category_key":   item.CategoryKey,
				"category_name":  item.CategoryName,
				"recommendation": item.Recommendation,
			}
		}
	})
	if curatedEntriesErr != nil {
		return nil, curatedEntriesErr
	}
	return cloneCuratedEntries(curatedEntriesData), nil
}

func cloneCuratedEntries(source map[string]map[string]string) map[string]map[string]string {
	result := make(map[string]map[string]string, len(source))
	for name, metadata := range source {
		copied := make(map[string]string, len(metadata))
		for key, value := range metadata {
			copied[key] = value
		}
		result[name] = copied
	}
	return result
}

func (s *Service) registryBaseRoot() string {
	base := strings.TrimSpace(s.config.CacheFileDir)
	if base == "" {
		base = "cache"
	}
	return filepath.Join(base, "skills", "registry")
}

func (s *Service) registryRoot(ctx context.Context) string {
	return s.registryRootForOwner(authctx.OwnerUserID(ctx))
}

func (s *Service) registryRootForOwner(ownerUserID string) string {
	return filepath.Join(s.registryBaseRoot(), registryUsersDirName, safeRegistrySegment(ownerUserID))
}

func safeRegistrySegment(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return authctx.SystemUserID
	}
	var builder strings.Builder
	for _, item := range trimmed {
		switch {
		case item >= 'a' && item <= 'z':
			builder.WriteRune(item)
		case item >= 'A' && item <= 'Z':
			builder.WriteRune(item)
		case item >= '0' && item <= '9':
			builder.WriteRune(item)
		case item == '-' || item == '_' || item == '.' || item == '@':
			builder.WriteRune(item)
		default:
			builder.WriteRune('_')
		}
	}
	if builder.Len() == 0 {
		return authctx.SystemUserID
	}
	return builder.String()
}

func builtinSearchRoots(root string) []string {
	home, _ := os.UserHomeDir()
	roots := []string{
		filepath.Join(root, "skills"),
		filepath.Join(home, ".codex", "skills"),
		filepath.Join(home, ".agents", "skills"),
		filepath.Join(home, ".cc-switch", "skills"),
	}
	seen := map[string]struct{}{}
	result := make([]string, 0, len(roots))
	for _, entry := range roots {
		clean := filepath.Clean(entry)
		if _, ok := seen[clean]; ok {
			continue
		}
		seen[clean] = struct{}{}
		result = append(result, clean)
	}
	return result
}

func readSkillSource(sourceDir string) (string, string, string, error) {
	skillMDPath := filepath.Join(sourceDir, "SKILL.md")
	content, err := os.ReadFile(skillMDPath)
	if err != nil {
		return "", "", "", err
	}
	return string(content), skillMDPath, filepath.Base(sourceDir), nil
}

func copyDirectory(sourceDir string, targetDir string) error {
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return err
	}
	return filepath.Walk(sourceDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relativePath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}
		if relativePath == "." {
			return nil
		}
		targetPath := filepath.Join(targetDir, relativePath)
		if info.IsDir() {
			return os.MkdirAll(targetPath, info.Mode())
		}
		if err = os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return err
		}
		sourceFile, openErr := os.Open(path)
		if openErr != nil {
			return openErr
		}
		targetFile, createErr := os.Create(targetPath)
		if createErr != nil {
			_ = sourceFile.Close()
			return createErr
		}
		if _, err = io.Copy(targetFile, sourceFile); err != nil {
			_ = sourceFile.Close()
			_ = targetFile.Close()
			return err
		}
		if err = sourceFile.Close(); err != nil {
			_ = targetFile.Close()
			return err
		}
		if err = targetFile.Close(); err != nil {
			return err
		}
		return os.Chmod(targetPath, info.Mode())
	})
}

func matchSkillQuery(detail Detail, query string) bool {
	fields := []string{
		strings.ToLower(detail.Name),
		strings.ToLower(detail.Title),
		strings.ToLower(detail.Description),
		strings.ToLower(strings.Join(detail.Tags, " ")),
	}
	for _, field := range fields {
		if strings.Contains(field, query) {
			return true
		}
	}
	return false
}

func defaultSkillScope(scope string) string {
	normalized := strings.TrimSpace(scope)
	if normalized == scopeMain {
		return scopeMain
	}
	if normalized == scopeRoom {
		return scopeRoom
	}
	return scopeAny
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func firstNonEmptySlice(candidates ...[]string) []string {
	for _, item := range candidates {
		if len(item) > 0 {
			return append(make([]string, 0, len(item)), item...)
		}
	}
	return []string{}
}

func projectRoot() string {
	return appfs.Root()
}
