// INPUT: 桌面模式、宿主标准 Skill 源与受管根生命周期。
// OUTPUT: 稳定的宿主 Skill 根、后台同步结果与 watcher 目录。
// POS: 宿主 Skill 来源层的启动、清理与刷新编排入口。
package workspace

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
)

var hostSkillLibrarySyncMu sync.Mutex

type skippedHostSkill struct {
	name             string
	err              error
	discardLastGood  bool
	retainedLastGood bool
}

type hostSkillSyncResult struct {
	skipped          []skippedHostSkill
	watchDirectories []string
}

// PrepareHostSkillLibrary 创建稳定的宿主兼容根，不扫描用户文件。
//
// Claude Code 只监听会话启动时已经存在的 additional directory。服务启动前先
// 创建空根，既不让可选宿主扫描阻塞健康检查，也不会让首个 runtime 永久漏掉该根。
func PrepareHostSkillLibrary(cfg config.Config) error {
	hostSkillLibrarySyncMu.Lock()
	defer hostSkillLibrarySyncMu.Unlock()

	if !isDesktopAppMode(cfg) {
		return removeHostSkillProjection()
	}
	_, err := ensureHostSkillProjectionLayout()
	return err
}

// EnsureHostSkillLibrary 同步桌面用户的标准全局 Skill 源。
//
// 多用户服务不能读取宿主进程的 home；桌面模式下一名本机用户共享一份受管根，
// Agent 只保存启用引用，不再把同一 Skill 复制到各自 workspace。
func EnsureHostSkillLibrary(cfg config.Config) error {
	_, err := syncHostSkillLibrary(cfg)
	return err
}

func syncHostSkillLibrary(cfg config.Config) (hostSkillSyncResult, error) {
	hostSkillLibrarySyncMu.Lock()
	defer hostSkillLibrarySyncMu.Unlock()

	if !isDesktopAppMode(cfg) {
		return hostSkillSyncResult{}, removeHostSkillProjection()
	}
	claudeLinked, err := ensureHostSkillProjectionLayout()
	if err != nil {
		return hostSkillSyncResult{}, err
	}
	if err = clearHostSkillStagingLayout(); err != nil {
		return hostSkillSyncResult{}, err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return hostSkillSyncResult{}, fmt.Errorf("读取宿主用户目录失败: %w", err)
	}
	if strings.TrimSpace(home) == "" {
		return hostSkillSyncResult{}, errors.New("宿主用户目录为空")
	}
	sourcePath := filepath.Join(home, ".agents", "skills")
	result := hostSkillSyncResult{
		watchDirectories: existingHostSkillWatchAnchors(sourcePath),
	}
	if _, statErr := os.Stat(sourcePath); os.IsNotExist(statErr) {
		return result, clearHostSkillProjectionEntries(claudeLinked)
	} else if statErr != nil {
		return result, statErr
	}

	source, resolvedHome, resolvedSource, err := openHostSkillSource(home, sourcePath)
	if err != nil {
		return result, err
	}
	defer source.Close()
	result, err = publishHostSkillProjection(
		resolvedHome,
		resolvedSource,
		source,
		claudeLinked,
	)
	result.watchDirectories = mergeHostSkillWatchDirectories(
		result.watchDirectories,
		existingHostSkillWatchAnchors(sourcePath),
	)
	return result, err
}

func isDesktopAppMode(cfg config.Config) bool {
	return strings.EqualFold(strings.TrimSpace(cfg.AppMode), "desktop")
}

