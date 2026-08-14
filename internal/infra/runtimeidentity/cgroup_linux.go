//go:build linux

package runtimeidentity

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

const (
	cgroupKillFile  = "cgroup.kill"
	cgroupProcsFile = "cgroup.procs"
)

// normalizeCgroupConfig 校验 cgroup 根必须是管理员预先指定的独立路径。
// runtime 不会获得修改该路径或其控制文件的权限。
func normalizeCgroupConfig(config *launcherConfig) error {
	if config == nil {
		return errors.New("launcher 配置为空")
	}
	root := strings.TrimSpace(config.CgroupRoot)
	if root == "" {
		if config.CgroupRequired {
			return errors.New("cgroup_required=true 但 cgroup_root 为空")
		}
		config.CgroupRoot = ""
		return nil
	}
	if !filepath.IsAbs(root) {
		return errors.New("cgroup_root 必须是绝对路径")
	}
	root = filepath.Clean(root)
	if root == string(filepath.Separator) {
		return errors.New("cgroup_root 不能是文件系统根")
	}
	if root == "/sys/fs/cgroup" {
		return errors.New("cgroup_root 必须是 cgroup2 的 Nexus 子目录")
	}
	if pathWithin(root, config.StateRoot) || pathWithin(config.StateRoot, root) {
		return errors.New("cgroup_root 不能与 state_root 重叠")
	}
	if info, err := os.Lstat(root); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return errors.New("cgroup_root 不能是符号链接")
		}
		if !info.IsDir() {
			return errors.New("cgroup_root 必须是目录")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("读取 cgroup_root: %w", err)
	}
	canonical, err := canonicalExistingOrPendingPath(root)
	if err != nil {
		return fmt.Errorf("解析 cgroup_root: %w", err)
	}
	config.CgroupRoot = canonical
	return nil
}

func cgroupEnabled(config launcherConfig) bool {
	return strings.TrimSpace(config.CgroupRoot) != ""
}

// ensureCgroupHost 准备并验证 cgroup2 根。cgroup.kill 是强制回收 orphan
// descendant 的必要能力；缺失时不能把“已启用隔离”降级成仅断开父进程。
func ensureCgroupHost(config launcherConfig) error {
	if !cgroupEnabled(config) {
		if config.CgroupRequired {
			return errors.New("cgroup_required=true 但 cgroup 未配置")
		}
		return nil
	}
	fd, err := ensureDirectoryNoSymlink(config.CgroupRoot, 0o750)
	if err != nil {
		return fmt.Errorf("创建 cgroup_root: %w", err)
	}
	defer unix.Close(fd)
	var filesystem unix.Statfs_t
	if err = unix.Fstatfs(fd, &filesystem); err != nil {
		return fmt.Errorf("读取 cgroup_root 文件系统: %w", err)
	}
	if uint64(filesystem.Type) != uint64(unix.CGROUP2_SUPER_MAGIC) {
		return errors.New("cgroup_root 不是 cgroup v2 文件系统")
	}
	var stat unix.Stat_t
	if err = unix.Fstat(fd, &stat); err != nil {
		return fmt.Errorf("读取 cgroup_root 权限: %w", err)
	}
	if stat.Uid != 0 || stat.Mode&0o022 != 0 {
		return errors.New("cgroup_root 必须由 root 持有且不可被 group/other 写入")
	}
	if err = unix.Fchown(fd, 0, config.HostGID); err != nil {
		return fmt.Errorf("设置 cgroup_root owner: %w", err)
	}
	if err = unix.Fchmod(fd, 0o750); err != nil {
		return fmt.Errorf("设置 cgroup_root 权限: %w", err)
	}
	killFD, err := openCgroupControl(filepath.Join(config.CgroupRoot, cgroupKillFile))
	if err != nil {
		return fmt.Errorf("cgroup v2 不支持 cgroup.kill: %w", err)
	}
	return unix.Close(killFD)
}

func ensureRuntimeCgroup(config launcherConfig, username string) (string, error) {
	if err := ensureCgroupHost(config); err != nil {
		return "", err
	}
	if !cgroupEnabled(config) {
		return "", nil
	}
	path, err := runtimeCgroupPath(config, username)
	if err != nil {
		return "", err
	}
	fd, err := ensureDirectoryNoSymlink(path, 0o700)
	if err != nil {
		return "", fmt.Errorf("创建 runtime cgroup: %w", err)
	}
	defer unix.Close(fd)
	var stat unix.Stat_t
	if err = unix.Fstat(fd, &stat); err != nil {
		return "", err
	}
	if stat.Uid != 0 || stat.Mode&0o022 != 0 {
		return "", errors.New("runtime cgroup 必须由 root 持有且不可被 group/other 写入")
	}
	if err = unix.Fchown(fd, 0, 0); err != nil {
		return "", fmt.Errorf("设置 runtime cgroup owner: %w", err)
	}
	if err = unix.Fchmod(fd, 0o700); err != nil {
		return "", fmt.Errorf("设置 runtime cgroup 权限: %w", err)
	}
	killFD, err := openCgroupControl(filepath.Join(path, cgroupKillFile))
	if err != nil {
		return "", fmt.Errorf("打开 runtime cgroup.kill: %w", err)
	}
	if err = unix.Close(killFD); err != nil {
		return "", err
	}
	return path, nil
}

