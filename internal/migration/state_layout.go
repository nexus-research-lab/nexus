// INPUT: 旧版 Nexus 状态根。
// OUTPUT: 将宿主数据、用户 runtime 数据和 workspace 迁移到 .nexus/app 与 users/<owner>。
// POS: 启动前执行的文件系统布局迁移；不覆盖冲突文件，失败时保留源数据并允许重试。
package migration

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
)

const (
	stateLayoutMigrationName = "20260723_state_layout_v1"

	// 迁移发生在 runtime 可能仍在清理旧 transcript 的窗口内。
	// 少量重试足以覆盖 rename/unlink 的瞬态竞争，同时不会把真正的
	// 权限、冲突或 I/O 错误吞掉。
	layoutMoveRetryAttempts = 3

	// Room overlay 是追加式 JSONL；限制一次迁移合并读取的大小，避免
	// 异常文件让启动迁移占用不受控的内存。
	maxRoomOverlayMergeBytes = 64 << 20
)

var stateLayoutHostEntries = []string{
	"data",
	"config",
	"cache",
	"platform-skills",
	".agents",
	"rooms",
	".last-cleanup",
	".migrations", // 旧版 Nexus 工作区迁移账本，属于 app 控制面。
}

var stateLayoutRuntimeEntries = []string{
	"projects",
	".claude.json",
	".claude",
	".config.json",
	"config.json",
	"commands",
	"mcp-needs-auth-cache.json",
	"settings.json",
	"history.jsonl",
	"CLAUDE.md",
	"skills",
	"agents",
	"plugins",
	"cowork_plugins",
	"uploads",
	"usage-data",
	"keybindings.json",
	"magic-docs",
	"startup-perf",
	".credentials.json",
	".update.lock",
	".npm-cache-cleanup",
	".version-cleanup",
	"chrome",
	"plans",
	"rules",
	"teams",
	"ide",
	"jobs",
	"local",
	"traces",
	"session-env",
	"sessions",
	"shell-snapshots",
	"tasks",
	"telemetry",
	"backups",
	"debug",
	"file-history",
	"image-cache",
	"paste-cache",
	"dump-prompts",
	"session-memory",
	"policy-limits.json",
	"remote-settings.json",
	"stats-cache.json",
	"computer-use.lock",
}

// 旧版本曾把 Claude 的 runtime 文件放在 .nexus/config 下；这些条目需要
// 在 config 作为宿主目录迁移前单独分流。
var stateLayoutNestedRuntimeEntries = []string{
	".claude.json",
	".claude",
	".config.json",
	"config.json",
	"mcp-needs-auth-cache.json",
	"settings.json",
	"projects",
	"commands",
	"skills",
	"agents",
	"CLAUDE.md",
	"logs",
	"file-history",
	"image-cache",
	"paste-cache",
	"plugins",
	"cowork_plugins",
	"local",
	"startup-perf",
	"backups",
	"debug",
	"session-env",
	"sessions",
	"shell-snapshots",
	"telemetry",
	"tasks",
	"traces",
	"uploads",
	"usage-data",
	"keybindings.json",
	"magic-docs",
	".credentials.json",
	".update.lock",
	".npm-cache-cleanup",
	".version-cleanup",
	"chrome",
	"plans",
	"rules",
	"teams",
	"ide",
	"jobs",
	"remote-settings.json",
	"policy-limits.json",
	"stats-cache.json",
	"computer-use.lock",
}

// 旧 config 中仍属于 Nexus/桌面宿主的少量文件。未列出的条目默认按
// runtime 处理，避免 Claude/nxs 新增文件因为清单滞后落入 app。
var stateLayoutNestedHostEntries = map[string]struct{}{
	"runtime-settings.json":                 {},
	"desktop-state.json":                    {},
	"connector-credentials.key":             {},
	"connector-credentials.dpapi":           {},
	"update-check.json":                     {},
	"last-update-cache-cleanup-version.txt": {},
}

// RunStateLayout 执行宿主状态根布局迁移。
//
// 迁移只使用 rename 或不覆盖式合并，不会丢弃有差异的业务源数据。
// Finder 的 .DS_Store 与 cache 是可再生数据：旧 cache 直接删除，
// .DS_Store 冲突时保留目标。其他目标冲突直接返回错误，调用方可处理后重试。
func RunStateLayout(stateRoot string, logger *slog.Logger) error {
	return runStateLayout(
		stateRoot,
		logger,
		appfs.RuntimeIsolationEnforced(),
	)
}

