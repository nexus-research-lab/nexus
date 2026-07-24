//go:build linux

package runtimeidentity

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"golang.org/x/sys/unix"
)

func withLockedRegistry[T any](
	config launcherConfig,
	mutate bool,
	callback func(*registry) (T, bool, error),
) (T, error) {
	var zero T
	if err := ensureRegistryLayout(config); err != nil {
		return zero, err
	}
	root := registryRoot(config)
	lockFD, err := openRegularNoSymlink(
		filepath.Join(root, "registry.lock"),
		unix.O_RDWR|unix.O_CREAT,
		0o600,
	)
	if err != nil {
		return zero, err
	}
	lock := os.NewFile(uintptr(lockFD), filepath.Join(root, "registry.lock"))
	if lock == nil {
		_ = unix.Close(lockFD)
		return zero, errors.New("创建 registry lock 文件失败")
	}
	defer lock.Close()
	if err = lock.Chown(0, 0); err != nil {
		return zero, err
	}
	if err = lock.Chmod(0o600); err != nil {
		return zero, err
	}
	if err = unix.Flock(int(lock.Fd()), unix.LOCK_EX); err != nil {
		return zero, err
	}
	defer unix.Flock(int(lock.Fd()), unix.LOCK_UN)

	current, err := readRegistry(config)
	if err != nil {
		return zero, err
	}
	result, changed, err := callback(current)
	if err != nil {
		return zero, err
	}
	if changed {
		if !mutate {
			return zero, errors.New("只读 registry 操作试图修改状态")
		}
		if err = writeRegistry(config, current); err != nil {
			return zero, err
		}
	}
	return result, nil
}

func readRegistry(config launcherConfig) (*registry, error) {
	path := registryPath(config)
	data, err := readRootOwnedFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return &registry{
			Version:    registryVersion,
			NextID:     config.UIDMinimum,
			Identities: map[string]*identity{},
			Projects:   map[string]*project{},
		}, nil
	}
	if err != nil {
		return nil, err
	}
	var current registry
	if err = json.Unmarshal(data, &current); err != nil {
		return nil, fmt.Errorf("解析 runtime identity registry: %w", err)
	}
	if current.Version != registryVersion {
		return nil, fmt.Errorf("不支持的 runtime identity registry version: %d", current.Version)
	}
	if current.Identities == nil {
		current.Identities = map[string]*identity{}
	}
	if current.Projects == nil {
		current.Projects = map[string]*project{}
	}
	if current.NextID < config.UIDMinimum {
		current.NextID = config.UIDMinimum
	}
	if err = validateRegistry(config, &current); err != nil {
		return nil, err
	}
	return &current, nil
}

func writeRegistry(config launcherConfig, current *registry) error {
	if current == nil {
		return errors.New("runtime identity registry 为空")
	}
	if err := validateRegistry(config, current); err != nil {
		return err
	}
	data, err := json.MarshalIndent(current, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	root := registryRoot(config)
	temporary, err := os.CreateTemp(root, ".registry-*.json")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err = temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return err
	}
	if err = temporary.Chown(0, 0); err != nil {
		temporary.Close()
		return err
	}
	if _, err = temporary.Write(data); err != nil {
		temporary.Close()
		return err
	}
	if err = temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if err = temporary.Close(); err != nil {
		return err
	}
	if err = os.Rename(temporaryPath, registryPath(config)); err != nil {
		return err
	}
	directory, err := os.Open(root)
	if err == nil {
		err = directory.Sync()
		_ = directory.Close()
	}
	return err
}

