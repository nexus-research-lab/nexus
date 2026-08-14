//go:build linux

package runtimeidentity

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"golang.org/x/sys/unix"
)

const (
	aclXattrVersion = 0x0002
	aclUserObject   = 0x01
	aclNamedUser    = 0x02
	aclGroupObject  = 0x04
	aclNamedGroup   = 0x08
	aclMask         = 0x10
	aclOther        = 0x20
	aclUndefinedID  = ^uint32(0)
)

type aclEntry struct {
	tag  uint16
	perm uint16
	id   uint32
}

func ensureIdentityLayout(config launcherConfig, value *identity) (bool, error) {
	if value == nil {
		return false, errors.New("runtime identity 为空")
	}
	usersRoot := filepath.Join(config.StateRoot, "users")
	usersFD, err := ensureDirectoryNoSymlink(usersRoot, 0o771)
	if err != nil {
		return false, err
	}
	if err = unix.Fchown(usersFD, 0, config.HostGID); err != nil {
		_ = unix.Close(usersFD)
		return false, err
	}
	if err = clearPOSIXACLFD(usersFD, true); err != nil {
		_ = unix.Close(usersFD)
		return false, err
	}
	if err = unix.Fchmod(usersFD, unix.S_ISGID|0o771); err != nil {
		_ = unix.Close(usersFD)
		return false, err
	}
	_ = unix.Close(usersFD)
	userRoot := appfs.UserDataRootAt(config.StateRoot, value.OwnerUserID)
	userRootFD, err := ensureDirectoryNoSymlink(userRoot, 0o750)
	if err != nil {
		return false, err
	}
	if err = applyRootOwnedBoundaryACLFD(
		userRootFD,
		value.PrivateGID,
		config.HostUID,
		5,
	); err != nil {
		_ = unix.Close(userRootFD)
		return false, err
	}
	_ = unix.Close(userRootFD)
	workspaceRoot := filepath.Join(userRoot, "workspace")
	runtimeRoot := filepath.Join(userRoot, "runtime")
	workspaceRootFD, err := ensureDirectoryNoSymlink(workspaceRoot, 0o770)
	if err != nil {
		return false, err
	}
	if err = applyHostOwnedWorkspaceRootACLFD(
		workspaceRootFD,
		value.PrivateGID,
		config.HostUID,
	); err != nil {
		_ = unix.Close(workspaceRootFD)
		return false, err
	}
	_ = unix.Close(workspaceRootFD)

	runtimeRootFD, err := ensureDirectoryNoSymlink(runtimeRoot, 0o770)
	if err != nil {
		return false, err
	}
	if err = applyRootOwnedBoundaryACLFD(
		runtimeRootFD,
		value.PrivateGID,
		config.HostUID,
		7,
	); err != nil {
		_ = unix.Close(runtimeRootFD)
		return false, err
	}
	_ = unix.Close(runtimeRootFD)
	if err := ensureOwnerStateLayout(userRoot, value, config.HostUID); err != nil {
		return false, err
	}
	requiredDirectories := []string{
		filepath.Join(userRoot, "runtime", "projects"),
		filepath.Join(userRoot, "runtime", "home"),
		filepath.Join(userRoot, "runtime", "cache"),
		filepath.Join(userRoot, "runtime", "logs"),
		filepath.Join(userRoot, "runtime", "tmp"),
	}
	for _, directory := range requiredDirectories {
		directoryFD, directoryErr := ensureDirectoryNoSymlink(directory, 0o770)
		if directoryErr != nil {
			return false, directoryErr
		}
		if directoryErr = applyPrivateACLFD(
			directoryFD,
			os.ModeDir|0o770,
			value.UID,
			value.PrivateGID,
			config.HostUID,
		); directoryErr != nil {
			_ = unix.Close(directoryFD)
			return false, directoryErr
		}
		_ = unix.Close(directoryFD)
		if directoryErr = verifyDirectoryNoSymlink(directory); directoryErr != nil {
			return false, directoryErr
		}
	}
	if value.LayoutVersion >= userLayoutVersion {
		return false, nil
	}
	if err := filepath.WalkDir(userRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			if filepath.Dir(path) == workspaceRoot {
				return fmt.Errorf("owner workspace 顶层不能包含符号链接边界: %s", path)
			}
			// 符号链接本身不携带可用 ACL；不对其目标做 root 跟随操作。
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() && !info.IsDir() {
			return nil
		}
		if path == userRoot || path == workspaceRoot || path == runtimeRoot {
			return nil
		}
		stateRoot := filepath.Join(userRoot, "state")
		if path == stateRoot {
			return nil
		}
		if pathWithin(path, stateRoot) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if info.IsDir() && filepath.Dir(path) == workspaceRoot {
			fd, openErr := openPathNoSymlink(path, true)
			if openErr != nil {
				return openErr
			}
			defer unix.Close(fd)
			return applyRootOwnedBoundaryACLFD(
				fd,
				value.PrivateGID,
				config.HostUID,
				7,
			)
		}
		return applyPrivateACL(
			path,
			info.Mode(),
			value.UID,
			value.PrivateGID,
			config.HostUID,
		)
	}); err != nil {
		return false, err
	}
	value.LayoutVersion = userLayoutVersion
	return true, nil
}