func runStateLayout(
	stateRoot string,
	logger *slog.Logger,
	launcherManagesPermissions bool,
) error {
	if logger == nil {
		logger = slog.Default()
	}
	stateRoot = filepath.Clean(stateRoot)
	if stateRoot == "." {
		return errors.New("状态根不能为空")
	}

	appRoot := filepath.Join(stateRoot, "app")
	markerPath := filepath.Join(appRoot, ".migrations", stateLayoutMigrationName)
	applied, err := layoutMigrationApplied(markerPath)
	if err != nil {
		return err
	}
	// 强隔离会在本迁移之后把状态树改成 root/runtime UID 所有。
	// 完成标记必须在任何 mkdir/chmod 之前判断，避免普通宿主在二次启动时
	// 重新收紧已经由 launcher 管理的 ACL。
	if applied {
		return nil
	}
	if err = prepareSkippedStateLayoutRecovery(stateRoot, logger); err != nil {
		return err
	}

	usersRoot := filepath.Join(stateRoot, "users")
	sharedRoot := filepath.Join(stateRoot, "shared-workspaces")
	systemRuntimeRoot := filepath.Join(usersRoot, authctx.SystemUserID, "runtime")
	for _, directory := range []string{appRoot, usersRoot, sharedRoot, systemRuntimeRoot} {
		if err = os.MkdirAll(directory, 0o700); err != nil {
			return fmt.Errorf("创建状态迁移目标目录 %q: %w", directory, err)
		}
	}

	affected := 0
	logsAffected, logsErr := moveLegacyLogs(
		filepath.Join(stateRoot, "logs"),
		filepath.Join(appRoot, "logs"),
		filepath.Join(systemRuntimeRoot, "logs"),
	)
	if logsErr != nil {
		return fmt.Errorf("迁移混合日志目录: %w", logsErr)
	}
	affected += logsAffected
	for _, name := range stateLayoutRuntimeEntries {
		// runtime 目录可能已被桌面壳或 Docker bind mount 预创建；
		// 差异内容保留为副本，不能让一份用户配置阻断整个升级。
		moved, moveErr := moveNestedRuntimeEntry(
			filepath.Join(stateRoot, name),
			runtimeEntryTarget(systemRuntimeRoot, name),
			name,
		)
		if moveErr != nil {
			return fmt.Errorf("迁移用户 runtime 状态 %q: %w", name, moveErr)
		}
		if moved {
			affected++
		}
	}
	nestedAffected, nestedErr := migrateNestedRuntimeEntries(
		filepath.Join(stateRoot, "config"),
		systemRuntimeRoot,
	)
	if nestedErr != nil {
		return nestedErr
	}
	affected += nestedAffected
	for _, name := range stateLayoutHostEntries {
		var moved bool
		var moveErr error
		sourcePath := filepath.Join(stateRoot, name)
		targetPath := filepath.Join(appRoot, name)
		switch name {
		case "config":
			moved, moveErr = moveLegacyHostConfig(sourcePath, targetPath)
		case "cache":
			moved, moveErr = removeLegacyCache(sourcePath)
		default:
			moved, moveErr = moveLayoutEntry(sourcePath, targetPath)
		}
		if moveErr != nil {
			return fmt.Errorf("迁移宿主状态 %q: %w", name, moveErr)
		}
		if moved {
			affected++
		}
	}

	if err = moveUnknownStateEntries(stateRoot, systemRuntimeRoot); err != nil {
		return err
	}
	if err = hardenMigratedStateLayout(
		stateRoot,
		appRoot,
		usersRoot,
		sharedRoot,
		launcherManagesPermissions,
	); err != nil {
		return err
	}
	if err = writeLayoutMigrationMarker(markerPath); err != nil {
		return err
	}
	logger.Info("状态根布局迁移完成",
		"migration", stateLayoutMigrationName,
		"state_root", stateRoot,
		"affected_entries", affected,
	)
	return nil
}

func hardenMigratedStateLayout(
	stateRoot string,
	appRoot string,
	usersRoot string,
	sharedRoot string,
	launcherManagesPermissions bool,
) error {
	if !shouldHardenMigratedPermissions(launcherManagesPermissions) {
		// 桌面端保留宿主用户的原生权限；Linux enforce 由 root launcher
		// 通过宿主/owner/project ACL 维护，迁移只负责移动数据。
		return nil
	}
	if err := hardenLayoutTree(appRoot); err != nil {
		return fmt.Errorf("收紧 app 状态权限: %w", err)
	}
	if err := hardenLayoutTree(usersRoot); err != nil {
		return fmt.Errorf("收紧用户状态权限: %w", err)
	}
	if err := hardenLayoutTree(sharedRoot); err != nil {
		return fmt.Errorf("收紧共享 workspace 权限: %w", err)
	}
	if err := hardenStateRoot(stateRoot); err != nil {
		return err
	}
	return nil
}

