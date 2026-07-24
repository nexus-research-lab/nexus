//go:build linux

package runtimeidentity

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

func loadLauncherConfig() (launcherConfig, error) {
	configPath := defaultConfigPath
	// 自定义配置只允许真正的 root 进程使用。setuid 调用中 real uid 仍是
	// nexus-host，不能通过环境变量把受信任配置换成 runtime 可写文件。
	if os.Getuid() == 0 && os.Geteuid() == 0 {
		if override := strings.TrimSpace(os.Getenv("NEXUS_RUNTIME_ISOLATION_CONFIG")); override != "" {
			configPath = filepath.Clean(override)
		}
	}
	data, err := readTrustedLauncherConfig(configPath)
	if err != nil {
		return launcherConfig{}, fmt.Errorf("读取 launcher 配置 %q: %w", configPath, err)
	}
	var config launcherConfig
	if err = json.Unmarshal(data, &config); err != nil {
		return launcherConfig{}, fmt.Errorf("解析 launcher 配置: %w", err)
	}
	if err = normalizeLauncherConfig(&config); err != nil {
		return launcherConfig{}, err
	}
	return config, nil
}

func readTrustedLauncherConfig(path string) ([]byte, error) {
	fd, err := openRegularNoSymlink(path, unix.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(fd), path)
	if file == nil {
		_ = unix.Close(fd)
		return nil, errors.New("打开 launcher 配置失败")
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Uid != 0 {
		return nil, errors.New("launcher 配置必须由 root 持有")
	}
	if info.Mode().Perm()&0o022 != 0 {
		return nil, errors.New("launcher 配置不能被 group/other 写入")
	}
	return io.ReadAll(file)
}

func normalizeLauncherConfig(config *launcherConfig) error {
	if config == nil {
		return errors.New("launcher 配置为空")
	}
	if config.HostUID <= 0 || config.HostGID <= 0 {
		return errors.New("launcher host uid/gid 必须是正整数")
	}
	if config.UIDMinimum == 0 {
		config.UIDMinimum = defaultUIDMinimum
	}
	if config.UIDMaximum == 0 {
		config.UIDMaximum = defaultUIDMaximum
	}
	if config.UIDMinimum < 1000 || config.UIDMaximum < config.UIDMinimum {
		return errors.New("launcher uid/gid 范围无效")
	}
	stateRoot, err := canonicalExistingOrPendingPath(config.StateRoot)
	if err != nil {
		return fmt.Errorf("解析 state root: %w", err)
	}
	if stateRoot == string(filepath.Separator) {
		return errors.New("state root 不能是文件系统根")
	}
	config.StateRoot = stateRoot
	if err = normalizeCgroupConfig(config); err != nil {
		return err
	}
	if config.TicketTTLSeconds <= 0 {
		config.TicketTTLSeconds = int(defaultTicketTTL / time.Second)
	}
	if config.TicketTTLSeconds > int((7*24*time.Hour)/time.Second) {
		return errors.New("launcher ticket ttl 不能超过 7 天")
	}
	if len(config.RuntimeExecutables) == 0 {
		return errors.New("launcher runtime executable allowlist 为空")
	}
	normalizedExecutables := make(map[string]string, len(config.RuntimeExecutables))
	for kind, path := range config.RuntimeExecutables {
		kind = strings.ToLower(strings.TrimSpace(kind))
		path = strings.TrimSpace(path)
		if kind == "" || path == "" || !filepath.IsAbs(path) {
			return fmt.Errorf("runtime executable 配置无效: kind=%q path=%q", kind, path)
		}
		canonical, canonicalErr := canonicalExistingOrPendingPath(path)
		if canonicalErr != nil {
			return fmt.Errorf("解析 %s runtime executable: %w", kind, canonicalErr)
		}
		for _, writableStateRoot := range []string{
			filepath.Join(config.StateRoot, "users"),
			filepath.Join(config.StateRoot, "shared-workspaces"),
		} {
			if pathWithin(canonical, writableStateRoot) {
				return fmt.Errorf("runtime executable 不能位于用户可写状态树: %s", canonical)
			}
		}
		normalizedExecutables[kind] = canonical
	}
	config.RuntimeExecutables = normalizedExecutables
	roots := make([]string, 0, len(config.ReadOnlyRoots))
	for _, root := range config.ReadOnlyRoots {
		if strings.TrimSpace(root) == "" {
			continue
		}
		canonical, canonicalErr := canonicalExistingOrPendingPath(root)
		if canonicalErr != nil {
			return fmt.Errorf("解析只读根 %q: %w", root, canonicalErr)
		}
		if err = validateConfiguredReadOnlyRoot(config.StateRoot, canonical); err != nil {
			return err
		}
		roots = append(roots, canonical)
	}
	config.ReadOnlyRoots = compactPaths(roots)
	return nil
}

func validateConfiguredReadOnlyRoot(stateRoot string, root string) error {
	for _, broadSystemRoot := range []string{
		"/bin", "/boot", "/dev", "/etc", "/home", "/lib", "/lib64", "/opt",
		"/proc", "/root", "/run", "/sbin", "/srv", "/sys", "/tmp", "/usr",
		"/usr/local", "/var",
	} {
		if root == filepath.Clean(broadSystemRoot) {
			return fmt.Errorf("只读根不能直接覆盖宽泛系统目录: %s", root)
		}
	}
	for _, denied := range []string{
		filepath.Join(stateRoot, "users"),
		filepath.Join(stateRoot, "shared-workspaces"),
		filepath.Join(stateRoot, "app", "cache"),
		filepath.Join(stateRoot, "app", "config"),
		filepath.Join(stateRoot, "app", "data"),
		filepath.Join(stateRoot, "app", "logs"),
		filepath.Join(stateRoot, "app", "rooms"),
	} {
		if pathWithin(root, denied) || pathWithin(denied, root) {
			return fmt.Errorf("只读根不能覆盖 runtime 或宿主敏感状态: %s", root)
		}
	}
	return nil
}

func verifyTrustedCaller(config launcherConfig) error {
	if os.Geteuid() != 0 {
		return errors.New("runtime launcher 必须以 root effective uid 运行")
	}
	if err := verifyLauncherBinary(); err != nil {
		return err
	}
	realUID := os.Getuid()
	if realUID == 0 {
		return nil
	}
	if realUID != config.HostUID {
		return fmt.Errorf("拒绝未受信任 launcher 调用方 uid=%d", realUID)
	}
	groups, err := os.Getgroups()
	if err != nil {
		return fmt.Errorf("读取 launcher 调用方 groups: %w", err)
	}
	hasHostGroup := os.Getgid() == config.HostGID
	for _, gid := range groups {
		hasHostGroup = hasHostGroup || gid == config.HostGID
	}
	if !hasHostGroup {
		return fmt.Errorf("launcher 调用方不属于配置的 host gid=%d", config.HostGID)
	}
	if info, err := os.Stat("/proc/self/exe"); err != nil {
		return fmt.Errorf("读取 launcher setuid 状态失败: %w", err)
	} else if info.Mode()&os.ModeSetuid == 0 {
		return errors.New("非 root 调用方必须通过 setuid launcher")
	}
	return nil
}

func verifyLauncherBinary() error {
	executablePath, err := os.Readlink("/proc/self/exe")
	if err != nil {
		return fmt.Errorf("解析 launcher 可执行文件: %w", err)
	}
	info, err := os.Lstat(executablePath)
	if err != nil {
		return fmt.Errorf("读取 launcher 可执行文件: %w", err)
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 ||
		info.Mode().Perm()&0o022 != 0 {
		return errors.New("launcher 可执行文件权限不安全")
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Uid != 0 {
		return errors.New("launcher 可执行文件必须由 root 持有")
	}
	return nil
}