func attachRuntimeCgroup(config launcherConfig, username string) error {
	path, err := ensureRuntimeCgroup(config, username)
	if err != nil {
		return err
	}
	if path == "" {
		return nil
	}
	if err = writeCgroupControl(
		filepath.Join(path, cgroupProcsFile),
		strconv.Itoa(os.Getpid()),
	); err != nil {
		return fmt.Errorf("把 runtime 加入 cgroup: %w", err)
	}
	return nil
}

// killRuntimeCgroup 使用内核级 cgroup.kill 回收整个 owner cgroup，而不是
// 只向已知父进程发送信号。这样主动 double-fork 的后台子进程也会被收口。
func killRuntimeCgroup(config launcherConfig, username string) error {
	if !cgroupEnabled(config) {
		if config.CgroupRequired {
			return errors.New("cgroup_required=true 但 cgroup 未配置")
		}
		return nil
	}
	path, err := runtimeCgroupPath(config, username)
	if err != nil {
		return err
	}
	if _, err = os.Lstat(path); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	if err = writeCgroupControl(filepath.Join(path, cgroupKillFile), "1"); err != nil {
		return fmt.Errorf("回收 runtime cgroup: %w", err)
	}
	return nil
}

// signalRuntimeSession 只向指定 owner UID/cgroup 内的 Unix session 发信号。
// 没有 cgroup 时稳定 owner UID 仍提供相同租户边界；强杀会同时清理该 session
// 的遗留子进程。
func signalRuntimeSession(
	config launcherConfig,
	value *identity,
	sessionID int,
	signal syscall.Signal,
	sessionWide bool,
) error {
	if value == nil || sessionID <= 1 {
		return errors.New("runtime process identity 或 pid 无效")
	}
	if !sessionWide {
		exists, matched, err := signalManagedRuntimeProcess(
			config,
			value,
			sessionID,
			sessionID,
			signal,
		)
		if err != nil {
			return err
		}
		if exists && !matched {
			return errors.New("runtime pid 不属于当前 owner 的 session leader")
		}
		return nil
	}
	processIDs, err := managedRuntimeProcessIDs(config, value)
	if err != nil {
		return err
	}
	matched := false
	for _, processID := range processIDs {
		if processID == sessionID {
			continue
		}
		_, processMatched, signalErr := signalManagedRuntimeProcess(
			config,
			value,
			processID,
			sessionID,
			signal,
		)
		if signalErr != nil {
			return signalErr
		}
		matched = matched || processMatched
	}
	exists, leaderMatched, err := signalManagedRuntimeProcess(
		config,
		value,
		sessionID,
		sessionID,
		signal,
	)
	if err != nil {
		return err
	}
	if exists && !leaderMatched && !matched {
		return errors.New("runtime pid 不属于当前 owner 的 session")
	}
	return nil
}

// signalManagedRuntimeProcess 先固定 pidfd，再校验 owner 和 session，避免 PID
// 复用竞态把 root launcher 的信号发给无关进程。
func signalManagedRuntimeProcess(
	config launcherConfig,
	value *identity,
	processID int,
	sessionID int,
	signal syscall.Signal,
) (bool, bool, error) {
	processFD, err := unix.PidfdOpen(processID, 0)
	if errors.Is(err, unix.ESRCH) {
		return false, false, nil
	}
	if err != nil {
		return false, false, fmt.Errorf("打开 runtime pidfd %d: %w", processID, err)
	}
	defer unix.Close(processFD)

	managed, err := runtimeProcessManaged(config, value, processID)
	if err != nil {
		return true, false, err
	}
	actualSessionID, sessionErr := unix.Getsid(processID)
	if errors.Is(sessionErr, unix.ESRCH) {
		return false, false, nil
	}
	if sessionErr != nil {
		return true, false, fmt.Errorf("读取 runtime process %d session: %w", processID, sessionErr)
	}
	if !managed || actualSessionID != sessionID {
		if aliveErr := unix.PidfdSendSignal(processFD, 0, nil, 0); errors.Is(aliveErr, unix.ESRCH) {
			return false, false, nil
		} else if aliveErr != nil {
			return true, false, fmt.Errorf("检查 runtime process %d: %w", processID, aliveErr)
		}
		return true, false, nil
	}
	if err = unix.PidfdSendSignal(processFD, unix.Signal(signal), nil, 0); err != nil &&
		!errors.Is(err, unix.ESRCH) {
		return true, true, fmt.Errorf("发送 runtime process %d 信号: %w", processID, err)
	}
	return true, true, nil
}

