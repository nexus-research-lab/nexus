// INPUT: 平台或用户 Skill 源目录、已授权 workspace 根与模板上下文。
// OUTPUT: nxs/Claude Skill 入口及包含默认产品说明、非可选 Goal/Execution 绑定的运行时可见/停用 Skill 名称。
// POS: 宿主同步 Skill 文件时的双向 confinedfs 边界。
package workspace

import (
	"errors"
	"io/fs"
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

var (
	baseSkillNames      = append(append([]string{"imagegen", "visualize", "automation"}, agentsvc.ManagedSemanticSkillNames()...), "nexus-configuration", "nexus-product-guide")
	mainAgentSkillNames = []string{"nexus-manager"}
	// createSymlink 仅作为平台能力探针；真正的创建由 confinedfs.Root.Symlink 完成。
	createSymlink = func(string, string) error { return nil }
)

// UndeploySkill 从 workspace 中移除指定 skill。
func UndeploySkill(workspacePath string, skillName string) error {
	root, err := confinedfs.Open(workspacePath)
	if err != nil {
		return err
	}
	defer root.Close()
	return UndeploySkillAt(root, skillName)
}

// UndeploySkillAt 从已验证的 workspace 根移除指定 skill。
func UndeploySkillAt(root *confinedfs.Root, skillName string) error {
	if err := validateWorkspaceSkillName(skillName); err != nil {
		return err
	}
	if err := root.RemoveAll(filepath.ToSlash(filepath.Join(".agents", "skills", skillName))); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := root.RemoveAll(filepath.ToSlash(filepath.Join(".claude", "skills", skillName))); err != nil && !os.IsNotExist(err) {
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
	root, err := confinedfs.Open(workspacePath)
	if os.IsNotExist(err) {
		return []string{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer root.Close()
	return ListDeployedSkillsAt(root)
}

// ListDeployedSkillsAt 从已验证的 workspace 根枚举已部署 skill。
func ListDeployedSkillsAt(root *confinedfs.Root) ([]string, error) {
	parents := []string{
		// Claude 兼容入口可能是普通镜像目录，不能只依赖 .agents/skills。
		".agents/skills",
		".claude/skills",
	}
	result := []string{}
	seen := map[string]struct{}{}
	for _, parent := range parents {
		parentRoot, err := root.OpenRootNoSymlink(parent)
		if os.IsNotExist(err) || errors.Is(err, confinedfs.ErrSymlink) {
			continue
		}
		if err != nil {
			return nil, err
		}
		entries, err := fs.ReadDir(parentRoot.FS(), ".")
		if err != nil {
			parentRoot.Close()
			return nil, err
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
			skillFile, fileErr := skillRoot.OpenFileNoSymlink("SKILL.md", os.O_RDONLY, 0)
			if fileErr != nil {
				skillRoot.Close()
				continue
			}
			_ = skillFile.Close()
			_ = skillRoot.Close()
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
		_ = parentRoot.Close()
	}
	sort.Strings(result)
	return result, nil
}

// RuntimeSkillNamesForAgent 从 owner 绑定的 workspace fd 读取运行时 Skill。
// Goal/Execution 受管 Skill 是宿主能力，不以可能陈旧的持久化选择作为启动条件。
func RuntimeSkillNamesForAgent(
	cfg config.Config,
	agentValue protocol.Agent,
) ([]string, error) {
	root, err := workspacestore.New(cfg.WorkspacePath).OpenOwnerWorkspacePath(
		agentValue.OwnerUserID,
		agentValue.WorkspacePath,
		false,
	)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	selectedSkillIDs, disabledSkillIDs := agentsvc.BindManagedSemanticSkills(
		agentValue.Options.SkillIDs,
		agentValue.Options.DisabledSkillIDs,
	)
	return RuntimeSkillNamesAt(
		root,
		selectedSkillIDs,
		disabledSkillIDs,
	)
}

// RuntimeDisabledSkillNamesForAgent 构造运行时明确拒绝的 Skill 名称。
//
// 全局源对所有 Agent 可见，但只有绑定到当前 Agent 的名称可用；工作区 Skill
// 则默认动态可见，仅在 Agent 显式停用后进入拒绝集合。
func RuntimeDisabledSkillNamesForAgent(
	cfg config.Config,
	agentValue protocol.Agent,
) ([]string, error) {
	selectedSkillIDs, disabledSkillIDs := agentsvc.BindManagedSemanticSkills(
		agentValue.Options.SkillIDs,
		agentValue.Options.DisabledSkillIDs,
	)
	selected := normalizedSkillNameSet(selectedSkillIDs)
	explicitlyDisabled := normalizedSkillNameSet(disabledSkillIDs)
	enabledInWorkspace := maps.Clone(selected)
	disabledByName := make(map[string]string)
	addDisabled := func(reference string) {
		name := canonicalRuntimeSkillName(reference)
		key := strings.ToLower(name)
		if key == "" {
			return
		}
		if _, exists := disabledByName[key]; !exists {
			disabledByName[key] = name
		}
	}
	for _, reference := range disabledSkillIDs {
		addDisabled(reference)
	}
	// Agent workspace 内的本地 Skill 默认动态可见；先把未显式停用的名称
	// 加入启用集合，避免后面的全局库扫描把同名本地 Skill 误判成“未绑定”。
	workspaceRoot, workspaceErr := workspacestore.New(cfg.WorkspacePath).OpenOwnerWorkspacePath(
		agentValue.OwnerUserID,
		agentValue.WorkspacePath,
		false,
	)
	if workspaceErr == nil {
		deployedNames, listErr := ListDeployedSkillsAt(workspaceRoot)
		closeErr := workspaceRoot.Close()
		if listErr != nil {
			return nil, listErr
		}
		if closeErr != nil {
			return nil, closeErr
		}
		for _, name := range deployedNames {
			key := strings.ToLower(strings.TrimSpace(name))
			if key == "" {
				continue
			}
			if _, blocked := explicitlyDisabled[key]; blocked {
				continue
			}
			enabledInWorkspace[key] = struct{}{}
		}
	} else if !os.IsNotExist(workspaceErr) {
		return nil, workspaceErr
	}
	for _, root := range SkillLibraryRoots(cfg, agentValue.OwnerUserID) {
		names, err := ListDeployedSkills(root)
		if err != nil {
			return nil, err
		}
		for _, name := range names {
			key := strings.ToLower(strings.TrimSpace(name))
			if _, enabled := enabledInWorkspace[key]; !enabled {
				addDisabled(name)
			}
		}
	}
	keys := make([]string, 0, len(disabledByName))
	for key := range disabledByName {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]string, 0, len(keys))
	for _, key := range keys {
		result = append(result, disabledByName[key])
	}
	return result, nil
}

// RuntimeSkillNamesAt 在已验证的 workspace 根中合并运行时 Skill。
func RuntimeSkillNamesAt(
	root *confinedfs.Root,
	selectedSkillIDs []string,
	disabledSkillSets ...[]string,
) ([]string, error) {
	result := make([]string, 0, len(selectedSkillIDs))
	seen := make(map[string]struct{}, len(result))
	var disabledSkillIDs []string
	if len(disabledSkillSets) > 0 {
		disabledSkillIDs = disabledSkillSets[0]
	}
	disabled := normalizedSkillNameSet(disabledSkillIDs)
	for _, reference := range selectedSkillIDs {
		name := reference
		if externalName, ok := protocol.ParseExternalSkillReference(reference); ok {
			name = externalName
		}
		if normalized := strings.ToLower(strings.TrimSpace(name)); normalized != "" {
			if _, blocked := disabled[normalized]; blocked {
				continue
			}
			if _, exists := seen[normalized]; exists {
				continue
			}
			result = append(result, name)
			seen[normalized] = struct{}{}
		}
	}
	deployedNames, err := ListDeployedSkillsAt(root)
	if err != nil {
		return nil, err
	}
	for _, name := range deployedNames {
		key := strings.ToLower(strings.TrimSpace(name))
		if key == "" {
			continue
		}
		if _, blocked := disabled[key]; blocked {
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

func normalizedSkillNameSet(names []string) map[string]struct{} {
	result := make(map[string]struct{}, len(names))
	for _, name := range names {
		normalized := canonicalRuntimeSkillName(name)
		if normalized != "" {
			result[strings.ToLower(normalized)] = struct{}{}
		}
	}
	return result
}

func canonicalRuntimeSkillName(reference string) string {
	normalized := strings.TrimSpace(reference)
	if externalName, ok := protocol.ParseExternalSkillReference(normalized); ok {
		return externalName
	}
	return normalized
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
	return ensureRelativeSymlinkAt(root, relativeLink, relativeTarget)
}

func ensureRelativeSymlinkAt(
	root *confinedfs.Root,
	relativeLink string,
	relativeTarget string,
) error {
	parentRoot, err := root.OpenOrCreateRootNoSymlink(path.Dir(relativeLink), 0o755)
	if err != nil {
		return err
	}
	defer parentRoot.Close()
	linkName := path.Base(relativeLink)
	if current, err := parentRoot.Readlink(linkName); err == nil {
		if current == relativeTarget {
			return nil
		}
		if err = parentRoot.Remove(linkName); err != nil {
			return err
		}
	} else if _, statErr := parentRoot.Lstat(linkName); statErr == nil {
		if err = parentRoot.RemoveAll(linkName); err != nil {
			return err
		}
	}
	linkPath := filepath.Join(root.Name(), filepath.FromSlash(relativeLink))
	if err = createSymlink(relativeTarget, linkPath); err != nil {
		return err
	}
	return parentRoot.Symlink(relativeTarget, linkName)
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
