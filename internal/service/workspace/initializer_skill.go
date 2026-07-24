package workspace

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

var (
	baseSkillNames      = []string{"imagegen", "goal-manager"}
	mainAgentSkillNames = []string{"nexus-manager"}
	// createSymlink 仅作为平台能力探针；真正的创建由 confinedfs.Root.Symlink 完成。
	createSymlink = func(string, string) error { return nil }
)

// BuildSkillRenderContext 构建 skill 模板渲染上下文。
func BuildSkillRenderContext(agentID string, agentName string, workspacePath string, createdAt time.Time) map[string]string {
	return buildTemplateContext(agentID, agentName, workspacePath, createdAt)
}

// DeploySkill 把指定 skill 部署到目标 workspace。
func DeploySkill(skillName string, sourceDir string, workspacePath string, context map[string]string) error {
	if err := validateWorkspaceSkillName(skillName); err != nil {
		return err
	}
	agentsSkillDir := filepath.Join(workspacePath, ".agents", "skills", skillName)
	claudeSkillEntry := filepath.Join(workspacePath, ".claude", "skills", skillName)
	if err := syncDirectory(sourceDir, workspacePath, agentsSkillDir, context); err != nil {
		return err
	}
	return ensureClaudeSkillEntry(
		sourceDir,
		workspacePath,
		claudeSkillEntry,
		filepath.Join("..", "..", ".agents", "skills", skillName),
		context,
	)
}