// repairRuntimeACL 修复 runtime 进程按 0600 创建文件后被压低的 ACL mask。
//
// runtime 以 owner UID 运行，宿主通过 named-user ACL 读取 transcript、debug
// cache 与任务状态。Linux 在 open(..., 0600) 时会把继承 ACL 的 mask 一并
// 限制为 ---，所以仅配置目录 default ACL 不足以覆盖运行期新文件。该函数
// 只遍历当前 owner 的 runtime 根，跳过符号链接和特殊文件，不触碰 workspace
// 与宿主 state。
func repairRuntimeACL(config launcherConfig, value *identity) error {
	if value == nil {
		return errors.New("runtime identity 为空")
	}
	runtimeRoot := filepath.Join(
		appfs.UserDataRootAt(config.StateRoot, value.OwnerUserID),
		"runtime",
	)
	info, err := os.Lstat(runtimeRoot)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return errors.New("owner runtime 根不是安全目录")
	}
	return filepath.WalkDir(runtimeRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			if errors.Is(walkErr, os.ErrNotExist) {
				return nil
			}
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		info, err := entry.Info()
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		if !info.IsDir() && !info.Mode().IsRegular() {
			return nil
		}
		if path == runtimeRoot {
			return nil
		}
		if err := applyPrivateACL(
			path,
			info.Mode(),
			value.UID,
			value.PrivateGID,
			config.HostUID,
		); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("修复 runtime ACL %s: %w", path, err)
		}
		return nil
	})
}

// prepareRuntimeArgFiles 让 bridge 以宿主身份生成的参数只对当前 runtime 私有组读写。
// 宿主进程不属于动态私有组，不能依赖 setgid 继承；launcher 必须在降权前收口归属。
func prepareRuntimeArgFiles(
	config launcherConfig,
	policy preparedPolicy,
	args []string,
) error {
	root := filepath.Join(
		runtimeRootForPolicy(config, policy.OwnerUserID),
		"runtime",
		"arg-files",
	)
	for _, argument := range args {
		path := filepath.Clean(argument)
		if !filepath.IsAbs(argument) || filepath.Dir(path) != root {
			continue
		}
		if err := prepareRuntimeArgFile(
			path,
			config.HostUID,
			policy.Identity.PrivateGID,
		); err != nil {
			return fmt.Errorf("准备 runtime 参数文件 %s: %w", path, err)
		}
	}
	return nil
}

func prepareRuntimeArgFile(path string, hostUID int, privateGID int) error {
	fd, err := openPathNoSymlink(path, false)
	if err != nil {
		return err
	}
	defer unix.Close(fd)
	var stat unix.Stat_t
	if err = unix.Fstat(fd, &stat); err != nil {
		return err
	}
	if stat.Nlink != 1 {
		return errors.New("runtime 参数文件不能是硬链接")
	}
	if err = unix.Fchown(fd, hostUID, privateGID); err != nil {
		return err
	}
	if err = clearPOSIXACLFD(fd, false); err != nil {
		return err
	}
	return unix.Fchmod(fd, 0o660)
}