func validateRegistry(config launcherConfig, current *registry) error {
	if current == nil {
		return errors.New("runtime identity registry 为空")
	}
	if current.Version != registryVersion {
		return fmt.Errorf("不支持的 runtime identity registry version: %d", current.Version)
	}
	if current.Identities == nil || current.Projects == nil {
		return errors.New("runtime identity registry map 为空")
	}
	if current.NextID < config.UIDMinimum || current.NextID > config.UIDMaximum {
		return errors.New("runtime identity registry next id 越界")
	}

	usedIDs := make(map[int]string, len(current.Identities)*2+len(current.Projects))
	registerID := func(value int, owner string) error {
		if value < config.UIDMinimum || value > config.UIDMaximum {
			return fmt.Errorf("registry %s 使用了越界 UID/GID: %d", owner, value)
		}
		if value == config.HostUID || value == config.HostGID {
			return fmt.Errorf("registry %s 与 host UID/GID 冲突: %d", owner, value)
		}
		if previous, exists := usedIDs[value]; exists && previous != owner {
			return fmt.Errorf("registry UID/GID 冲突: %s 与 %s 共用 %d", previous, owner, value)
		}
		usedIDs[value] = owner
		return nil
	}

	for ownerUserID, value := range current.Identities {
		if value == nil {
			return fmt.Errorf("registry identity %q 为空", ownerUserID)
		}
		if ownerUserID == "" || ownerUserID != value.OwnerUserID ||
			ownerUserID == "." || ownerUserID == ".." ||
			appfs.UserPathSegment(ownerUserID) != ownerUserID {
			return fmt.Errorf("registry identity owner 无效: %q", ownerUserID)
		}
		if value.Status != "active" && value.Status != "disabled" {
			return fmt.Errorf("registry identity %q status 无效: %q", ownerUserID, value.Status)
		}
		if value.Username != stableAccountName("nxu", ownerUserID) ||
			value.GroupName != value.Username {
			return fmt.Errorf("registry identity %q OS account 名称不稳定", ownerUserID)
		}
		if value.UID != value.PrivateGID {
			return fmt.Errorf("registry identity %q 的 UID/private GID 不一致", ownerUserID)
		}
		if value.LayoutVersion < 0 || value.LayoutVersion > userLayoutVersion {
			return fmt.Errorf("registry identity %q layout version 无效", ownerUserID)
		}
		if err := registerID(value.UID, "identity:"+ownerUserID); err != nil {
			return err
		}
		if err := registerID(value.PrivateGID, "identity:"+ownerUserID); err != nil {
			return err
		}
		userRoot := appfs.UserDataRootAt(config.StateRoot, ownerUserID)
		if filepath.Clean(value.HomeDir) != filepath.Join(userRoot, "runtime", "home") ||
			filepath.Clean(value.TempDir) != filepath.Join(userRoot, "runtime", "tmp") {
			return fmt.Errorf("registry identity %q runtime 路径不匹配", ownerUserID)
		}
	}

	sharedRoot := filepath.Join(config.StateRoot, "shared-workspaces")
	projectRoots := make([]*project, 0, len(current.Projects))
	for projectID, value := range current.Projects {
		if value == nil {
			return fmt.Errorf("registry project %q 为空", projectID)
		}
		if projectID == "" || projectID != value.ProjectID ||
			value.GroupName != stableAccountName("nxp", projectID) {
			return fmt.Errorf("registry project %q 标识不稳定", projectID)
		}
		if value.Members == nil {
			return fmt.Errorf("registry project %q members 为空", projectID)
		}
		if err := registerID(value.GID, "project:"+projectID); err != nil {
			return err
		}
		root, err := canonicalExistingOrPendingPath(value.Root)
		if err != nil {
			return fmt.Errorf("解析 registry project %q root: %w", projectID, err)
		}
		if root != filepath.Clean(value.Root) ||
			filepath.Dir(root) != filepath.Clean(sharedRoot) {
			return fmt.Errorf("registry project %q root 不是 shared-workspaces 的直接子目录", projectID)
		}
		for ownerUserID, access := range value.Members {
			identityValue := current.Identities[ownerUserID]
			if identityValue == nil {
				return fmt.Errorf("project %q 引用了不存在的 owner %q", projectID, ownerUserID)
			}
			if access != projectAccessRead && access != projectAccessWrite {
				return fmt.Errorf("project %q 的 owner %q access 无效", projectID, ownerUserID)
			}
		}
		projectRoots = append(projectRoots, value)
	}
	for leftIndex := range projectRoots {
		for rightIndex := leftIndex + 1; rightIndex < len(projectRoots); rightIndex++ {
			left, right := projectRoots[leftIndex], projectRoots[rightIndex]
			if pathWithin(left.Root, right.Root) || pathWithin(right.Root, left.Root) {
				return fmt.Errorf("registry project 不能嵌套: %q 与 %q", left.ProjectID, right.ProjectID)
			}
		}
	}
	return nil
}