func openHostSkillSource(
	home string,
	sourcePath string,
) (*confinedfs.Root, string, string, error) {
	resolvedHome, err := filepath.EvalSymlinks(filepath.Clean(home))
	if err != nil {
		return nil, "", "", fmt.Errorf("解析宿主用户目录失败: %w", err)
	}
	resolvedSource, err := filepath.EvalSymlinks(filepath.Clean(sourcePath))
	if err != nil {
		return nil, "", "", fmt.Errorf("解析宿主 Skill 根失败: %w", err)
	}
	if err = validateHostSkillProjectionBoundary(resolvedSource); err != nil {
		return nil, "", "", err
	}
	relativeSource, err := relativePathWithin(resolvedHome, resolvedSource)
	if err != nil {
		return nil, "", "", fmt.Errorf("宿主 Skill 根必须位于用户目录内: %w", err)
	}
	homeRoot, err := confinedfs.Open(resolvedHome)
	if err != nil {
		return nil, "", "", err
	}
	defer homeRoot.Close()
	source, err := homeRoot.OpenRootNoSymlink(relativeSource)
	if err != nil {
		return nil, "", "", err
	}
	return source, resolvedHome, resolvedSource, nil
}

func validateHostSkillProjectionBoundary(candidate string) error {
	projection, err := filepath.EvalSymlinks(filepath.Clean(appfs.HostSkillRoot()))
	if err != nil {
		return fmt.Errorf("解析宿主 Skill 受管根失败: %w", err)
	}
	if hostSkillPathsOverlap(filepath.Clean(candidate), projection) {
		return errors.New("宿主 Skill 源不能与 Nexus 受管投影重叠")
	}
	return nil
}

func hostSkillPathsOverlap(left string, right string) bool {
	if samePath(left, right) {
		return true
	}
	if _, err := relativePathWithin(left, right); err == nil {
		return true
	}
	_, err := relativePathWithin(right, left)
	return err == nil
}

// ensureHostSkillProjectionLayout 返回 Claude 是否直接链接到 canonical 视图。
//
// .agents/skills 始终保持为稳定目录；后台刷新只替换其中单个 Skill，避免正在
// 运行的 Claude watcher 因整个 additional directory 被换走而失效。
func ensureHostSkillProjectionLayout() (bool, error) {
	root, err := openOrCreateHostSkillProjectionRoot()
	if err != nil {
		return false, err
	}
	defer root.Close()
	agentsParent, err := openOrCreateManagedHostSkillDirectory(root, ".agents")
	if err != nil {
		return false, err
	}
	agentsRoot, err := openOrCreateManagedHostSkillDirectory(agentsParent, "skills")
	if err != nil {
		agentsParent.Close()
		return false, err
	}
	if err = agentsRoot.Close(); err == nil {
		err = agentsParent.Close()
	} else {
		agentsParent.Close()
	}
	if err != nil {
		return false, err
	}

	relativeTarget := filepath.Join("..", ".agents", "skills")
	claudeParent, err := openOrCreateManagedHostSkillDirectory(root, ".claude")
	if err != nil {
		return false, err
	}
	if current, readErr := claudeParent.Readlink("skills"); readErr == nil {
		if filepath.Clean(current) == filepath.Clean(relativeTarget) {
			return true, claudeParent.Close()
		}
		if err = claudeParent.RemoveAll("skills"); err != nil {
			claudeParent.Close()
			return false, err
		}
	} else if info, statErr := claudeParent.Lstat("skills"); statErr == nil {
		if info.Mode()&os.ModeSymlink == 0 && info.IsDir() {
			// Windows fallback 一旦建立便保持目录身份，后续按 Skill 增量刷新。
			fallbackRoot, openErr := openOrCreateManagedHostSkillDirectory(claudeParent, "skills")
			if openErr != nil {
				claudeParent.Close()
				return false, openErr
			}
			fallbackCloseErr := fallbackRoot.Close()
			parentCloseErr := claudeParent.Close()
			if fallbackCloseErr != nil {
				return false, fallbackCloseErr
			}
			return false, parentCloseErr
		}
		if err = claudeParent.RemoveAll("skills"); err != nil {
			claudeParent.Close()
			return false, err
		}
	} else if !os.IsNotExist(statErr) {
		claudeParent.Close()
		return false, statErr
	}
	if err = claudeParent.Close(); err != nil {
		return false, err
	}

	if err = ensureRelativeSymlinkAt(root, ".claude/skills", relativeTarget); err == nil {
		return true, nil
	}
	claudeRoot, createErr := root.OpenOrCreateRootNoSymlink(".claude/skills", 0o755)
	if createErr != nil {
		return false, fmt.Errorf("创建 Claude Skill 入口失败: %v；fallback 目录也失败: %w", err, createErr)
	}
	if chmodErr := claudeRoot.ChmodRoot(0o755); chmodErr != nil {
		claudeRoot.Close()
		return false, chmodErr
	}
	return false, claudeRoot.Close()
}