// ensureOwnerStateLayout 让 owner state 与 workspace/runtime 使用同一私有组。
func ensureOwnerStateLayout(userRoot string, value *identity, hostUID int) error {
	if value == nil {
		return errors.New("runtime identity 为空")
	}
	stateRoot := filepath.Join(userRoot, "state")
	stateFD, err := ensureDirectoryNoSymlink(stateRoot, 0o770)
	if err != nil {
		return err
	}
	_ = unix.Close(stateFD)

	roomsRoot := filepath.Join(stateRoot, "rooms")
	roomsFD, err := ensureDirectoryNoSymlink(roomsRoot, 0o770)
	if err != nil {
		return err
	}
	_ = unix.Close(roomsFD)
	return normalizeOwnerStateTree(stateRoot, value, hostUID)
}

func normalizeOwnerStateTree(root string, value *identity, hostUID int) error {
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("owner 状态不能包含符号链接: %s", path)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.IsDir() && !info.Mode().IsRegular() {
			return fmt.Errorf("owner 状态不能包含特殊文件: %s", path)
		}
		if info.Mode().IsRegular() {
			if stat, ok := info.Sys().(*unix.Stat_t); ok && stat.Nlink > 1 {
				return fmt.Errorf("owner 状态不能包含硬链接文件: %s", path)
			}
		}
		return applyPrivateACL(path, info.Mode(), value.UID, value.PrivateGID, hostUID)
	})
}

func ensureHostLayout(config launcherConfig) error {
	stateRootFD, err := ensureDirectoryNoSymlink(config.StateRoot, 0o771)
	if err != nil {
		return err
	}
	if err = unix.Fchown(stateRootFD, 0, config.HostGID); err != nil {
		_ = unix.Close(stateRootFD)
		return err
	}
	if err = clearPOSIXACLFD(stateRootFD, true); err != nil {
		_ = unix.Close(stateRootFD)
		return err
	}
	if err = unix.Fchmod(stateRootFD, unix.S_ISGID|0o771); err != nil {
		_ = unix.Close(stateRootFD)
		return err
	}
	_ = unix.Close(stateRootFD)

	appRoot := filepath.Join(config.StateRoot, "app")
	appRootFD, err := ensureDirectoryNoSymlink(appRoot, 0o771)
	if err != nil {
		return err
	}
	if err = unix.Fchown(appRootFD, 0, config.HostGID); err != nil {
		_ = unix.Close(appRootFD)
		return err
	}
	if err = clearPOSIXACLFD(appRootFD, true); err != nil {
		_ = unix.Close(appRootFD)
		return err
	}
	if err = unix.Fchmod(appRootFD, unix.S_ISGID|0o771); err != nil {
		_ = unix.Close(appRootFD)
		return err
	}
	_ = unix.Close(appRootFD)
	if err := lockLegacyRoomRoot(appRoot, config.HostGID); err != nil {
		return err
	}
	for _, name := range []string{"cache", "config", "data", "logs"} {
		root := filepath.Join(appRoot, name)
		rootFD, err := ensureDirectoryNoSymlink(root, 0o770)
		if err != nil {
			return err
		}
		if err = unix.Fchown(rootFD, 0, config.HostGID); err != nil {
			_ = unix.Close(rootFD)
			return err
		}
		if err = clearPOSIXACLFD(rootFD, true); err != nil {
			_ = unix.Close(rootFD)
			return err
		}
		if err = unix.Fchmod(rootFD, unix.S_ISGID|0o770); err != nil {
			_ = unix.Close(rootFD)
			return err
		}
		_ = unix.Close(rootFD)
		if err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if path == registryRoot(config) {
				// launcher registry/ticket 使用更窄的 root-owned 权限。
				if entry.Type()&os.ModeSymlink != 0 || !entry.IsDir() {
					return errors.New("runtime isolation registry root 不是安全目录")
				}
				return filepath.SkipDir
			}
			if entry.Type()&os.ModeSymlink != 0 {
				return nil
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			if !info.Mode().IsRegular() && !info.IsDir() {
				return nil
			}
			if err = applyHostPath(path, info.Mode(), config.HostGID); err != nil {
				return err
			}
			return nil
		}); err != nil {
			return err
		}
	}
	for _, root := range config.ReadOnlyRoots {
		if err := ensureRuntimeReadOnlyAncestors(config, root); err != nil {
			return err
		}
		if err := applyRuntimeReadOnlyTree(config, root); err != nil {
			return err
		}
	}
	return nil
}

