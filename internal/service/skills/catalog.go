package skills

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func (s *Service) catalogWithAgentState(ctx context.Context, agentID string) (map[string]catalogRecord, map[string]bool, bool, error) {
	records, err := s.loadCatalogRecords(ctx)
	if err != nil {
		return nil, nil, false, err
	}
	if err = s.populateAgentUsageCounts(ctx, records); err != nil {
		return nil, nil, false, err
	}
	enabledNames := map[string]bool{}
	isMainAgent := false
	if strings.TrimSpace(agentID) != "" {
		agentValue, err := s.ensureAgent(ctx, agentID)
		if err != nil {
			return nil, nil, false, err
		}
		isMainAgent = agentValue.IsMain
		enabledNames = enabledSkillNames(agentValue, records)
		disabled := disabledSkillNames(agentValue)
		workspaceRoot, err := s.openAgentWorkspace(agentValue)
		if err != nil {
			return nil, nil, false, err
		}
		defer workspaceRoot.Close()
		s.addWorkspaceLocalRecords(
			agentValue.WorkspacePath,
			workspaceRoot,
			records,
			enabledNames,
			disabled,
		)
	}
	return records, enabledNames, isMainAgent, nil
}

// populateAgentUsageCounts 为全局目录投影每个 Skill 的 Agent 使用数。
//
// 全局使用数只读取 Agent 的 Skill 引用，不从 workspace 文件反推绑定关系。
func (s *Service) populateAgentUsageCounts(
	ctx context.Context,
	records map[string]catalogRecord,
) error {
	for name, record := range records {
		record.Detail.EnabledAgentCount = 0
		records[name] = record
	}
	if s.agents == nil {
		return nil
	}
	agents, err := s.agents.ListAgentRecords(ctx)
	if err != nil {
		return err
	}
	counts := map[string]int{}
	for index := range agents {
		agentValue := &agents[index]
		enabled := enabledSkillNames(agentValue, records)
		seen := map[string]struct{}{}
		for name := range enabled {
			record, ok := findCatalogRecord(records, name)
			if !ok {
				continue
			}
			key := strings.ToLower(strings.TrimSpace(record.Detail.Name))
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			counts[key]++
		}
	}
	for name, record := range records {
		record.Detail.EnabledAgentCount = counts[strings.ToLower(strings.TrimSpace(name))]
		records[name] = record
	}
	return nil
}

func (s *Service) addWorkspaceLocalRecords(
	workspacePath string,
	workspaceRoot *confinedfs.Root,
	records map[string]catalogRecord,
	enabledNames map[string]bool,
	disabled map[string]struct{},
) {
	for _, record := range buildWorkspaceRecordsAt(workspacePath, workspaceRoot) {
		name := record.Detail.Name
		_, blocked := disabled[strings.ToLower(strings.TrimSpace(name))]
		replaceCatalogRecord(records, record)
		// 本地来源按名称覆盖全局目录投影；即使全局绑定仍存在，
		// 本地显式停用也必须让 Agent 设置页显示为未启用。
		enabledNames[name] = !blocked
	}
}

// buildWorkspaceRecordsAt 只读取当前 Agent workspace，不把结果写回全局目录。
func buildWorkspaceRecordsAt(
	workspacePath string,
	workspaceRoot *confinedfs.Root,
) []catalogRecord {
	skillDirs := discoverWorkspaceSkillDirsAt(workspaceRoot)
	skillNames := slices.Sorted(maps.Keys(skillDirs))
	records := make([]catalogRecord, 0, len(skillNames))
	for _, skillName := range skillNames {
		record, err := buildWorkspaceRecordAt(
			workspacePath,
			workspaceRoot,
			skillDirs[skillName],
		)
		if err == nil {
			records = append(records, record)
		}
	}
	return records
}

// replaceCatalogRecord 让当前 Agent 的本地同名 Skill 覆盖目录投影。
func replaceCatalogRecord(records map[string]catalogRecord, record catalogRecord) {
	for name := range records {
		if strings.EqualFold(strings.TrimSpace(name), strings.TrimSpace(record.Detail.Name)) {
			delete(records, name)
		}
	}
	records[record.Detail.Name] = record
}