func openOrCreateHostSkillProjectionRoot() (*confinedfs.Root, error) {
	targetPath := filepath.Clean(appfs.HostSkillRoot())
	parentPath := filepath.Dir(targetPath)
	if err := os.MkdirAll(parentPath, 0o755); err != nil {
		return nil, err
	}
	parentRoot, err := confinedfs.Open(parentPath)
	if err != nil {
		return nil, err
	}
	defer parentRoot.Close()
	return openOrCreateManagedHostSkillDirectory(parentRoot, filepath.Base(targetPath))
}

func openOrCreateManagedHostSkillDirectory(
	root *confinedfs.Root,
	name string,
) (*confinedfs.Root, error) {
	var directory *confinedfs.Root
	info, err := root.Lstat(name)
	if os.IsNotExist(err) {
		directory, err = root.OpenOrCreateRootNoSymlink(name, 0o755)
	} else if err != nil {
		return nil, err
	} else if info.Mode()&os.ModeSymlink == 0 && info.IsDir() {
		directory, err = root.OpenRootNoSymlink(name)
	} else {
		if err = root.RemoveAll(name); err != nil {
			return nil, fmt.Errorf("修复宿主 Skill 受管目录 %s 失败: %w", name, err)
		}
		directory, err = root.OpenOrCreateRootNoSymlink(name, 0o755)
	}
	if err != nil {
		return nil, fmt.Errorf("准备宿主 Skill 受管目录 %s 失败: %w", name, err)
	}
	if err = directory.ChmodRoot(0o755); err != nil {
		directory.Close()
		return nil, fmt.Errorf("修复宿主 Skill 受管目录 %s 权限失败: %w", name, err)
	}
	return directory, nil
}

func clearHostSkillStagingDirectories(root *confinedfs.Root) error {
	entries, exceeded, err := readBoundedHostSkillDirectory(root, hostSkillMaxEntries)
	if err != nil {
		return err
	}
	if exceeded {
		return fmt.Errorf("宿主 Skill 受管根条目数量超过 %d", hostSkillMaxEntries)
	}
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, ".host-skill-staging-") &&
			!strings.HasPrefix(name, ".host-claude-staging-") {
			continue
		}
		if err = root.RemoveAll(name); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func clearHostSkillStagingLayout() error {
	root, err := confinedfs.Open(appfs.HostSkillRoot())
	if err != nil {
		return err
	}
	defer root.Close()
	return clearHostSkillStagingDirectories(root)
}

func publishHostSkillProjection(
	home string,
	sourcePath string,
	source *confinedfs.Root,
	claudeLinked bool,
) (hostSkillSyncResult, error) {
	root, err := confinedfs.Open(appfs.HostSkillRoot())
	if err != nil {
		return hostSkillSyncResult{}, err
	}
	defer root.Close()
	stagingRelative, err := root.MkdirTemp(".", ".host-skill-staging-", 0o755)
	if err != nil {
		return hostSkillSyncResult{}, err
	}
	defer root.RemoveAll(stagingRelative)
	stagingRoot, err := root.OpenRootNoSymlink(stagingRelative)
	if err != nil {
		return hostSkillSyncResult{}, err
	}

	builder := hostSkillSnapshotBuilder{
		home:             home,
		sourcePath:       sourcePath,
		source:           source,
		staged:           make(map[string]struct{}),
		canonicalNames:   make(map[string]string),
		watchDirectories: map[string]struct{}{sourcePath: {}},
	}
	buildErr := builder.buildAgentsView(stagingRoot)
	closeErr := stagingRoot.Close()
	if buildErr != nil {
		return builder.syncResult(), buildErr
	}
	if closeErr != nil {
		return builder.syncResult(), closeErr
	}
	kept := retainedHostSkillNames(root, builder.staged, &builder.result)
	if err = reconcileHostSkillEntries(
		root,
		stagingRelative,
		".agents/skills",
		"canonical",
		kept,
		builder.staged,
		&builder.result,
	); err != nil {
		return builder.syncResult(), err
	}
	if !claudeLinked {
		if err = syncClaudeHostSkillFallback(root, &builder.result); err != nil {
			return builder.syncResult(), err
		}
	}
	return builder.syncResult(), nil
}

