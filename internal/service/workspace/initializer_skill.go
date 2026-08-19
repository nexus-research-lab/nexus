// INPUT: 平台或用户 Skill 源目录、已授权 workspace 根与模板上下文。
// OUTPUT: nxs/Claude Skill 入口及包含非可选 Goal/Execution 绑定的运行时可见/停用 Skill 名称。
// POS: 宿主同步 Skill 文件时的双向 confinedfs 边界。
package workspace

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/runtimecommand"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

var (
	baseSkillNames      = append(append([]string{"imagegen", "visualize", "automation"}, runtimecommand.ManagedSemanticSkillNames()...), "nexus-configuration")
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
	if err := os.MkdirAll(workspacePath, workspaceDirectoryMode()); err != nil {
		return err
	}
	root, err := confinedfs.Open(workspacePath)
	if err != nil {
		return err
	}
	defer root.Close()
	return DeploySkillAt(root, skillName, sourceDir, context)
}

// DeploySkillAt 把指定 skill 部署到已验证的 workspace 根。
func DeploySkillAt(
	root *confinedfs.Root,
	skillName string,
	sourceDir string,
	context map[string]string,
) error {
	sourceRoot, err := confinedfs.Open(sourceDir)
	if err != nil {
		return err
	}
	defer sourceRoot.Close()
	return DeploySkillAtFromRoots(root, sourceRoot, skillName, context)
}

// DeploySkillAtFromRoots 在已固定的源与目标目录句柄之间部署 Skill。
//
// 外部 Skill 源可能位于另一个 owner 的共享目录；调用方应先通过
// owner-aware store 固定 sourceRoot，再把它传入，避免校验后重新打开
// record.SourcePath 形成 TOCTOU 或跨 owner 路径旁路。
func DeploySkillAtFromRoots(
	root *confinedfs.Root,
	sourceRoot *confinedfs.Root,
	skillName string,
	context map[string]string,
) error {
	if root == nil || sourceRoot == nil {
		return errors.New("skill source 或 workspace 根句柄不能为空")
	}
	if err := validateWorkspaceSkillName(skillName); err != nil {
		return err
	}
	agentsSkillDir := filepath.ToSlash(filepath.Join(".agents", "skills", skillName))
	claudeSkillEntry := filepath.ToSlash(filepath.Join(".claude", "skills", skillName))
	if err := syncDirectoryRootAt(sourceRoot, root, agentsSkillDir, context); err != nil {
		return err
	}
	return ensureClaudeSkillEntryRootAt(
		sourceRoot,
		root,
		claudeSkillEntry,
		filepath.Join("..", "..", ".agents", "skills", skillName),
		context,
	)
}

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

