//go:build darwin

// INPUT: 创建时保留的宿主 lease 句柄和原目录身份；不读取任务可写 marker。
// OUTPUT: 放行前可持久保存的 parent/leaf 文件系统身份；替换或释放即拒绝。
// POS: macOS 崩溃资源恢复的身份来源，不自行删除目录或清除 unknown。
package runtime

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type sandboxDirectoryIdentityParts struct {
	version string
	dev     uint64
	ino     uint64
	gen     uint64
	sec     uint64
	nsec    uint64
}

func sandboxDirectoryIdentity(info os.FileInfo) (string, error) {
	if info == nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", errors.New("scratch identity requires directory")
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Ino == 0 {
		return "", errors.New("scratch native directory identity unavailable")
	}
	// Device numbers can change when macOS remounts a volume after an upgrade;
	// dev is recorded but advisory, matching ignores it. The stable object
	// fields remain sufficient to distinguish replacement.
	return fmt.Sprintf("darwin-v1:%x:%x:%x:%x:%x", uint64(stat.Dev), stat.Ino, stat.Gen, stat.Birthtimespec.Sec, stat.Birthtimespec.Nsec), nil
}

func parseSandboxDirectoryIdentity(value string) (sandboxDirectoryIdentityParts, error) {
	var parts sandboxDirectoryIdentityParts
	fields := strings.Split(value, ":")
	if len(fields) != 6 {
		return parts, errors.New("invalid scratch directory identity")
	}
	parts.version = fields[0]
	// v2 is read-only legacy from an unreleased build; v1 is still written for downgrade compatibility.
	if parts.version != "darwin-v1" && parts.version != "darwin-v2" {
		return parts, errors.New("unsupported scratch directory identity")
	}
	start := 2
	var err error
	parts.dev, err = strconv.ParseUint(fields[1], 16, 64)
	if err != nil {
		return parts, errors.New("invalid scratch device identity")
	}
	values := []*uint64{&parts.ino, &parts.gen, &parts.sec, &parts.nsec}
	for i, dst := range values {
		if fields[start+i] == "" {
			return parts, errors.New("invalid scratch directory identity")
		}
		parsed, err := strconv.ParseUint(fields[start+i], 16, 64)
		if err != nil {
			return parts, errors.New("invalid scratch directory identity")
		}
		*dst = parsed
	}
	return parts, nil
}

// sandboxDirectoryIdentityMatches accepts legacy v1 records while treating a
// changed device number as migration drift. All object identity fields remain
// strict, so a deleted-and-recreated directory is still rejected.
func sandboxDirectoryIdentityMatches(expected, actual string) (matches, deviceDrift bool, err error) {
	want, err := parseSandboxDirectoryIdentity(expected)
	if err != nil {
		return false, false, err
	}
	got, err := parseSandboxDirectoryIdentity(actual)
	if err != nil {
		return false, false, err
	}
	if want.ino != got.ino || want.gen != got.gen || want.sec != got.sec || want.nsec != got.nsec {
		return false, false, nil
	}
	if want.dev != got.dev {
		return true, true, nil
	}
	return true, false, nil
}

func supervisedScratchBinding(lease *SandboxResourceLease) (protocol.SandboxProcessScratch, error) {
	empty := protocol.SandboxProcessScratch{}
	if lease == nil {
		return empty, nil
	}
	lease.mu.Lock()
	defer lease.mu.Unlock()
	if lease.released || lease.resource == nil {
		return empty, errors.New("scratch binding requires live lease")
	}
	resource := lease.resource
	resource.mu.Lock()
	defer resource.mu.Unlock()
	if resource.closed || resource.cleanupErr != nil || resource.base == nil {
		return empty, errors.New("scratch binding requires clean host resource")
	}
	baseID, err := sandboxDirectoryIdentity(resource.baseIdentity)
	if err != nil {
		return empty, err
	}
	leafID, err := sandboxDirectoryIdentity(resource.leafIdentity)
	if err != nil {
		return empty, err
	}
	name := filepath.Base(resource.path)
	if filepath.Dir(resource.path) != resource.root || !strings.HasPrefix(name, ".scratch-") || !filepath.IsAbs(resource.root) {
		return empty, errors.New("invalid host scratch path")
	}
	// Resolve the currently published parent and compare it with the original
	// handle. Never silently replace captured identity with a new directory.
	current, err := confinedfs.Open(resource.root)
	if err != nil {
		return empty, err
	}
	defer current.Close()
	info, err := current.Stat(".")
	if err != nil {
		return empty, err
	}
	currentID, err := sandboxDirectoryIdentity(info)
	if err != nil {
		return empty, err
	}
	if matches, drift, matchErr := sandboxDirectoryIdentityMatches(baseID, currentID); matchErr != nil || !matches {
		return empty, errors.New("scratch parent identity changed")
	} else if drift {
		slog.Warn("sandbox scratch parent device identity drifted", "path", resource.root)
	}
	leaf, err := current.OpenRootNoSymlink(name)
	if err != nil {
		return empty, err
	}
	defer leaf.Close()
	info, err = leaf.Stat(".")
	if err != nil {
		return empty, err
	}
	currentID, err = sandboxDirectoryIdentity(info)
	if err != nil {
		return empty, err
	}
	if matches, drift, matchErr := sandboxDirectoryIdentityMatches(leafID, currentID); matchErr != nil || !matches {
		return empty, errors.New("scratch leaf identity changed")
	} else if drift {
		slog.Warn("sandbox scratch leaf device identity drifted", "path", resource.path)
	}
	return protocol.SandboxProcessScratch{BasePath: resource.root, LeafName: name, BaseIdentity: baseID, LeafIdentity: leafID}, nil
}