// lockLegacyRoomRoot 只收紧旧迁移源的根目录，不遍历内容。
//
// 目录内的 symlink、硬链接和特殊文件由 Room 迁移逐项隔离；先撤销根目录的
// runtime ACL，可避免旧 runtime 在 owner 解析与复制期间继续换入文件。
func lockLegacyRoomRoot(appRoot string, hostGID int) error {
	legacyRoot := filepath.Join(appRoot, "rooms")
	info, err := os.Lstat(legacyRoot)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		// 交给非阻断迁移移入隔离区；权限同步不跟随不可信路径。
		return nil
	}
	return applyHostPath(legacyRoot, info.Mode(), hostGID)
}

// ensureRuntimeReadOnlyAncestors 只为 state_root/app 下的显式只读根开放
// 穿越位。根本身仍由 applyRuntimeReadOnlyTree 决定读写权限，宿主敏感目录
// 不会因为 runtime 的资源入口而意外暴露。
func ensureRuntimeReadOnlyAncestors(config launcherConfig, root string) error {
	appRoot := filepath.Join(config.StateRoot, "app")
	root = filepath.Clean(root)
	if !pathWithin(root, appRoot) || root == appRoot {
		return nil
	}
	relative, err := filepath.Rel(appRoot, root)
	if err != nil || relative == "." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return nil
	}
	current := appRoot
	components := strings.Split(relative, string(filepath.Separator))
	for index := 0; index < len(components)-1; index++ {
		component := components[index]
		if component == "" || component == "." || component == ".." {
			continue
		}
		current = filepath.Join(current, component)
		fd, openErr := ensureDirectoryNoSymlink(current, 0o770)
		if openErr != nil {
			return openErr
		}
		if openErr = unix.Fchown(fd, 0, config.HostGID); openErr != nil {
			_ = unix.Close(fd)
			return openErr
		}
		if openErr = clearPOSIXACLFD(fd, true); openErr != nil {
			_ = unix.Close(fd)
			return openErr
		}
		if openErr = unix.Fchmod(fd, unix.S_ISGID|0o771); openErr != nil {
			_ = unix.Close(fd)
			return openErr
		}
		if openErr = unix.Close(fd); openErr != nil {
			return openErr
		}
	}
	return nil
}

func applyPrivateACL(
	path string,
	mode os.FileMode,
	uid int,
	gid int,
	hostUID int,
) error {
	fd, err := openPathNoSymlink(path, mode.IsDir())
	if err != nil {
		return err
	}
	defer unix.Close(fd)
	return applyPrivateACLFD(fd, mode, uid, gid, hostUID)
}

func ensureWorkspaceBoundaryACL(
	path string,
	value *identity,
	hostUID int,
) error {
	if value == nil {
		return errors.New("runtime identity 为空")
	}
	fd, err := openPathNoSymlink(path, true)
	if err != nil {
		return err
	}
	defer unix.Close(fd)
	return applyRootOwnedBoundaryACLFD(
		fd,
		value.PrivateGID,
		hostUID,
		7,
	)
}

func applyPrivateACLFD(
	fd int,
	mode os.FileMode,
	uid int,
	gid int,
	hostUID int,
) error {
	isDirectory := mode.IsDir()
	ownerPerm := permissionBits(mode, 6)
	if isDirectory {
		ownerPerm = 7
	} else {
		ownerPerm |= 6
	}
	groupPerm := ownerPerm
	hostPerm := ownerPerm
	targetMode := os.FileMode(ownerPerm<<6 | groupPerm<<3)
	if isDirectory {
		targetMode |= os.ModeSetgid
	}
	if err := unix.Fchown(fd, uid, gid); err != nil {
		return err
	}
	if err := unix.Fchmod(fd, uint32(targetMode.Perm())|setgidMode(targetMode)); err != nil {
		return err
	}
	entries := []aclEntry{
		{tag: aclUserObject, perm: ownerPerm, id: aclUndefinedID},
		{tag: aclNamedUser, perm: hostPerm, id: uint32(hostUID)},
		{tag: aclGroupObject, perm: groupPerm, id: aclUndefinedID},
		{tag: aclMask, perm: hostPerm | groupPerm, id: aclUndefinedID},
		{tag: aclOther, perm: 0, id: aclUndefinedID},
	}
	if err := setACLXattrFD(fd, "system.posix_acl_access", entries); err != nil {
		return err
	}
	if isDirectory {
		defaultEntries := []aclEntry{
			{tag: aclUserObject, perm: 7, id: aclUndefinedID},
			{tag: aclNamedUser, perm: 7, id: uint32(hostUID)},
			{tag: aclGroupObject, perm: 7, id: aclUndefinedID},
			{tag: aclMask, perm: 7, id: aclUndefinedID},
			{tag: aclOther, perm: 0, id: aclUndefinedID},
		}
		if err := setACLXattrFD(fd, "system.posix_acl_default", defaultEntries); err != nil {
			return err
		}
	}
	return nil
}

