// INPUT: 用户身份与 Nexus 全局配置目录。
// OUTPUT: 外部 Skill 的全局兼容入口，以及旧 workspace 副本清理能力。
// POS: 用户级 Skill 源与 Agent workspace 之间的迁移边界。
package workspace

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type externalSkillManifestHeader struct {
	Name       string `json:"name"`
	SourceType string `json:"source_type"`
}

// IsLegacyExternalSkillMarker 判断 workspace 中的 Skill 是否是旧版外部部署痕迹。
//
// 有 manifest 的目录必须明确标记为 external；没有 manifest 时只接受空目录，
// 这样不会把用户后来创建的 workspace-local Skill 当成迁移对象。
func IsLegacyExternalSkillMarker(workspacePath string, skillName string) bool {
	name := strings.TrimSpace(skillName)
	if name == "" {
		return false
	}
	for _, candidate := range []struct {
		root       string
		allowEmpty bool
	}{
		{root: filepath.Join(workspacePath, ".agents", "skills"), allowEmpty: true},
		{root: filepath.Join(workspacePath, ".agents")},
		{root: filepath.Join(workspacePath, ".claude", "skills"), allowEmpty: true},
	} {
		entryPath := filepath.Join(candidate.root, name)
		info, err := os.Stat(entryPath)
		if err != nil || !info.IsDir() {
			continue
		}
		manifestPath := filepath.Join(entryPath, ".nexus-skill.json")
		if payload, readErr := os.ReadFile(manifestPath); readErr == nil {
			var manifest externalSkillManifestHeader
			if json.Unmarshal(payload, &manifest) == nil &&
				strings.EqualFold(strings.TrimSpace(manifest.SourceType), "external") {
				return true
			}
		}
		if candidate.allowEmpty {
			if _, skillErr := os.Stat(filepath.Join(entryPath, "SKILL.md")); os.IsNotExist(skillErr) {
				return true
			}
		}
	}
	return false
}

// EnsureUserSkillLibrary 确保用户外部 Skill 同时具备 nxs 与 Claude 发现入口。
func EnsureUserSkillLibrary(ownerUserID string) error {
	root := appfs.UserSkillLibraryRoot(ownerUserID)
	agentsRoot := filepath.Join(root, ".agents", "skills")
	if err := os.MkdirAll(agentsRoot, 0o755); err != nil {
		return err
	}
	claudeRoot := filepath.Join(root, ".claude", "skills")
	if err := ensureRelativeSymlink(claudeRoot, filepath.Join("..", ".agents", "skills")); err == nil {
		return nil
	} else if mirrorErr := mirrorDirectory(agentsRoot, claudeRoot); mirrorErr != nil {
		return fmt.Errorf("创建用户 Skill Claude 入口失败: %w；镜像目录也失败: %v", err, mirrorErr)
	}
	return nil
}

// EnsureExternalSkillWorkspaceClean 清理已进入用户级源的旧版外部 Skill workspace 副本。
func EnsureExternalSkillWorkspaceClean(ownerUserID string, workspacePath string) error {
	return cleanExternalSkillWorkspace(ownerUserID, workspacePath, "")
}

// EnsureExternalSkillWorkspaceSkillClean 只清理指定的旧版外部 Skill 副本。
func EnsureExternalSkillWorkspaceSkillClean(ownerUserID string, workspacePath string, skillName string) error {
	return cleanExternalSkillWorkspace(ownerUserID, workspacePath, strings.TrimSpace(skillName))
}

func cleanExternalSkillWorkspace(ownerUserID string, workspacePath string, targetName string) error {
	available, err := userExternalSkillNames(ownerUserID)
	if err != nil {
		return err
	}
	for _, root := range []string{
		filepath.Join(workspacePath, ".agents", "skills"),
		filepath.Join(workspacePath, ".claude", "skills"),
	} {
		entries, err := os.ReadDir(root)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		for _, entry := range entries {
			skillDir := filepath.Join(root, entry.Name())
			if info, statErr := os.Stat(skillDir); statErr != nil || !info.IsDir() {
				continue
			}
			manifestPath := filepath.Join(root, entry.Name(), ".nexus-skill.json")
			payload, readErr := os.ReadFile(manifestPath)
			manifestPresent := readErr == nil
			var manifest externalSkillManifestHeader
			if manifestPresent && (json.Unmarshal(payload, &manifest) != nil ||
				!strings.EqualFold(strings.TrimSpace(manifest.SourceType), "external")) {
				manifestPresent = false
			}
			name := strings.TrimSpace(manifest.Name)
			if name == "" {
				name = entry.Name()
			}
			if !manifestPresent {
				// 旧版本可能只留下空目录标记；有 SKILL.md 的无 manifest 目录仍可能是 workspace-local。
				if targetName == "" {
					continue
				}
				if _, skillErr := os.Stat(filepath.Join(skillDir, "SKILL.md")); skillErr == nil {
					continue
				}
			}
			if targetName != "" && !strings.EqualFold(name, targetName) {
				continue
			}
			if _, exists := available[strings.ToLower(name)]; !exists {
				continue
			}
			if err := UndeploySkill(workspacePath, entry.Name()); err != nil {
				return err
			}
		}
	}
	return nil
}