var legacyHostLogEntries = map[string]struct{}{
	"logger.log":                 {},
	"system-package-install.log": {},
}

// moveLegacyLogs 拆分旧版同时被 Nexus 与 runtime 使用的 logs 目录。
//
// server logger 与系统包安装日志属于 app；其余条目（尤其是 nxs/Claude
// debug、trace、session 相关日志）没有可靠 owner 线索，保守进入 system
// runtime，避免把用户内容暴露给宿主共享目录。
func moveLegacyLogs(sourcePath string, appTarget string, runtimeTarget string) (int, error) {
	info, err := os.Lstat(sourcePath)
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		target := runtimeTarget
		if _, ok := legacyHostLogEntries[filepath.Base(sourcePath)]; ok {
			target = appTarget
		}
		moved, moveErr := moveLayoutEntry(sourcePath, target)
		return boolToInt(moved), moveErr
	}

	entries, err := os.ReadDir(sourcePath)
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	affected := 0
	for _, entry := range entries {
		targetRoot := runtimeTarget
		if _, ok := legacyHostLogEntries[entry.Name()]; ok {
			targetRoot = appTarget
		}
		moved, moveErr := moveLegacyHostConfig(
			filepath.Join(sourcePath, entry.Name()),
			filepath.Join(targetRoot, entry.Name()),
		)
		if moveErr != nil {
			return affected, moveErr
		}
		if moved {
			affected++
		}
	}
	remaining, err := os.ReadDir(sourcePath)
	if errors.Is(err, os.ErrNotExist) {
		return affected, nil
	}
	if err != nil {
		return affected, err
	}
	if len(remaining) == 0 {
		if err = os.Remove(sourcePath); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return affected, nil
			}
			return affected, err
		}
	}
	return affected, nil
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

// moveLegacyHostConfig 合并旧 config 与可能已经被桌面壳预创建的 app/config。
//
// 桌面壳在 sidecar 启动前可能写入新的 desktop-state 或 credentials 文件；
// 这些文件不能让整个升级失败。差异内容保留为 .legacy-config 副本，
// 不覆盖当前目标，也不删除旧数据。
func moveLegacyHostConfig(sourcePath string, targetPath string) (bool, error) {
	return retryMissingLayoutSource(sourcePath, func() (bool, error) {
		return moveLegacyHostConfigOnce(sourcePath, targetPath)
	})
}

func moveLegacyHostConfigOnce(sourcePath string, targetPath string) (bool, error) {
	sourceInfo, sourceErr := os.Lstat(sourcePath)
	if errors.Is(sourceErr, os.ErrNotExist) {
		return false, nil
	}
	if sourceErr != nil {
		return false, sourceErr
	}
	targetInfo, targetErr := os.Lstat(targetPath)
	if errors.Is(targetErr, os.ErrNotExist) {
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o700); err != nil {
			return false, err
		}
		if err := os.Rename(sourcePath, targetPath); err != nil {
			return false, err
		}
		return true, nil
	}
	if targetErr != nil {
		return false, targetErr
	}
	if sourceInfo.IsDir() && targetInfo.IsDir() {
		return mergeLegacyHostConfigDirectories(sourcePath, targetPath)
	}
	if sourceInfo.Mode().IsRegular() && targetInfo.Mode().IsRegular() {
		same, err := sameLayoutFile(sourcePath, targetPath)
		if err != nil {
			return false, err
		}
		if same {
			if err = os.Remove(sourcePath); err != nil {
				return false, err
			}
			return true, nil
		}
	}
	archivePath, err := nextAvailableLayoutPath(targetPath + ".legacy-config")
	if err != nil {
		return false, err
	}
	return moveLayoutEntry(sourcePath, archivePath)
}

func mergeLegacyHostConfigDirectories(sourcePath string, targetPath string) (bool, error) {
	entries, err := os.ReadDir(sourcePath)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	affected := false
	for _, entry := range entries {
		moved, moveErr := moveLegacyHostConfig(
			filepath.Join(sourcePath, entry.Name()),
			filepath.Join(targetPath, entry.Name()),
		)
		if moveErr != nil {
			return false, moveErr
		}
		affected = affected || moved
	}
	remaining, err := os.ReadDir(sourcePath)
	if errors.Is(err, os.ErrNotExist) {
		return affected, nil
	}
	if err != nil {
		return false, err
	}
	if len(remaining) == 0 {
		if err = os.Remove(sourcePath); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return affected, nil
			}
			return false, err
		}
	}
	return affected, nil
}