// applyRootOwnedBoundaryACLFD 让根目录结构保持由 launcher 控制，同时通过
// private group 决定 runtime 只读穿越或正常创建内容。
func applyRootOwnedBoundaryACLFD(
	fd int,
	gid int,
	hostUID int,
	groupPerm uint16,
) error {
	if err := unix.Fchown(fd, 0, gid); err != nil {
		return err
	}
	if err := unix.Fchmod(fd, unix.S_ISGID|0o770); err != nil {
		return err
	}
	entries := []aclEntry{
		{tag: aclUserObject, perm: 7, id: aclUndefinedID},
		{tag: aclNamedUser, perm: 7, id: uint32(hostUID)},
		{tag: aclGroupObject, perm: groupPerm, id: aclUndefinedID},
		{tag: aclMask, perm: 7, id: aclUndefinedID},
		{tag: aclOther, perm: 0, id: aclUndefinedID},
	}
	if err := setACLXattrFD(fd, "system.posix_acl_access", entries); err != nil {
		return err
	}
	return setACLXattrFD(fd, "system.posix_acl_default", entries)
}

// applyHostOwnedWorkspaceRootACLFD 让宿主成为 sticky 目录 owner，既能删除
// root-owned Agent 边界，又阻止 runtime UID 替换不属于自己的顶层条目。
func applyHostOwnedWorkspaceRootACLFD(fd int, gid int, hostUID int) error {
	if err := unix.Fchown(fd, hostUID, gid); err != nil {
		return err
	}
	if err := unix.Fchmod(fd, unix.S_ISGID|unix.S_ISVTX|0o770); err != nil {
		return err
	}
	accessEntries := []aclEntry{
		{tag: aclUserObject, perm: 7, id: aclUndefinedID},
		{tag: aclGroupObject, perm: 7, id: aclUndefinedID},
		{tag: aclMask, perm: 7, id: aclUndefinedID},
		{tag: aclOther, perm: 0, id: aclUndefinedID},
	}
	if err := setACLXattrFD(fd, "system.posix_acl_access", accessEntries); err != nil {
		return err
	}
	defaultEntries := []aclEntry{
		{tag: aclUserObject, perm: 7, id: aclUndefinedID},
		{tag: aclNamedUser, perm: 7, id: uint32(hostUID)},
		{tag: aclGroupObject, perm: 7, id: aclUndefinedID},
		{tag: aclMask, perm: 7, id: aclUndefinedID},
		{tag: aclOther, perm: 0, id: aclUndefinedID},
	}
	if err := setACLXattrFD(fd, "system.posix_acl_default", defaultEntries); err != nil {
		return err
	}
	return unix.Fchmod(fd, unix.S_ISGID|unix.S_ISVTX|0o770)
}

func applyProjectTreeACL(
	config launcherConfig,
	value *project,
	current *registry,
) error {
	if value == nil {
		return errors.New("project 为空")
	}
	memberUIDs := map[int]string{}
	for ownerUserID, access := range value.Members {
		identityValue := current.Identities[ownerUserID]
		if identityValue == nil || identityValue.Status != "active" {
			continue
		}
		memberUIDs[identityValue.UID] = access
	}
	return filepath.WalkDir(value.Root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() && !info.IsDir() {
			return nil
		}
		return applyProjectACL(path, info.Mode(), value.GID, config.HostUID, memberUIDs)
	})
}

