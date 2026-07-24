//go:build linux

package runtimeidentity

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/unix"
)

const (
	landlockRulePathBeneath = 1
)

func applyLandlock(
	config launcherConfig,
	policy preparedPolicy,
	executable string,
) (int, error) {
	if !config.LandlockRequired {
		return 0, errors.New("enforce launcher 配置必须启用 Landlock")
	}
	abi, err := landlockABI()
	if err != nil {
		return 0, fmt.Errorf("查询 Landlock ABI: %w", err)
	}
	if config.LandlockRequired && abi < 3 {
		return 0, fmt.Errorf("Landlock ABI %d 不足，enforce 至少需要 ABI 3", abi)
	}
	handled := landlockHandledAccess(abi)
	attribute := unix.LandlockRulesetAttr{Access_fs: handled}
	rulesetFD, err := landlockCreateRuleset(&attribute, unsafe.Sizeof(attribute), 0)
	if err != nil {
		return 0, fmt.Errorf("创建 Landlock ruleset: %w", err)
	}
	defer unix.Close(rulesetFD)

	readRoots := append(defaultSystemReadRoots(), config.ReadOnlyRoots...)
	readRoots = append(readRoots, policy.RuntimeReadRoots...)
	readRoots = append(readRoots, executable)
	for _, root := range compactPaths(readRoots) {
		canonical, canonicalErr := canonicalExistingOrPendingPath(root)
		if canonicalErr != nil {
			if errors.Is(canonicalErr, os.ErrNotExist) {
				continue
			}
			return 0, canonicalErr
		}
		if err = addLandlockPath(rulesetFD, canonical, landlockReadAccess(handled), false); err != nil {
			return 0, err
		}
	}
	for _, root := range compactPaths(policy.RuntimeWriteRoots) {
		canonical, canonicalErr := canonicalExistingOrPendingPath(root)
		if canonicalErr != nil {
			return 0, canonicalErr
		}
		if err = addLandlockPath(rulesetFD, canonical, landlockWriteAccess(handled), true); err != nil {
			return 0, err
		}
	}
	for _, path := range []string{"/dev/null", "/dev/tty"} {
		if err = addLandlockPath(rulesetFD, path, landlockDeviceWriteAccess(handled), false); err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				return 0, err
			}
		}
	}
	if err = addLandlockPath(rulesetFD, "/dev/shm", landlockWriteAccess(handled), false); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return 0, err
		}
	}
	if err = unix.Prctl(unix.PR_SET_NO_NEW_PRIVS, 1, 0, 0, 0); err != nil {
		return 0, fmt.Errorf("设置 no_new_privs: %w", err)
	}
	if err = landlockRestrictSelf(rulesetFD); err != nil {
		return 0, fmt.Errorf("应用 Landlock ruleset: %w", err)
	}
	return abi, nil
}

func landlockABI() (int, error) {
	fd, _, errno := unix.Syscall(
		unix.SYS_LANDLOCK_CREATE_RULESET,
		0,
		0,
		uintptr(unix.LANDLOCK_CREATE_RULESET_VERSION),
	)
	if errno != 0 {
		return 0, errno
	}
	return int(fd), nil
}

func landlockCreateRuleset(
	attribute *unix.LandlockRulesetAttr,
	size uintptr,
	flags uint32,
) (int, error) {
	fd, _, errno := unix.Syscall(
		unix.SYS_LANDLOCK_CREATE_RULESET,
		uintptr(unsafe.Pointer(attribute)),
		size,
		uintptr(flags),
	)
	if errno != 0 {
		return 0, errno
	}
	return int(fd), nil
}

func landlockAddRule(
	rulesetFD int,
	attribute *unix.LandlockPathBeneathAttr,
) error {
	_, _, errno := unix.Syscall6(
		unix.SYS_LANDLOCK_ADD_RULE,
		uintptr(rulesetFD),
		uintptr(landlockRulePathBeneath),
		uintptr(unsafe.Pointer(attribute)),
		0,
		0,
		0,
	)
	if errno != 0 {
		return errno
	}
	return nil
}