func runtimeEntryTarget(runtimeRoot string, name string) string {
	return filepath.Join(runtimeRoot, name)
}

func migrateNestedRuntimeEntries(sourceRoot string, targetRoot string) (int, error) {
	if info, err := os.Lstat(sourceRoot); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			// 不跟随旧 config 根的符号链接；稍后的宿主目录迁移会把
			// 这个链接作为一个整体移动，避免把外部目录内容混入 runtime。
			return 0, nil
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return 0, fmt.Errorf("读取旧 config 根: %w", err)
	}
	affected := 0
	for _, name := range stateLayoutNestedRuntimeEntries {
		sourcePath := filepath.Join(sourceRoot, name)
		targetPath := runtimeEntryTarget(targetRoot, name)
		moved, err := moveNestedRuntimeEntry(sourcePath, targetPath, name)
		if err != nil {
			return affected, fmt.Errorf("迁移旧 config runtime 条目 %q: %w", name, err)
		}
		if moved {
			affected++
		}
	}
	entries, err := os.ReadDir(sourceRoot)
	if errors.Is(err, os.ErrNotExist) {
		return affected, nil
	}
	if err != nil {
		return affected, fmt.Errorf("读取旧 config 未分类条目: %w", err)
	}
	for _, entry := range entries {
		if _, isHostEntry := stateLayoutNestedHostEntries[entry.Name()]; isHostEntry {
			continue
		}
		moved, moveErr := moveNestedRuntimeEntry(
			filepath.Join(sourceRoot, entry.Name()),
			filepath.Join(targetRoot, entry.Name()),
			entry.Name(),
		)
		if moveErr != nil {
			return affected, fmt.Errorf("迁移旧 config 未分类条目 %q: %w", entry.Name(), moveErr)
		}
		if moved {
			affected++
		}
	}
	return affected, nil
}

// 旧 config 目录中的 runtime 条目可能同时存在于状态根。状态根版本
// 通常是较新的运行态；遇到差异时保留旧版本副本，避免升级被无意义
// 的配置冲突阻断，也不静默丢弃用户数据。
func moveNestedRuntimeEntry(sourcePath string, targetPath string, _ string) (bool, error) {
	return retryMissingLayoutSource(sourcePath, func() (bool, error) {
		return moveNestedRuntimeEntryOnce(sourcePath, targetPath)
	})
}

func moveNestedRuntimeEntryOnce(sourcePath string, targetPath string) (bool, error) {
	sourceInfo, sourceErr := os.Lstat(sourcePath)
	if errors.Is(sourceErr, os.ErrNotExist) {
		return false, nil
	}
	if sourceErr != nil {
		return false, sourceErr
	}
	if _, targetErr := os.Lstat(targetPath); errors.Is(targetErr, os.ErrNotExist) {
		return moveLayoutEntry(sourcePath, targetPath)
	} else if targetErr != nil {
		return false, targetErr
	}
	targetInfo, targetErr := os.Lstat(targetPath)
	if targetErr != nil {
		return false, targetErr
	}
	if sourceInfo.Mode().IsRegular() && targetInfo.Mode().IsRegular() {
		same, compareErr := sameLayoutFile(sourcePath, targetPath)
		if compareErr != nil {
			return false, compareErr
		}
		if same {
			if removeErr := os.Remove(sourcePath); removeErr != nil {
				return false, removeErr
			}
			return true, nil
		}
	}
	if sourceInfo.IsDir() && targetInfo.IsDir() {
		return mergeLegacyHostConfigDirectories(sourcePath, targetPath)
	}
	suffixPath, err := nextAvailableLayoutPath(targetPath + ".legacy-config")
	if err != nil {
		return false, err
	}
	return moveLayoutEntry(sourcePath, suffixPath)
}

func nextAvailableLayoutPath(basePath string) (string, error) {
	if _, err := os.Lstat(basePath); errors.Is(err, os.ErrNotExist) {
		return basePath, nil
	} else if err != nil {
		return "", err
	}
	for index := 2; index < 10000; index++ {
		candidate := fmt.Sprintf("%s.%d", basePath, index)
		if _, err := os.Lstat(candidate); errors.Is(err, os.ErrNotExist) {
			return candidate, nil
		} else if err != nil {
			return "", err
		}
	}
	return "", fmt.Errorf("无法为布局迁移生成唯一备份路径 %q", basePath)
}