func applyProjectACL(
	path string,
	mode os.FileMode,
	gid int,
	hostUID int,
	members map[int]string,
) error {
	fd, err := openPathNoSymlink(path, mode.IsDir())
	if err != nil {
		return err
	}
	defer unix.Close(fd)
	return applyProjectACLFD(fd, mode, gid, hostUID, members)
}

func applyProjectACLFD(
	fd int,
	mode os.FileMode,
	gid int,
	hostUID int,
	members map[int]string,
) error {
	isDirectory := mode.IsDir()
	ownerPerm := permissionBits(mode, 6) | 6
	groupPerm := ownerPerm
	hostPerm := ownerPerm
	if isDirectory {
		ownerPerm, groupPerm, hostPerm = 7, 7, 7
	}
	targetMode := os.FileMode(ownerPerm<<6 | groupPerm<<3)
	if isDirectory {
		targetMode |= os.ModeSetgid
	}
	if err := unix.Fchown(fd, 0, gid); err != nil {
		return err
	}
	if err := unix.Fchmod(fd, uint32(targetMode.Perm())|setgidMode(targetMode)); err != nil {
		return err
	}
	entries := []aclEntry{
		{tag: aclUserObject, perm: ownerPerm, id: aclUndefinedID},
	}
	namedPermissions := map[int]uint16{hostUID: hostPerm}
	for uid := range members {
		permission := uint16(4) | ownerPerm&1
		if members[uid] == projectAccessWrite {
			permission = groupPerm
		}
		namedPermissions[uid] = permission
	}
	namedIDs := make([]int, 0, len(namedPermissions))
	for uid := range namedPermissions {
		namedIDs = append(namedIDs, uid)
	}
	slicesSortInts(namedIDs)
	for _, uid := range namedIDs {
		entries = append(entries, aclEntry{
			tag:  aclNamedUser,
			perm: namedPermissions[uid],
			id:   uint32(uid),
		})
	}
	maskPerm := groupPerm
	for _, permission := range namedPermissions {
		maskPerm |= permission
	}
	entries = append(entries,
		aclEntry{tag: aclGroupObject, perm: groupPerm, id: aclUndefinedID},
		aclEntry{tag: aclMask, perm: maskPerm, id: aclUndefinedID},
		aclEntry{tag: aclOther, perm: 0, id: aclUndefinedID},
	)
	if err := setACLXattrFD(fd, "system.posix_acl_access", entries); err != nil {
		return err
	}
	if isDirectory {
		defaultEntries := make([]aclEntry, 0, len(entries))
		for _, entry := range entries {
			defaultEntries = append(defaultEntries, entry)
		}
		if err := setACLXattrFD(fd, "system.posix_acl_default", defaultEntries); err != nil {
			return err
		}
	}
	return nil
}

func applyHostPath(path string, mode os.FileMode, gid int) error {
	fd, err := openPathNoSymlink(path, mode.IsDir())
	if err != nil {
		return err
	}
	defer unix.Close(fd)
	if err = unix.Fchown(fd, 0, gid); err != nil {
		return err
	}
	if err = clearPOSIXACLFD(fd, mode.IsDir()); err != nil {
		return err
	}
	if mode.IsDir() {
		return unix.Fchmod(fd, unix.S_ISGID|0o770)
	}
	ownerPerm := permissionBits(mode, 6) | 6
	return unix.Fchmod(fd, uint32(ownerPerm<<6|ownerPerm<<3))
}

func applyRuntimeReadOnlyTree(config launcherConfig, root string) error {
	if _, err := os.Lstat(root); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}
	appRoot := filepath.Join(config.StateRoot, "app")
	if !pathWithin(root, appRoot) {
		return validateExternalReadOnlyTree(config, root)
	}
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() && !info.IsDir() {
			return nil
		}
		fd, err := openPathNoSymlink(path, info.IsDir())
		if err != nil {
			return err
		}
		var stat unix.Stat_t
		if err = unix.Fstat(fd, &stat); err != nil {
			_ = unix.Close(fd)
			return err
		}
		if info.Mode().IsRegular() && stat.Nlink > 1 {
			_ = unix.Close(fd)
			return fmt.Errorf("宿主只读资源不能包含硬链接文件: %s", path)
		}
		if err = unix.Fchown(fd, 0, config.HostGID); err != nil {
			_ = unix.Close(fd)
			return err
		}
		if err = clearPOSIXACLFD(fd, info.IsDir()); err != nil {
			_ = unix.Close(fd)
			return err
		}
		var permissionErr error
		switch {
		case info.IsDir():
			permissionErr = unix.Fchmod(fd, unix.S_ISGID|0o775)
		default:
			permission := uint32(0o644)
			if info.Mode().Perm()&0o100 != 0 {
				permission = 0o755
			}
			permission |= 0o020
			permissionErr = unix.Fchmod(fd, permission)
		}
		closeErr := unix.Close(fd)
		if permissionErr != nil {
			return permissionErr
		}
		return closeErr
	})
}