func registryRoot(config launcherConfig) string {
	return filepath.Join(config.StateRoot, "app", "data", "runtime-isolation")
}

func registryPath(config launcherConfig) string {
	return filepath.Join(registryRoot(config), "registry.json")
}

func ticketRoot(config launcherConfig) string {
	return filepath.Join(registryRoot(config), "tickets")
}

func writeLaunchTicket(config launcherConfig, ticket launchTicket) error {
	root := ticketRoot(config)
	if err := ensureRegistryLayout(config); err != nil {
		return err
	}
	data, err := json.Marshal(ticket)
	if err != nil {
		return err
	}
	path := filepath.Join(root, ticket.TicketID+".json")
	return writeExclusiveRootFile(path, data)
}

func readLaunchTicket(config launcherConfig, ticketID string) (launchTicket, error) {
	if !validOpaqueID(ticketID) {
		return launchTicket{}, errors.New("runtime isolation ticket 格式无效")
	}
	if err := ensureRegistryLayout(config); err != nil {
		return launchTicket{}, err
	}
	path := filepath.Join(ticketRoot(config), ticketID+".json")
	data, err := readRootOwnedFile(path)
	if err != nil {
		return launchTicket{}, err
	}
	var ticket launchTicket
	if err = json.Unmarshal(data, &ticket); err != nil {
		return launchTicket{}, err
	}
	if ticket.TicketID != ticketID {
		return launchTicket{}, errors.New("runtime isolation ticket 身份不一致")
	}
	if time.Now().UTC().After(ticket.ExpiresAt) {
		return launchTicket{}, errors.New("runtime isolation ticket 已过期")
	}
	return ticket, nil
}

func cleanupExpiredTickets(config launcherConfig, now time.Time) {
	entries, err := os.ReadDir(ticketRoot(config))
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		path := filepath.Join(ticketRoot(config), entry.Name())
		data, readErr := readRootOwnedFile(path)
		if readErr != nil {
			continue
		}
		var ticket launchTicket
		if json.Unmarshal(data, &ticket) == nil && now.After(ticket.ExpiresAt) {
			_ = os.Remove(path)
		}
	}
}

// readRootOwnedFile 把元数据校验和读取绑定在同一个 fd 上，避免在
// Lstat 与 ReadFile 之间被路径替换。registry 根本身另由 0700 root 目录保护。
func readRootOwnedFile(path string) ([]byte, error) {
	fd, err := openRegularNoSymlink(path, unix.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(fd), path)
	if file == nil {
		_ = unix.Close(fd)
		return nil, errors.New("打开 root-owned 文件失败")
	}
	defer file.Close()
	var stat unix.Stat_t
	if err = unix.Fstat(fd, &stat); err != nil {
		return nil, err
	}
	if stat.Uid != 0 || stat.Mode&0o077 != 0 {
		return nil, fmt.Errorf("root-owned 文件权限无效: %s", path)
	}
	return io.ReadAll(file)
}

func newOpaqueID() (string, error) {
	buffer := make([]byte, 24)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return hex.EncodeToString(buffer), nil
}

