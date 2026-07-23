package skills

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	workspacesvc "github.com/nexus-research-lab/nexus/internal/service/workspace"
)

type legacyOwnerSkillSet map[string]map[string]string
type legacyOwnerIDMap map[string]string

const legacyRegistryMigrationMarkerName = ".user-skill-library-v1"

func (s *Service) ensureLegacyRegistryMigrated(ctx context.Context) error {
	// TODO(skill-legacy-registry): 这是旧全局 registry 的一次性兼容迁移逻辑，存量数据完成迁移后移除。
	s.legacyRegistryMu.Lock()
	defer s.legacyRegistryMu.Unlock()
	if s.legacyRegistryMigrated {
		return nil
	}
	if s.agents == nil {
		// 没有 Agent 仓储就无法恢复旧 workspace 的安装关系，不能写完成标记。
		return nil
	}
	completed, err := s.legacyRegistryMigrationCompleted()
	if err != nil {
		return err
	}
	if completed {
		s.legacyRegistryMigrated = true
		return nil
	}

	legacyOwnerIDs, err := s.legacyOwnerIDs(ctx)
	if err != nil {
		return err
	}
	legacyOwnerSkills, err := s.loadArchivedLegacyOwnerSkills(legacyOwnerIDs)
	if err != nil {
		return err
	}
	currentOwnerSkills, err := s.migrateLegacyOwnerRegistries(legacyOwnerIDs)
	if err != nil {
		return err
	}
	mergeLegacyOwnerSkills(legacyOwnerSkills, currentOwnerSkills)
	legacyDirs, err := s.findLegacySkillDirs()
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	archivedDirs, err := s.findArchivedLegacySkillDirs()
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if len(legacyDirs) > 0 || len(archivedDirs) > 0 {
		usageOwners, usageErr := s.legacySkillUsageOwners(ctx)
		if usageErr != nil {
			return usageErr
		}
		for skillName, skillDir := range legacyDirs {
			owners := usageOwners[strings.ToLower(skillName)]
			if err = s.migrateLegacySkillDir(skillName, skillDir, owners); err != nil {
				return err
			}
			for ownerUserID := range owners {
				addLegacyOwnerSkill(legacyOwnerSkills, ownerUserID, skillName)
			}
		}
		for skillName := range archivedDirs {
			owners := usageOwners[strings.ToLower(skillName)]
			for ownerUserID := range owners {
				if err = s.copyLegacySkillToOwner(skillName, archivedDirs[skillName], ownerUserID); err != nil {
					return err
				}
				addLegacyOwnerSkill(legacyOwnerSkills, ownerUserID, skillName)
			}
		}
	}
	if err = s.migrateLegacyAgentInstallations(ctx, legacyOwnerSkills); err != nil {
		return err
	}
	pending, err := s.hasUnresolvedLegacyOwnerRegistries(legacyOwnerIDs)
	if err != nil {
		return err
	}
	if pending {
		// 未知 owner 的旧源先保留，等对应 Agent 出现后再继续迁移，避免写完成标记后永久丢失可见性。
		return nil
	}
	if err = s.markLegacyRegistryMigrationCompleted(); err != nil {
		return err
	}
	s.legacyRegistryMigrated = true
	return nil
}

func (s *Service) hasUnresolvedLegacyOwnerRegistries(ownerIDs legacyOwnerIDMap) (bool, error) {
	for _, usersRoot := range []string{
		filepath.Join(s.legacyRegistryBaseRoot(), registryUsersDirName),
		filepath.Join(s.legacyRegistryBaseRoot(), registryLegacyMigratedDirName, registryUsersDirName),
	} {
		entries, err := os.ReadDir(usersRoot)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return false, err
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			if _, known := resolveLegacyOwnerID(entry.Name(), ownerIDs); !known {
				return true, nil
			}
		}
	}
	return false, nil
}