func discoverWorkspaceSkillDirsAt(
	confinedRoot *confinedfs.Root,
) map[string]string {
	result := map[string]string{}
	addSkillDirs := func(parent string) {
		parentRoot, err := confinedRoot.OpenRootNoSymlink(parent)
		if err != nil {
			return
		}
		defer parentRoot.Close()
		entries, err := fs.ReadDir(parentRoot.FS(), ".")
		if err != nil {
			return
		}
		for _, entry := range entries {
			info, statErr := parentRoot.Lstat(entry.Name())
			if statErr != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
				continue
			}
			skillRoot, openErr := parentRoot.OpenRootNoSymlink(entry.Name())
			if openErr != nil {
				continue
			}
			skillFile, openErr := skillRoot.OpenFileNoSymlink("SKILL.md", os.O_RDONLY, 0)
			skillRoot.Close()
			if openErr != nil {
				continue
			}
			skillFile.Close()
			skillDir := filepath.ToSlash(filepath.Join(parent, entry.Name()))
			if _, exists := result[entry.Name()]; !exists {
				result[entry.Name()] = skillDir
			}
		}
	}
	addSkillDirs(".agents/skills")
	addSkillDirs(".agents")
	addSkillDirs(".claude/skills")
	return result
}

func (s *Service) ensureAgent(ctx context.Context, agentID string) (*protocol.Agent, error) {
	if s.workspaces == nil {
		return nil, errors.New("workspace service 未初始化")
	}
	agentValue, err := s.workspaces.EnsureAgentWorkspace(
		ctx,
		strings.TrimSpace(agentID),
	)
	if err != nil {
		return nil, err
	}
	return agentValue, nil
}

// agentsReferencingSkill 返回正在引用 owner 共享源的 Agent。
//
// 外部 Skill 本身只保存一份 owner 源，更新源目录后所有引用会自然看到新内容；
// 这里仅返回受影响的 Agent，供 API 展示同步结果。
func (s *Service) agentsReferencingSkill(ctx context.Context, skillName string) ([]RedeployAgentSuccess, error) {
	if s.agents == nil {
		return []RedeployAgentSuccess{}, nil
	}
	agents, err := s.agents.ListAgentRecords(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]RedeployAgentSuccess, 0)
	for _, agentValue := range agents {
		referenced := false
		for _, reference := range agentValue.Options.SkillIDs {
			if skillReferenceMatches(reference, skillName) {
				referenced = true
				break
			}
		}
		if !referenced {
			continue
		}
		result = append(result, RedeployAgentSuccess{
			AgentID:   agentValue.AgentID,
			AgentName: agentValue.Name,
		})
	}
	return result, nil
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
	platformRoot := filepath.Clean(filepath.Join(projectRoot(), "skills"))
	for _, root := range builtinSearchRootsForContext(ctx, projectRoot(), s.config.AppMode) {
		entries, err := readConfinedDirectoryEntries(root)
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			skillName := entry.Name()
			if containsSkillName(systemSkillNames, skillName) {
				continue
			}
			if containsSkillName(internalSkillNames, skillName) {
				continue
			}
			if catalogHasSkillName(records, skillName) {
				continue
			}
			sourceKind := builtinSourceKind(root, platformRoot)
			record, buildErr := s.buildBuiltinRecord(filepath.Join(root, skillName), curatedEntries[skillName], sourceKind)
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
		if catalogHasSkillName(records, name) {
			// 平台/系统源优先，避免历史外部同名 Skill 在产品升级后覆盖官方源。
			continue
		}
		records[name] = record
	}
	return records, nil
}

func catalogHasSkillName(records map[string]catalogRecord, name string) bool {
	for existingName := range records {
		if strings.EqualFold(strings.TrimSpace(existingName), strings.TrimSpace(name)) {
			return true
		}
	}
	return false
}

func builtinSourceKind(root string, platformRoot string) string {
	if filepath.Clean(root) == filepath.Clean(platformRoot) {
		return sourceKindNexusPlatform
	}
	return sourceKindUserGlobal
}