// moveUnknownStateEntries 把旧状态根中未识别的条目保守归入系统用户 runtime。
//
// Nexus 宿主条目数量有限且由 stateLayoutHostEntries 明确维护；Claude/nxs
// 会持续增加用户级文件。默认归 runtime 能避免升级后把新版本用户配置误放
// 到 app 控制面，后续仍可基于明确证据重新归属。
func moveUnknownStateEntries(stateRoot string, systemRuntimeRoot string) error {
	entries, err := os.ReadDir(stateRoot)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("读取状态根待迁移条目: %w", err)
	}

	// Control 与公钥镜像是当前布局的一等顶层目录，不属于旧 runtime。
	skip := map[string]struct{}{
		"app":               {},
		"control":           {},
		"control-public":    {},
		"users":             {},
		"shared-workspaces": {},
		"workspace":         {},
		// 桌面壳在 sidecar 启动前已经持有这些文件，启动中移动会破坏单实例锁语义。
		"NexusDesktop.lock":     {},
		"NexusSidecar.pid.json": {},
	}
	hostNames := make(map[string]struct{}, len(stateLayoutHostEntries))
	for _, name := range stateLayoutHostEntries {
		hostNames[name] = struct{}{}
	}
	runtimeNames := make(map[string]struct{}, len(stateLayoutRuntimeEntries))
	for _, name := range stateLayoutRuntimeEntries {
		runtimeNames[name] = struct{}{}
	}
	for _, entry := range entries {
		name := entry.Name()
		if _, ok := skip[name]; ok {
			continue
		}
		if _, ok := hostNames[name]; ok {
			continue
		}
		if _, ok := runtimeNames[name]; ok {
			continue
		}
		moved, moveErr := moveLayoutEntry(
			filepath.Join(stateRoot, name),
			filepath.Join(systemRuntimeRoot, name),
		)
		if moveErr != nil {
			return fmt.Errorf("迁移未分类 runtime 条目 %q: %w", name, moveErr)
		}
		_ = moved
	}
	return nil
}

func hardenLayoutTree(root string) error {
	info, err := os.Lstat(root)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil
	}
	return filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			if errors.Is(walkErr, os.ErrNotExist) {
				return nil
			}
			return walkErr
		}
		modeType := entry.Type()
		if modeType&os.ModeSymlink != 0 {
			return nil
		}
		info, infoErr := entry.Info()
		if infoErr != nil {
			if errors.Is(infoErr, os.ErrNotExist) {
				return nil
			}
			return infoErr
		}
		permissions := info.Mode().Perm() & 0o700
		if entry.IsDir() {
			if permissions == 0 {
				permissions = 0o700
			}
		} else if permissions == 0 && info.Mode().IsRegular() {
			permissions = 0o600
		}
		if info.Mode().Perm() != permissions {
			if chmodErr := os.Chmod(path, permissions); chmodErr != nil {
				if errors.Is(chmodErr, os.ErrNotExist) {
					return nil
				}
				return chmodErr
			}
		}
		return nil
	})
}

func hardenStateRoot(stateRoot string) error {
	clean := filepath.Clean(stateRoot)
	if clean == string(filepath.Separator) || clean == "." {
		return nil
	}
	if err := os.Chmod(clean, 0o700); err != nil {
		return fmt.Errorf("收紧状态根权限: %w", err)
	}
	return nil
}

func moveLayoutEntry(sourcePath string, targetPath string) (bool, error) {
	return retryMissingLayoutSource(sourcePath, func() (bool, error) {
		return moveLayoutEntryOnce(sourcePath, targetPath)
	})
}

// removeLegacyCache 删除旧 cache。
//
// 桌面壳会在 sidecar 启动前初始化 WebView 缓存和版本标记，因此新旧 cache
// 同时存在是正常升级状态。cache 不承载 Nexus 业务真相且全部可再生，旧目录
// 直接删除，不能用内容冲突阻断数据库、workspace 与 Room 的后续迁移。
func removeLegacyCache(sourcePath string) (bool, error) {
	if _, err := os.Lstat(sourcePath); errors.Is(err, os.ErrNotExist) {
		return false, nil
	} else if err != nil {
		return false, fmt.Errorf("读取旧缓存根 %q: %w", sourcePath, err)
	}
	if err := os.RemoveAll(sourcePath); err != nil {
		return false, fmt.Errorf("清理可再生的旧缓存 %q: %w", sourcePath, err)
	}
	return true, nil
}