func runtimeProcessManaged(config launcherConfig, value *identity, processID int) (bool, error) {
	if cgroupEnabled(config) {
		path, err := runtimeCgroupPath(config, value.Username)
		if err != nil {
			return false, err
		}
		data, err := os.ReadFile(filepath.Join(path, cgroupProcsFile))
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		if err != nil {
			return false, fmt.Errorf("读取 runtime cgroup 进程: %w", err)
		}
		processIDs, err := parseProcessIDs(string(data))
		return slices.Contains(processIDs, processID), err
	}
	var stat unix.Stat_t
	if err := unix.Stat(filepath.Join("/proc", strconv.Itoa(processID)), &stat); err != nil {
		if errors.Is(err, unix.ENOENT) {
			return false, nil
		}
		return false, fmt.Errorf("读取 runtime process %d: %w", processID, err)
	}
	return stat.Uid == uint32(value.UID), nil
}

func managedRuntimeProcessIDs(config launcherConfig, value *identity) ([]int, error) {
	if cgroupEnabled(config) {
		path, err := runtimeCgroupPath(config, value.Username)
		if err != nil {
			return nil, err
		}
		data, err := os.ReadFile(filepath.Join(path, cgroupProcsFile))
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		if err != nil {
			return nil, fmt.Errorf("读取 runtime cgroup 进程: %w", err)
		}
		return parseProcessIDs(string(data))
	}
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, fmt.Errorf("读取 proc: %w", err)
	}
	processIDs := make([]int, 0)
	for _, entry := range entries {
		processID, parseErr := strconv.Atoi(entry.Name())
		if parseErr != nil || processID <= 1 {
			continue
		}
		var stat unix.Stat_t
		if statErr := unix.Stat(filepath.Join("/proc", entry.Name()), &stat); statErr != nil {
			if errors.Is(statErr, unix.ENOENT) {
				continue
			}
			return nil, fmt.Errorf("读取 runtime process %d: %w", processID, statErr)
		}
		if stat.Uid == uint32(value.UID) {
			processIDs = append(processIDs, processID)
		}
	}
	slices.Sort(processIDs)
	return processIDs, nil
}

func parseProcessIDs(data string) ([]int, error) {
	processIDs := make([]int, 0)
	for _, field := range strings.Fields(data) {
		processID, err := strconv.Atoi(field)
		if err != nil || processID <= 1 {
			return nil, fmt.Errorf("runtime cgroup pid 无效: %q", field)
		}
		processIDs = append(processIDs, processID)
	}
	slices.Sort(processIDs)
	return slices.Compact(processIDs), nil
}

func runtimeCgroupPath(config launcherConfig, username string) (string, error) {
	username = strings.TrimSpace(username)
	if username == "" || username == "." || username == ".." ||
		filepath.Base(username) != username ||
		strings.ContainsAny(username, `/\`) {
		return "", errors.New("runtime cgroup username 无效")
	}
	root := filepath.Clean(strings.TrimSpace(config.CgroupRoot))
	if root == "" || !filepath.IsAbs(root) {
		return "", errors.New("cgroup_root 未配置")
	}
	return filepath.Join(root, username), nil
}

func openCgroupControl(path string) (int, error) {
	path = filepath.Clean(strings.TrimSpace(path))
	if path == "" || !filepath.IsAbs(path) {
		return -1, errors.New("cgroup control path 必须是绝对路径")
	}
	how := &unix.OpenHow{
		Flags:   uint64(unix.O_WRONLY | unix.O_CLOEXEC | unix.O_NOFOLLOW | unix.O_NONBLOCK),
		Resolve: unix.RESOLVE_NO_SYMLINKS,
	}
	fd, err := unix.Openat2(unix.AT_FDCWD, path, how)
	if errors.Is(err, unix.ENOSYS) || errors.Is(err, unix.EINVAL) {
		return -1, fmt.Errorf("内核不支持无符号链接 cgroup control: %w", err)
	}
	return fd, err
}

func writeCgroupControl(path string, value string) error {
	fd, err := openCgroupControl(path)
	if err != nil {
		return err
	}
	defer unix.Close(fd)
	payload := []byte(strings.TrimSpace(value) + "\n")
	written, err := unix.Write(fd, payload)
	if err != nil {
		return err
	}
	if written != len(payload) {
		return io.ErrShortWrite
	}
	return nil
}
