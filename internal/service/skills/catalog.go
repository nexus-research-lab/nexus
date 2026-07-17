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

func (s *Service) redeploySkillToInstalledAgents(ctx context.Context, skillName string) (*RedeployResult, error) {
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
	agents, err := s.agents.ListAgentRecords(ctx)
	if err != nil {
		return nil, err
	}
	for index := range agents {
		agentValue := agents[index]
		names, err := workspacesvc.ListDeployedSkills(agentValue.WorkspacePath)
		if err != nil {
			result.Failures = append(result.Failures, RedeployAgentFailure{
				AgentID:   agentValue.AgentID,
				AgentName: agentValue.Name,
				Error:     err.Error(),
			})
			continue
		}
		if !slices.Contains(names, record.Detail.Name) {
			continue
		}
		if err = s.deploySkillToWorkspace(&agentValue, record); err != nil {
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
		ReadmeMarkdown:      parsed.ReadmeMarkdown,
		RuntimeInstructions: parsed.RuntimeInstructions,
		Recommendation:      "系统内置能力，安装状态由平台托管。",
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
		ReadmeMarkdown:      parsed.ReadmeMarkdown,
		RuntimeInstructions: parsed.RuntimeInstructions,
		Recommendation:      firstNonEmpty(curated["recommendation"], parsed.Recommendation, "自动收录的本地可用能力。"),
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
		ReadmeMarkdown:      parsed.ReadmeMarkdown,
		RuntimeInstructions: parsed.RuntimeInstructions,
		Recommendation:      firstNonEmpty(parsed.Recommendation, "仅在该智能体工作区内可用。"),
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
