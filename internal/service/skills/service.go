package skills

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"

	"github.com/nexus-research-lab/nexus/internal/appfs"
	"github.com/nexus-research-lab/nexus/internal/config"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
	workspacesvc "github.com/nexus-research-lab/nexus/internal/service/workspace"
)

const (
	sourceTypeSystem   = "system"
	sourceTypeBuiltin  = "builtin"
	sourceTypeExternal = "external"
	scopeMain          = "main"
	scopeAny           = "any"
)

var (
	systemSkillNames   = map[string]struct{}{"memory-manager": {}, "room-collaboration": {}}
	internalSkillNames = map[string]struct{}{"nexus-manager": {}}
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
	ImportMode     string   `json:"import_mode"`
	Recommendation string   `json:"recommendation"`
	GitURL         string   `json:"git_url"`
	GitBranch      string   `json:"git_branch"`
	GitCommit      string   `json:"git_commit"`
}

type catalogRecord struct {
	Detail     Detail
	SourcePath string
}

// Service 提供技能目录、安装与卸载能力。
type Service struct {
	config     config.Config
	agents     *agentsvc.Service
	workspaces *workspacesvc.Service
}

// NewService 创建技能服务。
func NewService(cfg config.Config, agents *agentsvc.Service, workspaces *workspacesvc.Service) *Service {
	return &Service{
		config:     cfg,
		agents:     agents,
		workspaces: workspaces,
	}
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
		if detail.Scope == scopeMain && query.AgentID != "" && !isMainAgent {
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
		if detail.Scope == scopeMain && query.AgentID != "" && !isMainAgent {
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
	detail.Installed = installedNames[detail.Name]
	return &detail, nil
}

// GetAgentSkills 返回 Agent 可见的技能列表。
func (s *Service) GetAgentSkills(ctx context.Context, agentID string) ([]Info, error) {
	return s.ListSkills(ctx, Query{AgentID: agentID})
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
	if record.Detail.Scope == scopeMain && !isMainAgent {
		return nil, errors.New("该 skill 仅允许主智能体安装")
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
	return workspacesvc.UndeploySkill(agentValue.WorkspacePath, record.Detail.Name)
}

// ImportLocalPath 从本地目录导入外部 skill。
func (s *Service) ImportLocalPath(localPath string) (*Detail, error) {
	if strings.TrimSpace(localPath) == "" {
		return nil, errors.New("请提供本地 zip 上传文件或 local_path")
	}
	sourceDir := filepath.Clean(strings.TrimSpace(localPath))
	content, skillMDPath, skillName, err := readSkillSource(sourceDir)
	if err != nil {
		return nil, err
	}
	parsed := parseSkillFrontmatter(content, skillName)
	if parsed.Name == "" {
		return nil, errors.New("SKILL.md 缺少 name")
	}
	targetDir := filepath.Join(s.registryRoot(), parsed.Name)
	if err = os.RemoveAll(targetDir); err != nil {
		return nil, err
	}
	if err = copyDirectory(filepath.Dir(skillMDPath), targetDir); err != nil {
		return nil, err
	}
	manifest := externalManifest{
		Name:           parsed.Name,
		Title:          parsed.Title,
		Description:    parsed.Description,
		Scope:          defaultSkillScope(parsed.Scope),
		Tags:           parsed.Tags,
		CategoryKey:    firstNonEmpty(parsed.CategoryKey, "custom-imports"),
		CategoryName:   firstNonEmpty(parsed.CategoryName, "自定义导入"),
		Version:        firstNonEmpty(parsed.Version, "local"),
		SourceType:     sourceTypeExternal,
		SourceRef:      sourceDir,
		ImportMode:     "local_path",
		Recommendation: firstNonEmpty(parsed.Recommendation, "来自本地路径导入。"),
	}
	payload, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, err
	}
	if err = os.WriteFile(filepath.Join(targetDir, ".nexus-skill.json"), payload, 0o644); err != nil {
		return nil, err
	}
	return s.GetSkillDetail(context.Background(), parsed.Name, "")
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
	agents, err := s.agents.ListAgents(ctx)
	if err != nil {
		return err
	}
	for _, agentValue := range agents {
		if err = workspacesvc.UndeploySkill(agentValue.WorkspacePath, record.Detail.Name); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return os.RemoveAll(filepath.Join(s.registryRoot(), record.Detail.Name))
}

func (s *Service) catalogWithAgentState(ctx context.Context, agentID string) (map[string]catalogRecord, map[string]bool, bool, error) {
	records, err := s.loadCatalogRecords()
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
	}
	return records, installedNames, isMainAgent, nil
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

func (s *Service) loadCatalogRecords() (map[string]catalogRecord, error) {
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
	externalRecords, err := s.loadExternalRecords()
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

func (s *Service) loadExternalRecords() (map[string]catalogRecord, error) {
	root := s.registryRoot()
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
				HasUpdate:    manifest.ImportMode == "git" || manifest.ImportMode == "skills_sh",
				Deletable:    true,
			},
			ReadmeMarkdown: parsed.ReadmeMarkdown,
			Recommendation: firstNonEmpty(manifest.Recommendation, parsed.Recommendation, "外部导入能力。"),
		}
		result[detail.Name] = catalogRecord{Detail: detail, SourcePath: skillDir}
	}
	return result, nil
}

func (s *Service) loadCuratedEntries() (map[string]map[string]string, error) {
	var catalog curatedCatalog
	if err := json.Unmarshal(curatedCatalogPayload, &catalog); err != nil {
		return nil, err
	}
	result := make(map[string]map[string]string, len(catalog.Skills))
	for _, item := range catalog.Skills {
		result[item.Name] = map[string]string{
			"category_key":   item.CategoryKey,
			"category_name":  item.CategoryName,
			"recommendation": item.Recommendation,
		}
	}
	return result, nil
}

func (s *Service) registryRoot() string {
	base := strings.TrimSpace(s.config.CacheFileDir)
	if base == "" {
		base = "cache"
	}
	return filepath.Join(base, "skills", "registry")
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
		sourceFile, err := os.Open(path)
		if err != nil {
			return err
		}
		defer sourceFile.Close()
		targetFile, err := os.Create(targetPath)
		if err != nil {
			return err
		}
		if _, err = io.Copy(targetFile, sourceFile); err != nil {
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
	if strings.TrimSpace(scope) == scopeMain {
		return scopeMain
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
