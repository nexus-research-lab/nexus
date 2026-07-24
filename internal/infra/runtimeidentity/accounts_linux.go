//go:build linux

package runtimeidentity

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
)

func ensureIdentity(
	config launcherConfig,
	current *registry,
	ownerUserID string,
) (*identity, bool, error) {
	ownerUserID = strings.TrimSpace(ownerUserID)
	if ownerUserID == "" || len(ownerUserID) > 128 {
		return nil, false, errors.New("owner_user_id 无效")
	}
	if appfs.UserPathSegment(ownerUserID) != ownerUserID ||
		ownerUserID == "." || ownerUserID == ".." {
		return nil, false, errors.New("owner_user_id 不能安全映射为单一路径段")
	}
	if existing := current.Identities[ownerUserID]; existing != nil {
		if existing.Status != "active" {
			return nil, false, errors.New("runtime identity 已停用")
		}
		if err := ensureOSAccount(*existing); err != nil {
			return nil, false, err
		}
		changed, err := ensureIdentityLayout(config, existing)
		return existing, changed, err
	}
	accountName := stableAccountName("nxu", ownerUserID)
	numericID, recovered, err := recoverStableIdentityID(config, current, accountName)
	if err != nil {
		return nil, false, err
	}
	if !recovered {
		numericID, err = allocateNumericID(config, current)
		if err != nil {
			return nil, false, err
		}
	}
	userRoot := appfs.UserDataRootAt(config.StateRoot, ownerUserID)
	nextGeneration := current.Generation + 1
	created := &identity{
		OwnerUserID: ownerUserID,
		Username:    accountName,
		GroupName:   accountName,
		UID:         numericID,
		PrivateGID:  numericID,
		HomeDir:     filepath.Join(userRoot, "runtime", "home"),
		TempDir:     filepath.Join(userRoot, "runtime", "tmp"),
		Status:      "active",
		Generation:  nextGeneration,
	}
	if err = ensureOSAccount(*created); err != nil {
		return nil, false, err
	}
	if _, err = ensureIdentityLayout(config, created); err != nil {
		return nil, false, err
	}
	current.Generation = nextGeneration
	current.Identities[ownerUserID] = created
	return created, true, nil
}

// recoverStableIdentityID 处理“OS 账号已创建但 registry 尚未落盘”的中断
// 场景。启动迁移重试时沿用稳定账号的 UID，避免留下不可恢复的孤儿账号。
func recoverStableIdentityID(
	config launcherConfig,
	current *registry,
	accountName string,
) (int, bool, error) {
	candidate := 0
	if existing, err := user.Lookup(accountName); err == nil {
		uid, uidErr := strconv.Atoi(existing.Uid)
		gid, gidErr := strconv.Atoi(existing.Gid)
		if uidErr != nil || gidErr != nil || uid != gid {
			return 0, false, fmt.Errorf("OS 用户 %s 的 UID/GID 无法恢复", accountName)
		}
		candidate = uid
	} else {
		var unknown user.UnknownUserError
		if !errors.As(err, &unknown) {
			return 0, false, err
		}
	}
	if group, err := user.LookupGroup(accountName); err == nil {
		gid, parseErr := strconv.Atoi(group.Gid)
		if parseErr != nil {
			return 0, false, fmt.Errorf("OS group %s 的 GID 无法恢复", accountName)
		}
		if candidate != 0 && candidate != gid {
			return 0, false, fmt.Errorf("OS 用户 %s 与同名 group 的 UID/GID 不一致", accountName)
		}
		candidate = gid
	} else {
		var unknown user.UnknownGroupError
		if !errors.As(err, &unknown) {
			return 0, false, err
		}
	}
	if candidate == 0 {
		return 0, false, nil
	}
	if candidate < config.UIDMinimum || candidate > config.UIDMaximum ||
		candidate == config.HostUID || candidate == config.HostGID ||
		numericIDInRegistry(current, candidate) {
		return 0, false, fmt.Errorf("稳定 OS 账号 %s 的 UID/GID 不可复用: %d", accountName, candidate)
	}
	return candidate, true, nil
}

func recoverStableGroupID(
	config launcherConfig,
	current *registry,
	groupName string,
) (int, bool, error) {
	existing, err := user.LookupGroup(groupName)
	if err != nil {
		var unknown user.UnknownGroupError
		if errors.As(err, &unknown) {
			return 0, false, nil
		}
		return 0, false, err
	}
	gid, err := strconv.Atoi(existing.Gid)
	if err != nil {
		return 0, false, fmt.Errorf("OS group %s 的 GID 无法恢复", groupName)
	}
	if gid < config.UIDMinimum || gid > config.UIDMaximum ||
		gid == config.HostUID || gid == config.HostGID ||
		numericIDInRegistry(current, gid) {
		return 0, false, fmt.Errorf("稳定 OS group %s 的 GID 不可复用: %d", groupName, gid)
	}
	return gid, true, nil
}