// retryMissingLayoutSource 将迁移期间的瞬态 ENOENT 与真实错误区分开。
//
// runtime 可能在迁移遍历目录时清理旧 session：目录项已经被 ReadDir
// 返回，但下一次 Lstat、hash 或 rename 时源路径已消失。源确实消失时，
// 说明另一个 writer 已经完成了删除/移动，本次迁移无需再阻断启动；
// 如果源仍存在，则仅重试有限次数，保留原始错误便于定位。
func retryMissingLayoutSource(sourcePath string, operation func() (bool, error)) (bool, error) {
	moved := false
	var lastErr error
	for attempt := 0; attempt < layoutMoveRetryAttempts; attempt++ {
		currentMoved, err := operation()
		moved = moved || currentMoved
		if err == nil {
			return moved, nil
		}
		lastErr = err
		if !errors.Is(err, os.ErrNotExist) {
			return moved, err
		}

		_, sourceErr := os.Lstat(sourcePath)
		if errors.Is(sourceErr, os.ErrNotExist) {
			return moved, nil
		}
		if sourceErr != nil {
			return moved, err
		}
	}
	return moved, lastErr
}

func moveLayoutEntryOnce(sourcePath string, targetPath string) (bool, error) {
	if sameLayoutPath(sourcePath, targetPath) {
		return false, nil
	}
	sourceInfo, err := os.Lstat(sourcePath)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("读取源路径 %q: %w", sourcePath, err)
	}

	targetInfo, targetErr := os.Lstat(targetPath)
	if errors.Is(targetErr, os.ErrNotExist) {
		if err = os.MkdirAll(filepath.Dir(targetPath), 0o700); err != nil {
			return false, fmt.Errorf("创建目标父目录 %q: %w", targetPath, err)
		}
		if err = os.Rename(sourcePath, targetPath); err != nil {
			return false, fmt.Errorf("移动 %q 到 %q: %w", sourcePath, targetPath, err)
		}
		return true, nil
	}
	if targetErr != nil {
		return false, fmt.Errorf("读取目标路径 %q: %w", targetPath, targetErr)
	}

	if isRoomOverlayPath(sourcePath) &&
		sourceInfo.Mode().IsRegular() &&
		targetInfo.Mode().IsRegular() {
		handled, moved, mergeErr := mergeRoomOverlayFiles(sourcePath, targetPath)
		if mergeErr != nil {
			return false, mergeErr
		}
		if handled {
			return moved, nil
		}
	}

	if isFinderMetadataPath(sourcePath) &&
		sourceInfo.Mode().IsRegular() &&
		targetInfo.Mode().IsRegular() {
		// .DS_Store 是 Finder 可再生的目录布局缓存，不属于 Nexus
		// 业务状态。目标已有一份时保留目标并丢弃旧源，避免不同
		// 机器或不同时间生成的缓存阻断宿主迁移。
		if removeErr := os.Remove(sourcePath); removeErr != nil {
			return false, removeErr
		}
		return true, nil
	}

	if sourceInfo.IsDir() && targetInfo.IsDir() {
		return mergeLayoutDirectories(sourcePath, targetPath)
	}
	if sourceInfo.Mode()&os.ModeSymlink != 0 && targetInfo.Mode()&os.ModeSymlink != 0 {
		sourceLink, sourceLinkErr := os.Readlink(sourcePath)
		targetLink, targetLinkErr := os.Readlink(targetPath)
		if sourceLinkErr == nil && targetLinkErr == nil && sourceLink == targetLink {
			if removeErr := os.Remove(sourcePath); removeErr != nil {
				return false, removeErr
			}
			return true, nil
		}
	}
	if sourceInfo.Mode().IsRegular() && targetInfo.Mode().IsRegular() {
		same, compareErr := sameLayoutFile(sourcePath, targetPath)
		if compareErr != nil {
			return false, compareErr
		}
		if same {
			if removeErr := os.Remove(sourcePath); removeErr != nil {
				return false, removeErr
			}
			return true, nil
		}
	}
	return false, fmt.Errorf("目标路径冲突且内容不同: source=%q target=%q", sourcePath, targetPath)
}

func isFinderMetadataPath(path string) bool {
	return filepath.Base(path) == ".DS_Store"
}

func isRoomOverlayPath(path string) bool {
	clean := filepath.Clean(path)
	if filepath.Base(clean) != "overlay.jsonl" {
		return false
	}
	roomDirectory := filepath.Dir(clean)
	return filepath.Base(filepath.Dir(roomDirectory)) == "rooms"
}