func validOpaqueID(value string) bool {
	if len(value) != 48 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func writeExclusiveRootFile(path string, data []byte) error {
	fd, err := openRegularNoSymlink(path, unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	file := os.NewFile(uintptr(fd), path)
	if file == nil {
		_ = unix.Close(fd)
		return errors.New("创建 root-owned 文件失败")
	}
	remove := true
	defer func() {
		_ = file.Close()
		if remove {
			_ = os.Remove(path)
		}
	}()
	if err = file.Chown(0, 0); err != nil {
		return err
	}
	if err = file.Chmod(0o600); err != nil {
		return err
	}
	if _, err = file.Write(data); err != nil {
		return err
	}
	if err = file.Sync(); err != nil {
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	remove = false
	return nil
}

// ensureRegistryLayout 在宿主可写的 app/data 下面建立 root-only 的 registry
// 命名空间。所有后续票据和锁文件都只能从这个无符号链接目录进入。
func ensureRegistryLayout(config launcherConfig) error {
	appDataRoot := filepath.Join(config.StateRoot, "app", "data")
	appDataFD, err := ensureDirectoryNoSymlink(appDataRoot, 0o770)
	if err != nil {
		return err
	}
	if err = unix.Fchown(appDataFD, 0, config.HostGID); err != nil {
		_ = unix.Close(appDataFD)
		return err
	}
	if err = unix.Fchmod(appDataFD, unix.S_ISGID|0o770); err != nil {
		_ = unix.Close(appDataFD)
		return err
	}
	_ = unix.Close(appDataFD)

	registryFD, err := ensureDirectoryNoSymlink(registryRoot(config), 0o700)
	if err != nil {
		return err
	}
	if err = secureRegistryDirectoryFD(registryFD); err != nil {
		_ = unix.Close(registryFD)
		return err
	}
	_ = unix.Close(registryFD)

	ticketsFD, err := ensureDirectoryNoSymlink(ticketRoot(config), 0o700)
	if err != nil {
		return err
	}
	if err = secureRegistryDirectoryFD(ticketsFD); err != nil {
		_ = unix.Close(ticketsFD)
		return err
	}
	_ = unix.Close(ticketsFD)

	for _, path := range []string{
		registryPath(config),
		filepath.Join(registryRoot(config), "registry.lock"),
	} {
		if err = normalizeRegistryFile(path); err != nil {
			return err
		}
	}
	return nil
}

func secureRegistryDirectoryFD(fd int) error {
	if err := unix.Fchown(fd, 0, 0); err != nil {
		return err
	}
	if err := clearPOSIXACLFD(fd, true); err != nil {
		return err
	}
	return unix.Fchmod(fd, 0o700)
}

func validateRegistryFile(path string, allowMissing bool) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) && allowMissing {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0o077 != 0 {
		return fmt.Errorf("registry 文件权限无效: %s", path)
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Uid != 0 {
		return fmt.Errorf("registry 文件必须由 root 持有: %s", path)
	}
	return nil
}

func normalizeRegistryFile(path string) error {
	if err := validateRegistryFile(path, true); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		// 旧版本 registry 可能仍是 root:host 0600/0640；只要它不是
		// 符号链接，就用已解析的 fd 收紧到 root-only。
		info, statErr := os.Lstat(path)
		if statErr != nil || info.Mode()&os.ModeSymlink != 0 ||
			!info.Mode().IsRegular() {
			return err
		}
	}
	fd, err := openRegularNoSymlink(path, unix.O_RDONLY, 0)
	if errors.Is(err, unix.ENOENT) {
		return nil
	}
	if err != nil {
		return err
	}
	defer unix.Close(fd)
	if err = unix.Fchown(fd, 0, 0); err != nil {
		return err
	}
	if err = unix.Fchmod(fd, 0o600); err != nil {
		return err
	}
	return validateRegistryFile(path, false)
}

func canonicalExistingOrPendingPath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" || !filepath.IsAbs(path) {
		return "", errors.New("路径必须是非空绝对路径")
	}
	path = filepath.Clean(path)
	current := path
	missing := make([]string, 0)
	for {
		resolved, err := filepath.EvalSymlinks(current)
		if err == nil {
			for index := len(missing) - 1; index >= 0; index-- {
				resolved = filepath.Join(resolved, missing[index])
			}
			return filepath.Clean(resolved), nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return "", err
		}
		parent := filepath.Dir(current)
		if parent == current {
			return path, nil
		}
		missing = append(missing, filepath.Base(current))
		current = parent
	}
}

func pathWithin(path string, root string) bool {
	path = filepath.Clean(path)
	root = filepath.Clean(root)
	if path == root {
		return true
	}
	relative, err := filepath.Rel(root, path)
	return err == nil && relative != ".." && relative != "." &&
		!strings.HasPrefix(relative, ".."+string(filepath.Separator)) &&
		!filepath.IsAbs(relative)
}

func compactPaths(paths []string) []string {
	normalized := make([]string, 0, len(paths))
	for _, path := range paths {
		if path = strings.TrimSpace(path); path != "" {
			normalized = append(normalized, filepath.Clean(path))
		}
	}
	slices.Sort(normalized)
	return slices.Compact(normalized)
}