// RuntimeSkillNames 合并 Agent 引用与 workspace-local Skill，形成运行时白名单。
//
// 外部引用以 external:<name> 形式持久化，进入 SDK 前还原为 canonical name；
// workspace-local Skill 仍从 workspace 文件发现，避免显式白名单把它过滤掉。
func RuntimeSkillNames(workspacePath string, selectedSkillIDs []string) ([]string, error) {
	root, err := confinedfs.Open(workspacePath)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	return RuntimeSkillNamesAt(root, selectedSkillIDs, nil)
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
	selectedSkillIDs, disabledSkillIDs := runtimecommand.BindManagedSemanticSkills(
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
	selectedSkillIDs, disabledSkillIDs := runtimecommand.BindManagedSemanticSkills(
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
		return strings.TrimSpace(externalName)
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
	return syncDirectoryAt(sourceDir, root, relativeTarget, context)
}

func syncDirectoryAt(
	sourceDir string,
	root *confinedfs.Root,
	relativeTarget string,
	context map[string]string,
) error {
	sourceRoot, err := confinedfs.Open(sourceDir)
	if err != nil {
		return err
	}
	defer sourceRoot.Close()
	return syncDirectoryRootAt(sourceRoot, root, relativeTarget, context)
}

func syncDirectoryRootAt(
	sourceRoot *confinedfs.Root,
	root *confinedfs.Root,
	relativeTarget string,
	context map[string]string,
) error {
	if sourceRoot == nil || root == nil {
		return errors.New("skill source 或 workspace 根句柄不能为空")
	}
	var err error
	if err = root.RemoveAll(relativeTarget); err != nil && !os.IsNotExist(err) {
		return err
	}
	targetRoot, err := root.OpenOrCreateRootNoSymlink(relativeTarget, workspaceDirectoryMode())
	if err != nil {
		return err
	}
	defer targetRoot.Close()
	return syncSkillDirectory(sourceRoot, targetRoot, context)
}

func syncSkillDirectory(
	sourceRoot *confinedfs.Root,
	targetRoot *confinedfs.Root,
	context map[string]string,
) error {
	entries, err := fs.ReadDir(sourceRoot.FS(), ".")
	if err != nil {
		return err
	}
	for _, entry := range entries {
		info, err := sourceRoot.Lstat(entry.Name())
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			continue
		}
		if info.IsDir() {
			sourceChild, err := sourceRoot.OpenRootNoSymlink(entry.Name())
			if err != nil {
				return err
			}
			targetChild, targetErr := targetRoot.OpenOrCreateRootNoSymlink(
				entry.Name(),
				workspaceDirectoryMode(),
			)
			if targetErr != nil {
				sourceChild.Close()
				return targetErr
			}
			copyErr := syncSkillDirectory(sourceChild, targetChild, context)
			sourceChild.Close()
			targetChild.Close()
			if copyErr != nil {
				return copyErr
			}
			continue
		}
		if !info.Mode().IsRegular() {
			continue
		}
		sourceFile, err := sourceRoot.OpenFileNoSymlink(entry.Name(), os.O_RDONLY, 0)
		if err != nil {
			return err
		}
		openedInfo, err := sourceFile.Stat()
		if err != nil {
			sourceFile.Close()
			return err
		}
		if entry.Name() == "SKILL.md" {
			content, readErr := io.ReadAll(sourceFile)
			sourceFile.Close()
			if readErr != nil {
				return readErr
			}
			rendered := renderTemplate(string(content), context)
			if err = targetRoot.WriteFileAtomic(
				entry.Name(),
				[]byte(strings.TrimSpace(rendered)+"\n"),
				workspaceFileMode(),
			); err != nil {
				return err
			}
			continue
		}
		err = targetRoot.WriteFileAtomicFrom(
			entry.Name(),
			sourceFile,
			workspaceCopyFileMode(openedInfo.Mode()),
		)
		sourceFile.Close()
		if err != nil {
			return err
		}
	}
	return nil
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

func ensureClaudeSkillEntryAt(
	sourceDir string,
	root *confinedfs.Root,
	entryPath string,
	relativeTarget string,
	context map[string]string,
) error {
	sourceRoot, err := confinedfs.Open(sourceDir)
	if err != nil {
		return err
	}
	defer sourceRoot.Close()
	return ensureClaudeSkillEntryRootAt(sourceRoot, root, entryPath, relativeTarget, context)
}

func ensureClaudeSkillEntryRootAt(
	sourceRoot *confinedfs.Root,
	root *confinedfs.Root,
	entryPath string,
	relativeTarget string,
	context map[string]string,
) error {
	if sourceRoot == nil || root == nil {
		return errors.New("skill source 或 workspace 根句柄不能为空")
	}
	err := ensureRelativeSymlinkAt(root, entryPath, relativeTarget)
	if err == nil {
		return nil
	}
	// Windows 默认可能没有目录 symlink 权限，失败时镜像一份给 Claude 读取。
	if mirrorErr := syncDirectoryRootAt(sourceRoot, root, entryPath, context); mirrorErr != nil {
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