// mergeRoomOverlayFiles 合并旧、新 Room overlay 的行集合。
//
// Room overlay 只追加消息和 transcript 引用，不执行 Agent overlay 的
// rewrite/prune。迁移时保留目标现有内容，只追加源独有行，避免覆盖
// 迁移期间目标侧的新写入；两边的独有历史都得到保留。无法证明一方
// 包含另一方时不擅自猜测，交回通用冲突保护。
func mergeRoomOverlayFiles(sourcePath string, targetPath string) (bool, bool, error) {
	sourceInfo, err := os.Stat(sourcePath)
	if err != nil {
		return false, false, fmt.Errorf("读取 Room overlay 源文件 %q: %w", sourcePath, err)
	}
	targetInfo, err := os.Stat(targetPath)
	if err != nil {
		return false, false, fmt.Errorf("读取 Room overlay 目标文件 %q: %w", targetPath, err)
	}
	if sourceInfo.Size() > maxRoomOverlayMergeBytes ||
		targetInfo.Size() > maxRoomOverlayMergeBytes {
		return false, false, nil
	}

	sourceData, err := os.ReadFile(sourcePath)
	if err != nil {
		return false, false, fmt.Errorf("读取 Room overlay 源内容 %q: %w", sourcePath, err)
	}
	targetData, err := os.ReadFile(targetPath)
	if err != nil {
		return false, false, fmt.Errorf("读取 Room overlay 目标内容 %q: %w", targetPath, err)
	}
	sourceLines := layoutTextLines(sourceData)
	targetLines := layoutTextLines(targetData)

	switch {
	case containsLayoutLines(sourceLines, targetLines):
		// 旧源已经包含目标的全部行；目标可能只是迁移期间预创建的
		// 子集，追加源独有历史但不覆盖目标。
		if err := appendMissingRoomOverlayLines(
			targetPath,
			targetData,
			missingLayoutLines(sourceLines, targetLines),
		); err != nil {
			return false, false, err
		}
	case containsLayoutLines(targetLines, sourceLines):
		// 目标已经包含旧源的全部行，不需要重写目标。
	default:
		if !equivalentRoomOverlayTimestamps(sourceLines, targetLines) {
			return false, false, nil
		}
	}

	if err := os.Remove(sourcePath); err != nil {
		return false, false, fmt.Errorf("删除已合并的 Room overlay 源文件 %q: %w", sourcePath, err)
	}
	return true, true, nil
}

func equivalentRoomOverlayTimestamps(sourceLines []string, targetLines []string) bool {
	if len(sourceLines) != len(targetLines) {
		return false
	}
	for index := range sourceLines {
		sourceKey, sourceOK := canonicalRoomOverlayLine(sourceLines[index])
		targetKey, targetOK := canonicalRoomOverlayLine(targetLines[index])
		if !sourceOK || !targetOK || sourceKey != targetKey {
			return false
		}
	}
	return true
}

func canonicalRoomOverlayLine(line string) (string, bool) {
	var row map[string]json.RawMessage
	if err := json.Unmarshal([]byte(line), &row); err != nil {
		return "", false
	}
	var messageID string
	if err := json.Unmarshal(row["message_id"], &messageID); err != nil ||
		strings.TrimSpace(messageID) == "" {
		return "", false
	}
	delete(row, "timestamp")
	payload, err := json.Marshal(row)
	if err != nil {
		return "", false
	}
	return string(payload), true
}

func layoutTextLines(data []byte) []string {
	if len(data) == 0 {
		return nil
	}
	text := strings.TrimSuffix(string(data), "\n")
	if text == "" {
		return nil
	}
	return strings.Split(text, "\n")
}

func containsLayoutLines(container []string, candidate []string) bool {
	counts := make(map[string]int, len(container))
	for _, line := range container {
		counts[line]++
	}
	for _, line := range candidate {
		if counts[line] == 0 {
			return false
		}
		counts[line]--
	}
	return true
}

func missingLayoutLines(candidate []string, container []string) []string {
	counts := make(map[string]int, len(container))
	for _, line := range container {
		counts[line]++
	}
	missing := make([]string, 0)
	for _, line := range candidate {
		if counts[line] > 0 {
			counts[line]--
			continue
		}
		missing = append(missing, line)
	}
	return missing
}