func clearHostSkillProjectionEntries(claudeLinked bool) error {
	root, err := confinedfs.Open(appfs.HostSkillRoot())
	if err != nil {
		return err
	}
	defer root.Close()
	if err = clearHostSkillDirectoryAt(root, ".agents/skills"); err != nil {
		return err
	}
	if claudeLinked {
		return nil
	}
	return clearHostSkillDirectoryAt(root, ".claude/skills")
}

func clearHostSkillDirectoryAt(root *confinedfs.Root, relativePath string) error {
	directory, err := root.OpenRootNoSymlink(relativePath)
	if err != nil {
		return err
	}
	entries, exceeded, readErr := readBoundedHostSkillDirectory(directory, hostSkillMaxEntries)
	closeErr := directory.Close()
	if readErr != nil {
		return readErr
	}
	if exceeded {
		return fmt.Errorf("宿主 Skill 受管视图条目数量超过 %d", hostSkillMaxEntries)
	}
	if closeErr != nil {
		return closeErr
	}
	for _, entry := range entries {
		if err = root.RemoveAll(filepath.ToSlash(filepath.Join(relativePath, entry.Name()))); err != nil &&
			!os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func removeHostSkillProjection() error {
	targetPath := appfs.HostSkillRoot()
	parentPath := filepath.Clean(filepath.Dir(targetPath))
	parentRoot, err := confinedfs.Open(parentPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	defer parentRoot.Close()
	if err = parentRoot.RemoveAll(filepath.Base(filepath.Clean(targetPath))); err != nil &&
		!os.IsNotExist(err) {
		return err
	}
	return nil
}

func existingHostSkillWatchAnchors(sourcePath string) []string {
	home := filepath.Clean(filepath.Dir(filepath.Dir(sourcePath)))
	resolvedHome, err := filepath.EvalSymlinks(home)
	if err != nil {
		return nil
	}
	for _, candidate := range []string{filepath.Dir(sourcePath), resolvedHome} {
		resolvedCandidate, resolveErr := filepath.EvalSymlinks(filepath.Clean(candidate))
		if resolveErr != nil {
			continue
		}
		if resolvedCandidate != resolvedHome {
			if _, boundaryErr := relativePathWithin(resolvedHome, resolvedCandidate); boundaryErr != nil {
				continue
			}
		}
		info, statErr := os.Stat(resolvedCandidate)
		if statErr == nil && info.IsDir() {
			return []string{resolvedCandidate}
		}
	}
	return nil
}

func mergeHostSkillWatchDirectories(groups ...[]string) []string {
	directories := make(map[string]struct{})
	for _, group := range groups {
		for _, directory := range group {
			if strings.TrimSpace(directory) == "" {
				continue
			}
			directories[filepath.Clean(directory)] = struct{}{}
		}
	}
	return sortedHostSkillWatchDirectories(directories)
}

func sortedHostSkillWatchDirectories(directories map[string]struct{}) []string {
	result := make([]string, 0, len(directories))
	for directory := range directories {
		result = append(result, directory)
	}
	sort.Strings(result)
	return result
}