// validateExternalReadOnlyTree 不重写 /opt 等应用包目录。部署负责准备
// root/host-owned 的只读内容，launcher 只验证 runtime 无法借 owner 或 mode
// 修改它，避免错误配置递归 chmod 系统文件。
func validateExternalReadOnlyTree(config launcherConfig, root string) error {
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() && !info.IsDir() {
			return fmt.Errorf("外部只读根包含不受支持的文件类型: %s", path)
		}
		fd, err := openPathNoSymlink(path, info.IsDir())
		if err != nil {
			return err
		}
		var stat unix.Stat_t
		if err = unix.Fstat(fd, &stat); err != nil {
			_ = unix.Close(fd)
			return err
		}
		closeErr := unix.Close(fd)
		if stat.Uid != 0 && int(stat.Uid) != config.HostUID {
			return fmt.Errorf("外部只读根只能由 root 或 host uid 持有: %s", path)
		}
		if stat.Mode&0o022 != 0 {
			return fmt.Errorf("外部只读根不能被 group/other 写入: %s", path)
		}
		if info.IsDir() && stat.Mode&0o005 != 0o005 {
			return fmt.Errorf("外部只读目录必须允许 runtime 读取和穿越: %s", path)
		}
		if info.Mode().IsRegular() && stat.Mode&0o004 == 0 {
			return fmt.Errorf("外部只读文件必须允许 runtime 读取: %s", path)
		}
		return closeErr
	})
}

func setACLXattrFD(fd int, name string, entries []aclEntry) error {
	buffer := make([]byte, 4+len(entries)*8)
	binary.LittleEndian.PutUint32(buffer[:4], aclXattrVersion)
	for index, entry := range entries {
		offset := 4 + index*8
		binary.LittleEndian.PutUint16(buffer[offset:offset+2], entry.tag)
		binary.LittleEndian.PutUint16(buffer[offset+2:offset+4], entry.perm&7)
		binary.LittleEndian.PutUint32(buffer[offset+4:offset+8], entry.id)
	}
	return unix.Fsetxattr(fd, name, buffer, 0)
}

func clearPOSIXACLFD(fd int, directory bool) error {
	names := []string{"system.posix_acl_access"}
	if directory {
		names = append(names, "system.posix_acl_default")
	}
	for _, name := range names {
		if err := unix.Fremovexattr(fd, name); err != nil &&
			!errors.Is(err, unix.ENODATA) &&
			!errors.Is(err, unix.ENOTSUP) {
			return err
		}
	}
	return nil
}

func setgidMode(mode os.FileMode) uint32 {
	if mode&os.ModeSetgid != 0 {
		return unix.S_ISGID
	}
	return 0
}

func ensureDirectoryNoSymlink(path string, createMode uint32) (int, error) {
	path = filepath.Clean(path)
	if !filepath.IsAbs(path) {
		return -1, errors.New("目录路径必须是绝对路径")
	}
	fd, err := unix.Open("/", unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return -1, err
	}
	components := strings.Split(strings.TrimPrefix(path, string(filepath.Separator)), string(filepath.Separator))
	for _, component := range components {
		if component == "" || component == "." {
			continue
		}
		if component == ".." {
			_ = unix.Close(fd)
			return -1, errors.New("目录路径包含 ..")
		}
		child, openErr := openAtDirectoryNoSymlink(fd, component)
		if errors.Is(openErr, unix.ENOENT) {
			if mkdirErr := unix.Mkdirat(fd, component, createMode&0o777); mkdirErr != nil &&
				!errors.Is(mkdirErr, unix.EEXIST) {
				_ = unix.Close(fd)
				return -1, mkdirErr
			}
			child, openErr = openAtDirectoryNoSymlink(fd, component)
		}
		if openErr != nil {
			_ = unix.Close(fd)
			return -1, openErr
		}
		_ = unix.Close(fd)
		fd = child
	}
	return fd, nil
}