func userExternalSkillNames(ownerUserID string) (map[string]string, error) {
	root := appfs.UserSkillDiscoveryRoot(ownerUserID)
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, err
	}
	result := map[string]string{}
	for _, entry := range entries {
		skillDir := filepath.Join(root, entry.Name())
		if info, statErr := os.Stat(skillDir); statErr != nil || !info.IsDir() {
			continue
		}
		payload, readErr := os.ReadFile(filepath.Join(skillDir, ".nexus-skill.json"))
		if readErr != nil {
			continue
		}
		var manifest externalSkillManifestHeader
		if json.Unmarshal(payload, &manifest) != nil ||
			!strings.EqualFold(strings.TrimSpace(manifest.SourceType), "external") {
			continue
		}
		if _, statErr := os.Stat(filepath.Join(skillDir, "SKILL.md")); statErr != nil {
			continue
		}
		name := strings.TrimSpace(manifest.Name)
		if name == "" {
			name = strings.TrimSpace(entry.Name())
		}
		if name != "" {
			result[strings.ToLower(name)] = name
		}
	}
	return result, nil
}

// ListLegacyExternalSkillNames 返回旧版按 workspace 部署过的外部 Skill。
//
// 旧版本没有把外部 Skill 写入 Agent SkillIDs，只能从部署目录里的 manifest
// 恢复安装关系；调用方应先持久化关系，再调用 EnsureInitialized 清理副本。
func ListLegacyExternalSkillNames(workspacePath string) ([]string, error) {
	seen := map[string]string{}
	for _, root := range []string{
		filepath.Join(workspacePath, ".agents", "skills"),
		filepath.Join(workspacePath, ".claude", "skills"),
	} {
		entries, err := os.ReadDir(root)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			skillDir := filepath.Join(root, entry.Name())
			if info, statErr := os.Stat(skillDir); statErr != nil || !info.IsDir() {
				continue
			}
			payload, readErr := os.ReadFile(filepath.Join(skillDir, ".nexus-skill.json"))
			if readErr != nil {
				continue
			}
			var manifest externalSkillManifestHeader
			if json.Unmarshal(payload, &manifest) != nil ||
				!strings.EqualFold(strings.TrimSpace(manifest.SourceType), "external") {
				continue
			}
			name := strings.TrimSpace(manifest.Name)
			if name == "" {
				name = strings.TrimSpace(entry.Name())
			}
			if name != "" {
				seen[strings.ToLower(name)] = name
			}
		}
	}
	result := make([]string, 0, len(seen))
	for _, name := range seen {
		result = append(result, name)
	}
	sort.Strings(result)
	return result, nil
}

// MergeLegacyExternalSkillReferences 将已有用户级源的旧 workspace 副本转换成引用。
//
// 用户级源不存在时保留旧副本，避免 runtime 路径先于 registry 迁移执行时丢失 Skill。
func MergeLegacyExternalSkillReferences(ownerUserID string, workspacePath string, selected []string) ([]string, bool, error) {
	names, err := ListLegacyExternalSkillNames(workspacePath)
	if err != nil {
		return nil, false, err
	}
	available, err := userExternalSkillNames(ownerUserID)
	if err != nil {
		return nil, false, err
	}
	legacyByName := map[string]string{}
	migratableNames := make([]string, 0, len(names))
	for _, name := range names {
		key := strings.ToLower(strings.TrimSpace(name))
		if canonical, exists := available[key]; exists {
			legacyByName[key] = canonical
			migratableNames = append(migratableNames, canonical)
		}
	}
	result := make([]string, 0, len(selected)+len(names))
	seen := map[string]struct{}{}
	changed := false
	for _, reference := range selected {
		value := strings.TrimSpace(reference)
		if value == "" {
			changed = true
			continue
		}
		canonical := value
		matchName := value
		if externalName, ok := protocol.ParseExternalSkillReference(value); ok {
			canonical = protocol.BuildExternalSkillReference(externalName)
			matchName = externalName
		}
		if legacyName, ok := legacyByName[strings.ToLower(matchName)]; ok {
			canonical = protocol.BuildExternalSkillReference(legacyName)
		}
		if value != canonical {
			changed = true
		}
		key := strings.ToLower(canonical)
		if _, exists := seen[key]; exists {
			changed = true
			continue
		}
		seen[key] = struct{}{}
		result = append(result, canonical)
	}
	for _, name := range migratableNames {
		reference := protocol.BuildExternalSkillReference(name)
		key := strings.ToLower(reference)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, reference)
		changed = true
	}
	return result, changed, nil
}

// ReplaceDirectory 原子替换一个 Skill 源目录，避免 runtime 读到半份文件。
func ReplaceDirectory(sourceRoot string, targetRoot string) error {
	return replaceDirectory(sourceRoot, targetRoot)
}

func mirrorDirectory(sourceRoot string, targetRoot string) error {
	if err := os.MkdirAll(filepath.Dir(targetRoot), 0o755); err != nil {
		return err
	}
	temporaryRoot, err := os.MkdirTemp(filepath.Dir(targetRoot), ".user-skill-mirror-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temporaryRoot)
	if err = filepath.WalkDir(sourceRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(sourceRoot, path)
		if err != nil {
			return err
		}
		if relative == "." {
			return nil
		}
		target := filepath.Join(temporaryRoot, relative)
		if entry.IsDir() {
			info, infoErr := entry.Info()
			if infoErr != nil {
				return infoErr
			}
			if mkdirErr := os.MkdirAll(target, info.Mode().Perm()); mkdirErr != nil {
				return mkdirErr
			}
			return os.Chmod(target, info.Mode().Perm())
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		input, err := os.Open(path)
		if err != nil {
			return err
		}
		output, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
		if err != nil {
			_ = input.Close()
			return err
		}
		_, copyErr := io.Copy(output, input)
		closeInputErr := input.Close()
		closeOutputErr := output.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeInputErr != nil {
			return closeInputErr
		}
		if closeOutputErr != nil {
			return closeOutputErr
		}
		info, infoErr := entry.Info()
		if infoErr != nil {
			return infoErr
		}
		return os.Chmod(target, info.Mode().Perm())
	}); err != nil {
		return err
	}
	return replaceDirectory(temporaryRoot, targetRoot)
}