func (s *Service) legacyRegistryMigrationCompleted() (bool, error) {
	_, err := os.Stat(filepath.Join(s.legacyRegistryBaseRoot(), legacyRegistryMigrationMarkerName))
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func (s *Service) markLegacyRegistryMigrationCompleted() error {
	markerPath := filepath.Join(s.legacyRegistryBaseRoot(), legacyRegistryMigrationMarkerName)
	if err := os.MkdirAll(filepath.Dir(markerPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(markerPath, []byte("completed\n"), 0o644)
}

func (s *Service) migrateLegacyAgentInstallations(ctx context.Context, legacyOwnerSkills legacyOwnerSkillSet) error {
	if s.agents == nil || len(legacyOwnerSkills) == 0 {
		return nil
	}
	agents, err := s.agents.ListAllAgentRecordsForMaintenance(ctx)
	if err != nil {
		return err
	}
	for index := range agents {
		agentValue := agents[index]
		ownerSkills := legacyOwnerSkills[strings.TrimSpace(agentValue.OwnerUserID)]
		if len(ownerSkills) == 0 {
			continue
		}
		deployedNames, listErr := workspacesvc.ListDeployedSkills(agentValue.WorkspacePath)
		if listErr != nil {
			return listErr
		}
		selected := agentValue.Options.SkillIDs
		migratedNames := make([]string, 0)
		changed := false
		for _, deployedName := range deployedNames {
			if !workspacesvc.IsLegacyExternalSkillMarker(agentValue.WorkspacePath, deployedName) {
				continue
			}
			skillName, ok := ownerSkills[strings.ToLower(strings.TrimSpace(deployedName))]
			if !ok {
				continue
			}
			sourceDir := filepath.Join(s.registryRootForOwner(agentValue.OwnerUserID), skillName)
			if !isExternalSkillSourceDir(sourceDir) {
				continue
			}
			var referenceChanged bool
			selected, referenceChanged = ensureExternalSkillReference(selected, skillName)
			changed = changed || referenceChanged
			migratedNames = append(migratedNames, deployedName)
		}
		if len(migratedNames) == 0 {
			continue
		}
		agentContext := authctx.WithPrincipal(ctx, &authctx.Principal{
			UserID:     agentValue.OwnerUserID,
			Username:   agentValue.OwnerUserID,
			Role:       authctx.RoleOwner,
			AuthMethod: authctx.AuthMethodLocal,
		})
		if changed {
			options := agentValue.Options
			options.SkillIDs = selected
			if _, err = s.agents.UpdateAgent(agentContext, agentValue.AgentID, protocol.UpdateRequest{Options: &options}); err != nil {
				return err
			}
		}
		for _, deployedName := range migratedNames {
			if err = workspacesvc.UndeploySkill(agentValue.WorkspacePath, deployedName); err != nil {
				return err
			}
		}
	}
	return nil
}

func isExternalSkillSourceDir(sourceDir string) bool {
	for _, fileName := range []string{"SKILL.md", ".nexus-skill.json"} {
		if info, err := os.Stat(filepath.Join(sourceDir, fileName)); err != nil || info.IsDir() {
			return false
		}
	}
	manifest, err := os.ReadFile(filepath.Join(sourceDir, ".nexus-skill.json"))
	if err != nil {
		return false
	}
	var header struct {
		SourceType string `json:"source_type"`
	}
	if json.Unmarshal(manifest, &header) != nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(header.SourceType), sourceTypeExternal)
}

func (s *Service) findLegacySkillDirs() (map[string]string, error) {
	return findLegacySkillDirsAt(s.legacyRegistryBaseRoot())
}

func (s *Service) findArchivedLegacySkillDirs() (map[string]string, error) {
	return findLegacySkillDirsAt(filepath.Join(s.legacyRegistryBaseRoot(), registryLegacyMigratedDirName))
}

func findLegacySkillDirsAt(baseRoot string) (map[string]string, error) {
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

func (s *Service) loadArchivedLegacyOwnerSkills(ownerIDs legacyOwnerIDMap) (legacyOwnerSkillSet, error) {
	result := legacyOwnerSkillSet{}
	usersRoot := filepath.Join(s.legacyRegistryBaseRoot(), registryLegacyMigratedDirName, registryUsersDirName)
	ownerEntries, err := os.ReadDir(usersRoot)
	if os.IsNotExist(err) {
		return result, nil
	}
	if err != nil {
		return nil, err
	}
	for _, ownerEntry := range ownerEntries {
		if !ownerEntry.IsDir() {
			continue
		}
		ownerUserID, knownOwner := resolveLegacyOwnerID(ownerEntry.Name(), ownerIDs)
		if !knownOwner {
			continue
		}
		skillEntries, readErr := os.ReadDir(filepath.Join(usersRoot, ownerEntry.Name()))
		if readErr != nil {
			return nil, readErr
		}
		for _, skillEntry := range skillEntries {
			if !skillEntry.IsDir() {
				continue
			}
			sourceDir := filepath.Join(usersRoot, ownerEntry.Name(), skillEntry.Name())
			if !isExternalSkillSourceDir(sourceDir) {
				continue
			}
			content, _, fallbackName, sourceErr := readSkillSource(sourceDir)
			if sourceErr != nil {
				continue
			}
			parsed := parseSkillFrontmatter(content, fallbackName)
			skillName := firstNonEmpty(parsed.Name, fallbackName, skillEntry.Name())
			if validateSkillName(skillName) == nil {
				if err = s.copyLegacySkillToOwner(skillName, sourceDir, ownerUserID); err != nil {
					return nil, err
				}
				addLegacyOwnerSkill(result, ownerUserID, skillName)
			}
		}
	}
	return result, nil
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
	if err := workspacesvc.EnsureUserSkillLibrary(ownerUserID); err != nil {
		return err
	}
	_, err := os.Stat(targetDir)
	if err == nil {
		return nil
	}
	if !os.IsNotExist(err) {
		return err
	}
	if err = copyDirectory(skillDir, targetDir); err != nil {
		return err
	}
	return workspacesvc.EnsureUserSkillLibrary(ownerUserID)
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
			if !workspacesvc.IsLegacyExternalSkillMarker(agentValue.WorkspacePath, normalizedName) {
				continue
			}
			key := strings.ToLower(normalizedName)
			if _, ok := result[key]; !ok {
				result[key] = map[string]struct{}{}
			}
			result[key][ownerUserID] = struct{}{}
		}
	}
	return result, nil
}

func (s *Service) archiveLegacySkillDir(skillName string, sourceDir string, bucket string) error {
	targetDir := filepath.Join(s.legacyRegistryBaseRoot(), bucket, skillName)
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

func (s *Service) migrateLegacyOwnerRegistries(ownerIDs legacyOwnerIDMap) (legacyOwnerSkillSet, error) {
	result := legacyOwnerSkillSet{}
	usersRoot := filepath.Join(s.legacyRegistryBaseRoot(), registryUsersDirName)
	entries, err := os.ReadDir(usersRoot)
	if os.IsNotExist(err) {
		return result, nil
	}
	if err != nil {
		return nil, err
	}
	for _, ownerEntry := range entries {
		if !ownerEntry.IsDir() {
			continue
		}
		ownerUserID, knownOwner := resolveLegacyOwnerID(ownerEntry.Name(), ownerIDs)
		if !knownOwner {
			continue
		}
		ownerSegment := ownerEntry.Name()
		legacyOwnerRoot := filepath.Join(usersRoot, ownerSegment)
		skillEntries, readErr := os.ReadDir(legacyOwnerRoot)
		if readErr != nil {
			return nil, readErr
		}
		migrated := false
		for _, skillEntry := range skillEntries {
			if !skillEntry.IsDir() {
				continue
			}
			skillDir := filepath.Join(legacyOwnerRoot, skillEntry.Name())
			if !isExternalSkillSourceDir(skillDir) {
				continue
			}
			content, _, fallbackName, sourceErr := readSkillSource(skillDir)
			if sourceErr != nil {
				continue
			}
			parsed := parseSkillFrontmatter(content, fallbackName)
			skillName := firstNonEmpty(parsed.Name, fallbackName, skillEntry.Name())
			if validateSkillName(skillName) != nil {
				continue
			}
			targetDir := filepath.Join(s.registryRootForOwner(ownerUserID), skillName)
			if err = workspacesvc.EnsureUserSkillLibrary(ownerUserID); err != nil {
				return nil, err
			}
			if _, statErr := os.Stat(targetDir); os.IsNotExist(statErr) {
				if err = copyDirectory(skillDir, targetDir); err != nil {
					return nil, err
				}
				if err = workspacesvc.EnsureUserSkillLibrary(ownerUserID); err != nil {
					return nil, err
				}
			} else if statErr != nil {
				return nil, statErr
			}
			addLegacyOwnerSkill(result, ownerUserID, skillName)
			migrated = true
		}
		if !migrated {
			continue
		}
		archiveRoot := filepath.Join(s.legacyRegistryBaseRoot(), registryLegacyMigratedDirName, registryUsersDirName, ownerSegment)
		if err = os.MkdirAll(filepath.Dir(archiveRoot), 0o755); err != nil {
			return nil, err
		}
		if err = os.RemoveAll(archiveRoot); err != nil {
			return nil, err
		}
		if err = os.Rename(legacyOwnerRoot, archiveRoot); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (s *Service) legacyOwnerIDs(ctx context.Context) (legacyOwnerIDMap, error) {
	result := legacyOwnerIDMap{}
	if s.agents == nil {
		return result, nil
	}
	agents, err := s.agents.ListAllAgentRecordsForMaintenance(ctx)
	if err != nil {
		return nil, err
	}
	for _, agentValue := range agents {
		ownerUserID := strings.TrimSpace(agentValue.OwnerUserID)
		if ownerUserID == "" {
			continue
		}
		result[ownerUserID] = ownerUserID
		result[legacyOwnerSegment(ownerUserID)] = ownerUserID
	}
	return result, nil
}

func resolveLegacyOwnerID(segment string, ownerIDs legacyOwnerIDMap) (string, bool) {
	trimmed := strings.TrimSpace(segment)
	if ownerUserID, ok := ownerIDs[trimmed]; ok {
		return ownerUserID, true
	}
	return "", false
}

func legacyOwnerSegment(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return authctx.SystemUserID
	}
	var builder strings.Builder
	for _, character := range trimmed {
		switch {
		case character >= 'a' && character <= 'z', character >= 'A' && character <= 'Z', character >= '0' && character <= '9':
			builder.WriteRune(character)
		case character == '-', character == '_', character == '.', character == '@':
			builder.WriteRune(character)
		default:
			builder.WriteRune('_')
		}
	}
	if builder.Len() == 0 {
		return authctx.SystemUserID
	}
	return builder.String()
}

func addLegacyOwnerSkill(result legacyOwnerSkillSet, ownerUserID string, skillName string) {
	owner := strings.TrimSpace(ownerUserID)
	name := strings.TrimSpace(skillName)
	if owner == "" || name == "" {
		return
	}
	if result[owner] == nil {
		result[owner] = map[string]string{}
	}
	result[owner][strings.ToLower(name)] = name
}

func mergeLegacyOwnerSkills(target legacyOwnerSkillSet, source legacyOwnerSkillSet) {
	for ownerUserID, skills := range source {
		for _, skillName := range skills {
			addLegacyOwnerSkill(target, ownerUserID, skillName)
		}
	}
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
	if !strings.EqualFold(strings.TrimSpace(manifest.SourceType), sourceTypeExternal) {
		return "", false
	}
	content, _, fallbackName, err := readSkillSource(skillDir)
	if err != nil {
		return "", false
	}
	parsed := parseSkillFrontmatter(content, fallbackName)
	skillName := firstNonEmpty(manifest.Name, parsed.Name, fallbackName)
	if validateSkillName(skillName) != nil {
		return "", false
	}
	return skillName, skillName != ""
}

func isReservedRegistryDir(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case registryUsersDirName, registryLegacyMigratedDirName, registryLegacyUnassignedDirName, legacyRegistryMigrationMarkerName:
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
