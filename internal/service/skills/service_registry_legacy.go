package skills

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"

	workspacesvc "github.com/nexus-research-lab/nexus/internal/service/workspace"
)

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
	slices.Sort(result)
	return result
}