func builtinStorageScope(sourceKind string) string {
	if sourceKind == sourceKindNexusPlatform {
		return storageScopePlatform
	}
	return storageScopeUserGlobal
}

func builtinOriginKind(sourceKind string) string {
	if sourceKind == sourceKindNexusPlatform {
		return originKindBuiltin
	}
	return originKindUserImport
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
			StorageScope: storageScopePlatform,
			OriginKind:   originKindBuiltin,
		},
		ReadmeMarkdown: parsed.ReadmeMarkdown,
		Recommendation: "系统内置能力，启用状态由平台托管。",
	}
	return catalogRecord{Detail: detail, SourcePath: sourceDir}, nil
}

func (s *Service) buildBuiltinRecord(sourceDir string, curated map[string]string, sourceKind string) (catalogRecord, error) {
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
			SourceKind:   sourceKind,
			StorageScope: builtinStorageScope(sourceKind),
			OriginKind:   builtinOriginKind(sourceKind),
		},
		ReadmeMarkdown: parsed.ReadmeMarkdown,
		Recommendation: firstNonEmpty(curated["recommendation"], parsed.Recommendation, "自动收录的本地可用能力。"),
	}
	return catalogRecord{Detail: detail, SourcePath: sourceDir}, nil
}

func buildWorkspaceRecordAt(
	workspacePath string,
	root *confinedfs.Root,
	relativeSourceDir string,
) (catalogRecord, error) {
	contentBytes, err := root.ReadFile(filepath.ToSlash(filepath.Join(relativeSourceDir, "SKILL.md")))
	if err != nil {
		return catalogRecord{}, err
	}
	content := string(contentBytes)
	skillName := filepath.Base(relativeSourceDir)
	sourceDir := filepath.Join(workspacePath, filepath.FromSlash(relativeSourceDir))
	parsed := parseSkillFrontmatter(content, skillName)
	categoryKey := firstNonEmpty(parsed.CategoryKey, "agent-workspace")
	categoryName := firstNonEmpty(parsed.CategoryName, "智能体工作区")
	recommendation := firstNonEmpty(parsed.Recommendation, "当前 Agent 工作区的本地 Skill，仅对当前 Agent 可见并默认启用。")
	detail := Detail{
		Info: Info{
			Name:         parsed.Name,
			Title:        firstNonEmpty(parsed.Title, parsed.Name),
			Description:  parsed.Description,
			Scope:        defaultSkillScope(parsed.Scope),
			Tags:         parsed.Tags,
			CategoryKey:  categoryKey,
			CategoryName: categoryName,
			SourceType:   sourceTypeWorkspace,
			SourceRef:    sourceDir,
			Version:      firstNonEmpty(parsed.Version, "workspace"),
			Locked:       false,
			Deletable:    true,
			StorageScope: storageScopeAgent,
			OriginKind:   originKindAgentCreated,
		},
		ReadmeMarkdown: parsed.ReadmeMarkdown,
		Recommendation: recommendation,
	}
	return catalogRecord{Detail: detail, SourcePath: sourceDir}, nil
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
		result[name] = maps.Clone(metadata)
	}
	return result
}

func builtinSearchRoots(root string) []string {
	roots := []string{filepath.Join(root, "skills")}
	if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
		roots = append(roots, filepath.Join(home, ".agents", "skills"))
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

// builtinSearchRootsForContext 按部署模式限制 ~/.agents/skills 的扫描边界。
//
// 桌面模式的 workspace 与本机用户一一对应，可以读取标准 Agent Skill 目录；
// 多用户服务只允许平台源，避免把服务进程的宿主文件暴露给登录用户。
func builtinSearchRootsForContext(ctx context.Context, root string, appMode string) []string {
	if state, ok := authctx.StateFromContext(ctx); ok &&
		state.AuthRequired &&
		!strings.EqualFold(strings.TrimSpace(appMode), "desktop") {
		return []string{filepath.Join(root, "skills")}
	}
	return builtinSearchRoots(root)
}