func appendMissingRoomOverlayLines(
	path string,
	existingData []byte,
	lines []string,
) error {
	if len(lines) == 0 {
		return nil
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		return fmt.Errorf("打开 Room overlay 目标文件 %q: %w", path, err)
	}
	defer file.Close()

	if len(existingData) > 0 && existingData[len(existingData)-1] != '\n' {
		if _, err = file.WriteString("\n"); err != nil {
			return fmt.Errorf("补齐 Room overlay 行尾 %q: %w", path, err)
		}
	}
	for _, line := range lines {
		if _, err = file.WriteString(line + "\n"); err != nil {
			return fmt.Errorf("追加 Room overlay 行 %q: %w", path, err)
		}
	}
	if err = file.Sync(); err != nil {
		return fmt.Errorf("同步 Room overlay 目标文件 %q: %w", path, err)
	}
	return nil
}

func sameLayoutPath(left string, right string) bool {
	left = filepath.Clean(left)
	right = filepath.Clean(right)
	if os.PathSeparator == '\\' {
		return strings.EqualFold(left, right)
	}
	return left == right
}

func mergeLayoutDirectories(sourcePath string, targetPath string) (bool, error) {
	entries, err := os.ReadDir(sourcePath)
	if err != nil {
		return false, fmt.Errorf("读取待合并目录 %q: %w", sourcePath, err)
	}
	affected := false
	for _, entry := range entries {
		moved, moveErr := moveLayoutEntry(
			filepath.Join(sourcePath, entry.Name()),
			filepath.Join(targetPath, entry.Name()),
		)
		if moveErr != nil {
			return false, moveErr
		}
		affected = affected || moved
	}
	remaining, readErr := os.ReadDir(sourcePath)
	if readErr != nil {
		return false, fmt.Errorf("检查待合并目录 %q: %w", sourcePath, readErr)
	}
	if len(remaining) == 0 {
		if removeErr := os.Remove(sourcePath); removeErr != nil {
			return false, fmt.Errorf("删除空源目录 %q: %w", sourcePath, removeErr)
		}
	}
	return affected, nil
}

func sameLayoutFile(sourcePath string, targetPath string) (bool, error) {
	sourceInfo, err := os.Stat(sourcePath)
	if err != nil {
		return false, fmt.Errorf("读取源文件 %q: %w", sourcePath, err)
	}
	targetInfo, err := os.Stat(targetPath)
	if err != nil {
		return false, fmt.Errorf("读取目标文件 %q: %w", targetPath, err)
	}
	if sourceInfo.Size() != targetInfo.Size() {
		return false, nil
	}
	sourceHash, err := layoutFileHash(sourcePath)
	if err != nil {
		return false, err
	}
	targetHash, err := layoutFileHash(targetPath)
	if err != nil {
		return false, err
	}
	return sourceHash == targetHash, nil
}

func layoutFileHash(path string) ([sha256.Size]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return [sha256.Size]byte{}, fmt.Errorf("打开文件 %q: %w", path, err)
	}
	defer file.Close()

	hash := sha256.New()
	if _, err = io.Copy(hash, file); err != nil {
		return [sha256.Size]byte{}, fmt.Errorf("读取文件 %q: %w", path, err)
	}
	var result [sha256.Size]byte
	copy(result[:], hash.Sum(nil))
	return result, nil
}

func layoutMigrationApplied(markerPath string) (bool, error) {
	content, err := os.ReadFile(markerPath)
	if err == nil {
		if string(content) != "completed\n" {
			return false, fmt.Errorf("状态布局迁移标记内容无效 %q", markerPath)
		}
		return true, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return false, fmt.Errorf("检查状态布局迁移标记 %q: %w", markerPath, err)
	}
	return false, nil
}

func writeLayoutMigrationMarker(markerPath string) error {
	if err := os.MkdirAll(filepath.Dir(markerPath), 0o700); err != nil {
		return fmt.Errorf("创建状态布局迁移标记目录: %w", err)
	}
	temporaryPath := markerPath + ".tmp"
	if err := os.WriteFile(temporaryPath, []byte("completed\n"), 0o600); err != nil {
		return fmt.Errorf("写入状态布局迁移临时标记: %w", err)
	}
	if err := os.Chmod(temporaryPath, 0o600); err != nil {
		_ = os.Remove(temporaryPath)
		return fmt.Errorf("收紧状态布局迁移临时标记权限: %w", err)
	}
	if err := os.Rename(temporaryPath, markerPath); err != nil {
		_ = os.Remove(temporaryPath)
		return fmt.Errorf("提交状态布局迁移标记: %w", err)
	}
	return nil
}
