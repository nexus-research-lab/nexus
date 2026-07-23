package skills

import (
	"context"
	"encoding/json"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	workspacesvc "github.com/nexus-research-lab/nexus/internal/service/workspace"
)

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
		agentValue, err = normalizeAgentSkillReferences(ctx, agentValue, records, s.agents)
		if err != nil {
			return nil, nil, false, err
		}
		installedNames = installedSkillNames(agentValue, records)
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
	skillNames := slices.Sorted(maps.Keys(skillDirs))
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
	addSkillDirs(filepath.Join(root, ".claude", "skills"))
	return result
}

func (s *Service) ensureAgent(ctx context.Context, agentID string) (*protocol.Agent, error) {
	if err := workspacesvc.EnsurePlatformSkillLibrary(); err != nil {
		return nil, err
	}
	agentValue, err := s.agents.GetAgent(ctx, strings.TrimSpace(agentID))
	if err != nil {
		return nil, err
	}
	if err = workspacesvc.EnsureUserSkillLibrary(agentValue.OwnerUserID); err != nil {
		return nil, err
	}
	selected, changed, err := workspacesvc.MergeLegacyExternalSkillReferences(
		agentValue.OwnerUserID,
		agentValue.WorkspacePath,
		agentValue.Options.SkillIDs,
	)
	if err != nil {
		return nil, err
	}
	if changed {
		options := agentValue.Options
		options.SkillIDs = selected
		agentValue, err = s.agents.UpdateAgent(ctx, agentValue.AgentID, protocol.UpdateRequest{Options: &options})
		if err != nil {
			return nil, err
		}
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
	if err = workspacesvc.EnsureExternalSkillWorkspaceClean(agentValue.OwnerUserID, agentValue.WorkspacePath); err != nil {
		return nil, err
	}
	return agentValue, nil
}

func (s *Service) deploySkillToWorkspace(agentValue *protocol.Agent, record catalogRecord) error {
	context := workspacesvc.BuildSkillRenderContext(agentValue.AgentID, agentValue.Name, agentValue.WorkspacePath, agentValue.CreatedAt)
	return workspacesvc.DeploySkill(record.Detail.Name, record.SourcePath, agentValue.WorkspacePath, context)
}

// refreshSkillForInstalledAgents 将旧 workspace 副本迁移到用户级引用，并清理副本。
// 新版本不再把更新内容逐 Agent 复制，所有 runtime 直接读取同一个用户源。
func (s *Service) refreshSkillForInstalledAgents(ctx context.Context, skillName string) (*RedeployResult, error) {
	result := &RedeployResult{
		SuccessAgents: make([]RedeployAgentSuccess, 0),
		Failures:      make([]RedeployAgentFailure, 0),
	}
	if s.agents == nil {
		return result, nil
	}
	records, err := s.loadCatalogRecords(ctx)
	if err != nil {
		return nil, err
	}
	record, ok := records[strings.TrimSpace(skillName)]
	if !ok {
		return nil, errors.New("skill not found")
	}
	if record.Detail.SourceType != sourceTypeExternal {
		return result, nil
	}
	agents, err := s.agents.ListAgentRecords(ctx)
	if err != nil {
		return nil, err
	}
	for index := range agents {
		agentValue := agents[index]
		legacyNames, err := workspacesvc.ListLegacyExternalSkillNames(agentValue.WorkspacePath)
		if err != nil {
			result.Failures = append(result.Failures, RedeployAgentFailure{
				AgentID:   agentValue.AgentID,
				AgentName: agentValue.Name,
				Error:     err.Error(),
			})
			continue
		}
		installed := false
		for _, reference := range agentValue.Options.SkillIDs {
			if skillReferenceMatches(reference, record.Detail.Name) {
				installed = true
				break
			}
		}
		if !installed {
			for _, legacyName := range legacyNames {
				if strings.EqualFold(strings.TrimSpace(legacyName), record.Detail.Name) {
					installed = true
					break
				}
			}
		}
		if !installed {
			continue
		}
		selected, changed := ensureExternalSkillReference(agentValue.Options.SkillIDs, record.Detail.Name)
		if changed {
			options := agentValue.Options
			options.SkillIDs = selected
			if _, err = s.agents.UpdateAgent(ctx, agentValue.AgentID, protocol.UpdateRequest{Options: &options}); err != nil {
				result.Failures = append(result.Failures, RedeployAgentFailure{
					AgentID:   agentValue.AgentID,
					AgentName: agentValue.Name,
					Error:     err.Error(),
				})
				continue
			}
		}
		if err = workspacesvc.EnsureExternalSkillWorkspaceSkillClean(agentValue.OwnerUserID, agentValue.WorkspacePath, record.Detail.Name); err != nil {
			result.Failures = append(result.Failures, RedeployAgentFailure{
				AgentID:   agentValue.AgentID,
				AgentName: agentValue.Name,
				Error:     err.Error(),
			})
			continue
		}
		result.SuccessAgents = append(result.SuccessAgents, RedeployAgentSuccess{
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
