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

	legacyDirs, err := s.findLegacySkillDirs()
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if len(legacyDirs) == 0 {
		return nil
	}
	usageOwners, err := s.legacySkillUsageOwners(ctx)
	if err != nil {
		return err
	}
	for skillName, skillDir := range legacyDirs {
		if err = s.migrateLegacySkillDir(skillName, skillDir, usageOwners[skillName]); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) findLegacySkillDirs() (map[string]string, error) {
	baseRoot := s.registryBaseRoot()
	entries, err := os.ReadDir(baseRoot)
	if err != nil {
		return nil, err
	}
	legacyDirs := make(map[string]string)
	for _, entry := range entries {
		if !entry.IsDir() || isReservedRegistryDir(entry.Name()) {
			continue
		}
		skillDir := filepath.Join(baseRoot, entry.Name())
		if skillName, ok := legacyExternalSkillName(skillDir); ok {
			legacyDirs[skillName] = skillDir
		}
	}
	return legacyDirs, nil
}

func (s *Service) migrateLegacySkillDir(
	skillName string,
	skillDir string,
	ownerSet map[string]struct{},
) error {
	owners := sortedOwnerSet(ownerSet)
	if len(owners) == 0 {
		return s.archiveLegacySkillDir(skillName, skillDir, registryLegacyUnassignedDirName)
	}
	for _, ownerUserID := range owners {
		if err := s.copyLegacySkillToOwner(skillName, skillDir, ownerUserID); err != nil {
			return err
		}
	}
	return s.archiveLegacySkillDir(skillName, skillDir, registryLegacyMigratedDirName)
}

func (s *Service) copyLegacySkillToOwner(skillName string, skillDir string, ownerUserID string) error {
	targetDir := filepath.Join(s.registryRootForOwner(ownerUserID), skillName)
	_, err := os.Stat(targetDir)
	if err == nil {
		return nil
	}
	if !os.IsNotExist(err) {
		return err
	}
	return copyDirectory(skillDir, targetDir)
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