// UndeploySkill 从 workspace 中移除指定 skill。
func UndeploySkill(workspacePath string, skillName string) error {
	if err := validateWorkspaceSkillName(skillName); err != nil {
		return err
	}
	if err := removeWorkspaceEntry(workspacePath, filepath.Join(".agents", "skills", skillName)); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := removeWorkspaceEntry(workspacePath, filepath.Join(".claude", "skills", skillName)); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func validateWorkspaceSkillName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, `/\`+"\x00") {
		return errors.New("skill name contains an invalid path segment")
	}
	return nil
}

// ListDeployedSkills 返回 workspace 当前已部署的全部 skill。
func ListDeployedSkills(workspacePath string) ([]string, error) {
	type skillParent struct {
		relativePath     string
		requireSkillFile bool
	}
	parents := []skillParent{
		// Claude 兼容入口可能是普通镜像目录，不能只依赖 .agents/skills。
		{relativePath: ".agents/skills"},
		{relativePath: ".agents", requireSkillFile: true},
		{relativePath: ".claude/skills"},
	}
	root, err := confinedfs.Open(workspacePath)
	if os.IsNotExist(err) {
		return []string{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer root.Close()
	result := []string{}
	seen := map[string]struct{}{}
	for _, parent := range parents {
		entries, err := fs.ReadDir(root.FS(), parent.relativePath)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			skillDir := filepath.ToSlash(filepath.Join(parent.relativePath, entry.Name()))
			info, err := root.Stat(skillDir)
			if err != nil || !info.IsDir() {
				continue
			}
			if parent.requireSkillFile {
				if _, err = root.Stat(filepath.ToSlash(filepath.Join(skillDir, "SKILL.md"))); err != nil {
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

// RuntimeSkillNames 合并 Agent 引用与 workspace-local Skill，形成运行时白名单。
//
// 外部引用以 external:<name> 形式持久化，进入 SDK 前还原为 canonical name；
// workspace-local Skill 仍从 workspace 文件发现，避免显式白名单把它过滤掉。
func RuntimeSkillNames(workspacePath string, selectedSkillIDs []string) ([]string, error) {
	result := make([]string, 0, len(selectedSkillIDs))
	seen := make(map[string]struct{}, len(result))
	for _, reference := range selectedSkillIDs {
		name := reference
		if externalName, ok := protocol.ParseExternalSkillReference(reference); ok {
			name = externalName
		}
		if normalized := strings.ToLower(strings.TrimSpace(name)); normalized != "" {
			if _, exists := seen[normalized]; exists {
				continue
			}
			result = append(result, name)
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
	// 这些名称用于确保平台 Skill 不会落入 Agent workspace。
	items := slices.Clone(baseSkillNames)
	if isMainAgent {
		items = append(items, mainAgentSkillNames...)
	}
	// 产品新增平台 Skill 后，按产品源目录动态补入名称，保持清理集合完整。
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

func syncDirectory(
	sourceDir string,
	boundaryRoot string,
	targetDir string,
	context map[string]string,
) error {
	if err := os.MkdirAll(boundaryRoot, workspaceDirectoryMode()); err != nil {
		return err
	}
	root, err := confinedfs.Open(boundaryRoot)
	if err != nil {
		return err
	}
	defer root.Close()
	relativeTarget, err := relativePathWithin(boundaryRoot, targetDir)
	if err != nil {
		return err
	}
	if err = root.RemoveAll(relativeTarget); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err = root.MkdirAll(relativeTarget, workspaceDirectoryMode()); err != nil {
		return err
	}
	targetRoot, err := root.OpenRoot(relativeTarget)
	if err != nil {
		return err
	}
	defer targetRoot.Close()
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
		relativePath = filepath.ToSlash(relativePath)
		if entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		if entry.IsDir() {
			return targetRoot.MkdirAll(relativePath, workspaceDirectoryMode())
		}
		if err = targetRoot.MkdirAll(filepath.Dir(relativePath), workspaceDirectoryMode()); err != nil {
			return err
		}
		if filepath.Base(path) == "SKILL.md" {
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			rendered := renderTemplate(string(content), context)
			return targetRoot.WriteFileAtomic(relativePath, []byte(strings.TrimSpace(rendered)+"\n"), workspaceFileMode())
		}
		return copyFileToRoot(path, targetRoot, relativePath)
	})
}

func ensureClaudeSkillEntry(
	sourceDir string,
	workspacePath string,
	entryPath string,
	relativeTarget string,
	context map[string]string,
) error {
	err := ensureRelativeSymlink(workspacePath, entryPath, relativeTarget)
	if err == nil {
		return nil
	}
	// Windows 默认可能没有目录 symlink 权限，失败时镜像一份给 Claude 读取。
	if mirrorErr := syncDirectory(sourceDir, workspacePath, entryPath, context); mirrorErr != nil {
		return fmt.Errorf("创建 Claude Skill 入口失败: %w；镜像目录也失败: %v", err, mirrorErr)
	}
	return nil
}

func ensureRelativeSymlink(rootPath string, linkPath string, relativeTarget string) error {
	// 该 helper 同时服务只读的平台 Skill，目录需允许隔离 UID 穿越读取。
	root, err := confinedfs.Open(rootPath)
	if err != nil {
		return err
	}
	defer root.Close()
	relativeLink, err := filepath.Rel(filepath.Clean(rootPath), filepath.Clean(linkPath))
	if err != nil || relativeLink == ".." || strings.HasPrefix(relativeLink, ".."+string(filepath.Separator)) {
		return errors.New("symlink path escapes root")
	}
	relativeLink = filepath.ToSlash(relativeLink)
	if err := root.MkdirAll(filepath.ToSlash(filepath.Dir(relativeLink)), 0o755); err != nil {
		return err
	}
	if current, err := root.Readlink(relativeLink); err == nil {
		if current == relativeTarget {
			return nil
		}
		if err = root.Remove(relativeLink); err != nil {
			return err
		}
	} else if _, statErr := root.Lstat(relativeLink); statErr == nil {
		if err = root.RemoveAll(relativeLink); err != nil {
			return err
		}
	}
	if err = createSymlink(relativeTarget, linkPath); err != nil {
		return err
	}
	return root.Symlink(relativeTarget, relativeLink)
}

func copyFileToRoot(sourcePath string, targetRoot *confinedfs.Root, targetPath string) error {
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	targetFile, err := targetRoot.OpenFile(targetPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, workspaceFileMode())
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
	return targetRoot.Chmod(targetPath, workspaceCopyFileMode(info.Mode()))
}

func removeWorkspaceEntry(workspacePath string, relativePath string) error {
	root, err := confinedfs.Open(workspacePath)
	if err != nil {
		return err
	}
	defer root.Close()
	return root.RemoveAll(relativePath)
}

func relativePathWithin(rootPath string, targetPath string) (string, error) {
	rootPath = filepath.Clean(strings.TrimSpace(rootPath))
	targetPath = filepath.Clean(strings.TrimSpace(targetPath))
	relativePath, err := filepath.Rel(rootPath, targetPath)
	if err != nil || relativePath == "." || relativePath == ".." ||
		strings.HasPrefix(relativePath, ".."+string(filepath.Separator)) {
		return "", errors.New("target path escapes confined root")
	}
	return filepath.ToSlash(relativePath), nil
}

func workspaceCopyFileMode(sourceMode os.FileMode) os.FileMode {
	if !runtimeIsolationEnforced() {
		return sourceMode
	}
	ownerPermissions := sourceMode.Perm()&0o700 | 0o600
	return ownerPermissions | ownerPermissions>>3
}