func ensureOSAccount(value identity) error {
	if err := ensureOSGroup(value.GroupName, value.PrivateGID); err != nil {
		return err
	}
	existing, err := user.Lookup(value.Username)
	if err == nil {
		uid, uidErr := strconv.Atoi(existing.Uid)
		gid, gidErr := strconv.Atoi(existing.Gid)
		if uidErr != nil || gidErr != nil || uid != value.UID || gid != value.PrivateGID {
			return fmt.Errorf("OS 用户 %s 与 registry UID/GID 不一致", value.Username)
		}
		return nil
	}
	var unknown user.UnknownUserError
	if !errors.As(err, &unknown) {
		return err
	}
	if byID, lookupErr := user.LookupId(strconv.Itoa(value.UID)); lookupErr == nil {
		return fmt.Errorf("UID %d 已被 OS 用户 %s 占用", value.UID, byID.Username)
	}
	useradd, err := fixedExecutable("/usr/sbin/useradd", "/sbin/useradd")
	if err != nil {
		return err
	}
	command := exec.Command(
		useradd,
		"--no-create-home",
		"--no-user-group",
		"--uid", strconv.Itoa(value.UID),
		"--gid", value.GroupName,
		"--home-dir", value.HomeDir,
		"--shell", nologinPath(),
		value.Username,
	)
	command.Env = []string{"PATH=/usr/sbin:/usr/bin:/sbin:/bin", "LANG=C"}
	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("创建 runtime OS 用户 %s: %w: %s", value.Username, err, strings.TrimSpace(string(output)))
	}
	return nil
}

func ensureOSGroup(name string, gid int) error {
	existing, err := user.LookupGroup(name)
	if err == nil {
		actual, parseErr := strconv.Atoi(existing.Gid)
		if parseErr != nil || actual != gid {
			return fmt.Errorf("OS group %s 与 registry GID 不一致", name)
		}
		return nil
	}
	var unknown user.UnknownGroupError
	if !errors.As(err, &unknown) {
		return err
	}
	if byID, lookupErr := user.LookupGroupId(strconv.Itoa(gid)); lookupErr == nil {
		return fmt.Errorf("GID %d 已被 OS group %s 占用", gid, byID.Name)
	}
	groupadd, err := fixedExecutable("/usr/sbin/groupadd", "/sbin/groupadd")
	if err != nil {
		return err
	}
	command := exec.Command(groupadd, "--gid", strconv.Itoa(gid), name)
	command.Env = []string{"PATH=/usr/sbin:/usr/bin:/sbin:/bin", "LANG=C"}
	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("创建 runtime OS group %s: %w: %s", name, err, strings.TrimSpace(string(output)))
	}
	return nil
}

func verifyOSGroup(name string, gid int) error {
	name = strings.TrimSpace(name)
	if name == "" || gid <= 0 {
		return errors.New("runtime project group 无效")
	}
	existing, err := user.LookupGroup(name)
	if err != nil {
		return fmt.Errorf("runtime project group %s 不存在: %w", name, err)
	}
	actual, err := strconv.Atoi(existing.Gid)
	if err != nil || actual != gid {
		return fmt.Errorf("runtime project group %s 与 registry GID 不一致", name)
	}
	byID, err := user.LookupGroupId(strconv.Itoa(gid))
	if err != nil {
		return fmt.Errorf("runtime project GID %d 不存在: %w", gid, err)
	}
	if byID.Name != name {
		return fmt.Errorf("runtime project GID %d 已映射到其他 group %s", gid, byID.Name)
	}
	return nil
}

func allocateNumericID(config launcherConfig, current *registry) (int, error) {
	start := current.NextID
	if start < config.UIDMinimum || start > config.UIDMaximum {
		start = config.UIDMinimum
	}
	rangeSize := config.UIDMaximum - config.UIDMinimum + 1
	for offset := 0; offset < rangeSize; offset++ {
		candidate := config.UIDMinimum + (start-config.UIDMinimum+offset)%rangeSize
		if candidate == config.HostUID || candidate == config.HostGID ||
			numericIDInRegistry(current, candidate) || numericIDInOS(candidate) {
			continue
		}
		current.NextID = candidate + 1
		if current.NextID > config.UIDMaximum {
			current.NextID = config.UIDMinimum
		}
		return candidate, nil
	}
	return 0, errors.New("runtime UID/GID 范围已耗尽")
}

func numericIDInRegistry(current *registry, candidate int) bool {
	for _, value := range current.Identities {
		if value != nil && (value.UID == candidate || value.PrivateGID == candidate) {
			return true
		}
	}
	for _, value := range current.Projects {
		if value != nil && value.GID == candidate {
			return true
		}
	}
	return false
}

func numericIDInOS(candidate int) bool {
	value := strconv.Itoa(candidate)
	if _, err := user.LookupId(value); err == nil {
		return true
	}
	if _, err := user.LookupGroupId(value); err == nil {
		return true
	}
	return false
}

func stableAccountName(prefix string, value string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(value)))
	return prefix + "-" + hex.EncodeToString(sum[:8])
}

func fixedExecutable(candidates ...string) (string, error) {
	for _, candidate := range candidates {
		info, err := os.Stat(candidate)
		if err != nil || !info.Mode().IsRegular() ||
			info.Mode().Perm()&0o111 == 0 || info.Mode().Perm()&0o022 != 0 {
			continue
		}
		stat, ok := info.Sys().(*syscall.Stat_t)
		if ok && stat.Uid == 0 {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("找不到受信任的系统命令: %s", strings.Join(candidates, ", "))
}

func nologinPath() string {
	for _, candidate := range []string{"/usr/sbin/nologin", "/sbin/nologin"} {
		if info, err := os.Stat(candidate); err == nil && info.Mode().IsRegular() {
			return candidate
		}
	}
	return "/bin/false"
}