func landlockRestrictSelf(rulesetFD int) error {
	_, _, errno := unix.Syscall(
		unix.SYS_LANDLOCK_RESTRICT_SELF,
		uintptr(rulesetFD),
		0,
		0,
	)
	if errno != 0 {
		return errno
	}
	return nil
}

func addLandlockPath(
	rulesetFD int,
	path string,
	allowed uint64,
	required bool,
) error {
	path = filepath.Clean(path)
	how := &unix.OpenHow{
		Flags:   uint64(unix.O_PATH | unix.O_CLOEXEC | unix.O_NOFOLLOW),
		Resolve: unix.RESOLVE_NO_SYMLINKS,
	}
	fd, err := unix.Openat2(unix.AT_FDCWD, path, how)
	if errors.Is(err, unix.ENOENT) && !required {
		return nil
	}
	if err != nil {
		return fmt.Errorf("打开 Landlock path %q: %w", path, err)
	}
	defer unix.Close(fd)
	var stat unix.Stat_t
	if err = unix.Fstat(fd, &stat); err != nil {
		return fmt.Errorf("读取 Landlock path %q: %w", path, err)
	}
	if stat.Mode&unix.S_IFMT != unix.S_IFDIR {
		allowed &= unix.LANDLOCK_ACCESS_FS_EXECUTE |
			unix.LANDLOCK_ACCESS_FS_READ_FILE |
			unix.LANDLOCK_ACCESS_FS_WRITE_FILE |
			unix.LANDLOCK_ACCESS_FS_TRUNCATE
	}
	if allowed == 0 {
		return nil
	}
	attribute := unix.LandlockPathBeneathAttr{
		Allowed_access: allowed,
		Parent_fd:      int32(fd),
	}
	if err = landlockAddRule(rulesetFD, &attribute); err != nil {
		return fmt.Errorf("添加 Landlock path %q: %w", path, err)
	}
	return nil
}

func landlockHandledAccess(abi int) uint64 {
	access := uint64(
		unix.LANDLOCK_ACCESS_FS_EXECUTE |
			unix.LANDLOCK_ACCESS_FS_WRITE_FILE |
			unix.LANDLOCK_ACCESS_FS_READ_FILE |
			unix.LANDLOCK_ACCESS_FS_READ_DIR |
			unix.LANDLOCK_ACCESS_FS_REMOVE_DIR |
			unix.LANDLOCK_ACCESS_FS_REMOVE_FILE |
			unix.LANDLOCK_ACCESS_FS_MAKE_CHAR |
			unix.LANDLOCK_ACCESS_FS_MAKE_DIR |
			unix.LANDLOCK_ACCESS_FS_MAKE_REG |
			unix.LANDLOCK_ACCESS_FS_MAKE_SOCK |
			unix.LANDLOCK_ACCESS_FS_MAKE_FIFO |
			unix.LANDLOCK_ACCESS_FS_MAKE_BLOCK |
			unix.LANDLOCK_ACCESS_FS_MAKE_SYM,
	)
	if abi >= 2 {
		access |= unix.LANDLOCK_ACCESS_FS_REFER
	}
	if abi >= 3 {
		access |= unix.LANDLOCK_ACCESS_FS_TRUNCATE
	}
	// ABI 5 的 IOCTL_DEV 会影响 tty、GPU 和图像工具；当前边界只声明
	// 文件路径访问，设备 ioctl 继续交给部署级 seccomp/cgroup。
	return access
}

func landlockReadAccess(handled uint64) uint64 {
	return handled & (unix.LANDLOCK_ACCESS_FS_EXECUTE |
		unix.LANDLOCK_ACCESS_FS_READ_FILE |
		unix.LANDLOCK_ACCESS_FS_READ_DIR)
}

func landlockWriteAccess(handled uint64) uint64 {
	return handled
}

func landlockDeviceWriteAccess(handled uint64) uint64 {
	return handled & (unix.LANDLOCK_ACCESS_FS_READ_FILE |
		unix.LANDLOCK_ACCESS_FS_WRITE_FILE |
		unix.LANDLOCK_ACCESS_FS_TRUNCATE)
}

func defaultSystemReadRoots() []string {
	return []string{
		"/bin",
		"/dev",
		"/etc",
		"/lib",
		"/lib64",
		"/proc",
		"/sbin",
		"/sys",
		"/usr",
	}
}