func verifyDirectoryNoSymlink(path string) error {
	fd, err := openPathNoSymlink(path, true)
	if err != nil {
		return err
	}
	return unix.Close(fd)
}

func openAtDirectoryNoSymlink(parentFD int, name string) (int, error) {
	how := &unix.OpenHow{
		Flags:   uint64(unix.O_RDONLY | unix.O_DIRECTORY | unix.O_CLOEXEC | unix.O_NOFOLLOW),
		Resolve: unix.RESOLVE_NO_SYMLINKS,
	}
	fd, err := unix.Openat2(parentFD, name, how)
	if errors.Is(err, unix.ENOSYS) || errors.Is(err, unix.EINVAL) {
		return -1, fmt.Errorf("内核不支持无符号链接目录解析: %w", err)
	}
	return fd, err
}

func openPathNoSymlink(path string, directory bool) (int, error) {
	path = filepath.Clean(path)
	if !filepath.IsAbs(path) {
		return -1, errors.New("路径必须是绝对路径")
	}
	flags := unix.O_RDONLY | unix.O_CLOEXEC | unix.O_NOFOLLOW | unix.O_NONBLOCK
	if directory {
		flags |= unix.O_DIRECTORY
	}
	how := &unix.OpenHow{
		Flags:   uint64(flags),
		Resolve: unix.RESOLVE_NO_SYMLINKS,
	}
	fd, err := unix.Openat2(unix.AT_FDCWD, path, how)
	if errors.Is(err, unix.ENOSYS) || errors.Is(err, unix.EINVAL) {
		return -1, fmt.Errorf("内核不支持无符号链接路径解析: %w", err)
	}
	if err != nil {
		return -1, err
	}
	var stat unix.Stat_t
	if err = unix.Fstat(fd, &stat); err != nil {
		_ = unix.Close(fd)
		return -1, err
	}
	if directory && stat.Mode&unix.S_IFMT != unix.S_IFDIR {
		_ = unix.Close(fd)
		return -1, errors.New("目标不是目录")
	}
	if !directory && stat.Mode&unix.S_IFMT != unix.S_IFREG {
		_ = unix.Close(fd)
		return -1, errors.New("目标不是普通文件")
	}
	return fd, nil
}

func openRegularNoSymlink(path string, flags int, mode uint32) (int, error) {
	path = filepath.Clean(path)
	if !filepath.IsAbs(path) {
		return -1, errors.New("文件路径必须是绝对路径")
	}
	parentFD, err := openPathNoSymlink(filepath.Dir(path), true)
	if err != nil {
		return -1, err
	}
	defer unix.Close(parentFD)
	how := &unix.OpenHow{
		Flags:   uint64(flags | unix.O_CLOEXEC | unix.O_NOFOLLOW),
		Mode:    uint64(mode),
		Resolve: unix.RESOLVE_NO_SYMLINKS,
	}
	fd, err := unix.Openat2(parentFD, filepath.Base(path), how)
	if errors.Is(err, unix.ENOSYS) || errors.Is(err, unix.EINVAL) {
		return -1, fmt.Errorf("内核不支持无符号链接文件解析: %w", err)
	}
	if err != nil {
		return -1, err
	}
	var stat unix.Stat_t
	if err = unix.Fstat(fd, &stat); err != nil {
		_ = unix.Close(fd)
		return -1, err
	}
	if stat.Mode&unix.S_IFMT != unix.S_IFREG {
		_ = unix.Close(fd)
		return -1, errors.New("目标不是普通文件")
	}
	return fd, nil
}

func permissionBits(mode os.FileMode, shift uint) uint16 {
	return uint16((mode.Perm() >> shift) & 7)
}

func slicesSortInts(values []int) {
	for index := 1; index < len(values); index++ {
		for cursor := index; cursor > 0 && values[cursor] < values[cursor-1]; cursor-- {
			values[cursor], values[cursor-1] = values[cursor-1], values[cursor]
		}
	}
}
