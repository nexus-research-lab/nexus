package workspace

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"
)

var (
	baseSkillNames      = []string{"imagegen", "goal-manager"}
	mainAgentSkillNames = []string{"nexus-manager"}
	createSymlink       = os.Symlink
)

// BuildSkillRenderContext 构建 skill 模板渲染上下文。
func BuildSkillRenderContext(agentID string, agentName string, workspacePath string, createdAt time.Time) map[string]string {
	return buildTemplateContext(agentID, agentName, workspacePath, createdAt)
}

// DeploySkill 把指定 skill 部署到目标 workspace。
func DeploySkill(skillName string, sourceDir string, workspacePath string, context map[string]string) error {
	agentsSkillDir := filepath.Join(workspacePath, ".agents", "skills", skillName)
	legacySkillEntry := filepath.Join(workspacePath, ".claude", "skills", skillName)
	if err := syncDirectory(sourceDir, agentsSkillDir, context); err != nil {
		return err
	}
	return ensureLegacySkillEntry(sourceDir, legacySkillEntry, filepath.Join("..", "..", ".agents", "skills", skillName), context)
}

// UndeploySkill 从 workspace 中移除指定 skill。
func UndeploySkill(workspacePath string, skillName string) error {
	targetDir := filepath.Join(workspacePath, ".agents", "skills", skillName)
	legacySkillEntry := filepath.Join(workspacePath, ".claude", "skills", skillName)
	if err := os.RemoveAll(targetDir); err != nil {
		return err
	}
	if err := os.RemoveAll(legacySkillEntry); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// ListDeployedSkills 返回 workspace 当前已部署的全部 skill。
func ListDeployedSkills(workspacePath string) ([]string, error) {
	type skillParent struct {
		path             string
		requireSkillFile bool
	}
	parents := []skillParent{
		// 旧版迁移用空目录标记已安装状态，不能在读取时丢失这类记录。
		{path: filepath.Join(workspacePath, ".agents", "skills")},
		{path: filepath.Join(workspacePath, ".agents"), requireSkillFile: true},
		{path: filepath.Join(workspacePath, ".claude", "skills")},
	}
	result := []string{}
	seen := map[string]struct{}{}
	for _, parent := range parents {
		entries, err := os.ReadDir(parent.path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			skillDir := filepath.Join(parent.path, entry.Name())
			info, err := os.Stat(skillDir)
			if err != nil || !info.IsDir() {
				continue
			}
			if parent.requireSkillFile {
				if _, err = os.Stat(filepath.Join(skillDir, "SKILL.md")); err != nil {
					continue
				}
			}
			key := strings.ToLower(strings.TrimSpace(entry.Name()))
			if key == "" {
				continue
			}
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			result = append(result, entry.Name())
		}
	}
	sort.Strings(result)
	return result, nil
}

// RuntimeSkillNames 合并平台选择与 workspace 已部署 Skill，形成运行时白名单。
//
// 平台 Skill 只保存稳定 ID；外部和 workspace-local Skill 仍以文件部署，因此
// 必须在启动 runtime 时补入名称，避免显式平台白名单把旧机制中的 Skill 过滤掉。
func RuntimeSkillNames(workspacePath string, selectedSkillIDs []string) ([]string, error) {
	result := slices.Clone(selectedSkillIDs)
	seen := make(map[string]struct{}, len(result))
	for _, name := range result {
		if normalized := strings.ToLower(strings.TrimSpace(name)); normalized != "" {
			seen[normalized] = struct{}{}
		}
	}
	deployedNames, err := ListDeployedSkills(workspacePath)
	if err != nil {
		return nil, err
	}
	for _, name := range deployedNames {
		key := strings.ToLower(strings.TrimSpace(name))
		if key == "" {
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, name)
	}
	return result, nil
}

func managedSkillNames(isMainAgent bool) []string {
	// 这些名称仅用于清理旧版 workspace 副本，平台库本身不再按 Agent 部署。
	items := slices.Clone(baseSkillNames)
	if isMainAgent {
		items = append(items, mainAgentSkillNames...)
	}
	// 产品新增平台 Skill 后，旧版本可能已经在 workspace 留下同名副本；
	// 按产品源目录动态补入名称，避免迁移遗漏新 Skill。
	productSkillsRoot := filepath.Join(projectRoot(), "skills")
	if entries, err := os.ReadDir(productSkillsRoot); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			if _, err := os.Stat(filepath.Join(productSkillsRoot, entry.Name(), "SKILL.md")); err != nil {
				continue
			}
			items = appendSkillNameOnce(items, entry.Name())
		}
	}
	return items
}

func appendSkillNameOnce(items []string, name string) []string {
	key := strings.ToLower(strings.TrimSpace(name))
	if key == "" {
		return items
	}
	for _, current := range items {
		if strings.EqualFold(strings.TrimSpace(current), key) {
			return items
		}
	}
	return append(items, name)
}

func syncDirectory(sourceDir string, targetDir string, context map[string]string) error {
	if err := os.RemoveAll(targetDir); err != nil {
		return err
	}
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return err
	}
	return filepath.WalkDir(sourceDir, func(path string, entry fs.DirEntry, walkErr error) error {
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
		if entry.IsDir() {
			return os.MkdirAll(targetPath, 0o755)
		}
		if err = os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return err
		}
		if filepath.Base(path) == "SKILL.md" {
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			rendered := renderTemplate(string(content), context)
			return os.WriteFile(targetPath, []byte(strings.TrimSpace(rendered)+"\n"), 0o644)
		}
		return copyFile(path, targetPath)
	})
}

func ensureLegacySkillEntry(sourceDir string, entryPath string, relativeTarget string, context map[string]string) error {
	err := ensureRelativeSymlink(entryPath, relativeTarget)
	if err == nil {
		return nil
	}
	// Windows 默认可能没有目录 symlink 权限，失败时镜像一份给旧版 runtime 读取。
	if mirrorErr := syncDirectory(sourceDir, entryPath, context); mirrorErr != nil {
		return fmt.Errorf("创建 legacy skill symlink 失败: %w；镜像目录也失败: %v", err, mirrorErr)
	}
	return nil
}

func ensureRelativeSymlink(linkPath string, relativeTarget string) error {
	if err := os.MkdirAll(filepath.Dir(linkPath), 0o755); err != nil {
		return err
	}
	if current, err := os.Readlink(linkPath); err == nil {
		if current == relativeTarget {
			return nil
		}
		if err = os.Remove(linkPath); err != nil {
			return err
		}
	} else if _, statErr := os.Stat(linkPath); statErr == nil {
		if err = os.RemoveAll(linkPath); err != nil {
			return err
		}
	}
	return createSymlink(relativeTarget, linkPath)
}

func copyFile(sourcePath string, targetPath string) error {
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	targetFile, err := os.Create(targetPath)
	if err != nil {
		return err
	}
	defer targetFile.Close()

	if _, err = io.Copy(targetFile, sourceFile); err != nil {
		return err
	}
	info, err := os.Stat(sourcePath)
	if err != nil {
		return err
	}
	return os.Chmod(targetPath, info.Mode())
}
