//go:build linux

package runtimeidentity

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
)

func TestNormalizeCgroupConfigRequiresRootWhenEnabled(t *testing.T) {
	config := launcherConfig{
		StateRoot:      "/var/lib/nexus",
		CgroupRequired: true,
	}
	if err := normalizeCgroupConfig(&config); err == nil {
		t.Fatal("cgroup_required=true 且未配置 root 时应失败")
	}
}

func TestParseRuntimeProcessSignal(t *testing.T) {
	signal, sessionWide, err := parseRuntimeProcessSignal("kill")
	if err != nil || signal != unix.SIGKILL || !sessionWide {
		t.Fatalf("parse kill = signal:%v sessionWide:%v err:%v", signal, sessionWide, err)
	}
	if _, _, err = parseRuntimeProcessSignal("stop"); err == nil {
		t.Fatal("未知 runtime signal 应被拒绝")
	}
}

func TestSignalManagedRuntimeProcessChecksOwnerAndKernelSession(t *testing.T) {
	sessionID, err := unix.Getsid(os.Getpid())
	if err != nil {
		t.Fatalf("Getsid() error = %v", err)
	}
	exists, matched, err := signalManagedRuntimeProcess(
		launcherConfig{},
		&identity{UID: os.Getuid()},
		os.Getpid(),
		sessionID,
		0,
	)
	if err != nil || !exists || !matched {
		t.Fatalf("current process = exists:%v matched:%v err:%v", exists, matched, err)
	}
	exists, matched, err = signalManagedRuntimeProcess(
		launcherConfig{},
		&identity{UID: os.Getuid() + 1},
		os.Getpid(),
		sessionID,
		0,
	)
	if err != nil || !exists || matched {
		t.Fatalf("wrong owner = exists:%v matched:%v err:%v", exists, matched, err)
	}
}

func TestNormalizeCgroupConfigRejectsStateRootOverlap(t *testing.T) {
	config := launcherConfig{
		StateRoot:  "/var/lib/nexus",
		CgroupRoot: "/var/lib/nexus/cgroups",
	}
	if err := normalizeCgroupConfig(&config); err == nil {
		t.Fatal("cgroup_root 位于 state_root 下时应失败")
	}
}

func TestRuntimeCgroupPathRejectsTraversal(t *testing.T) {
	config := launcherConfig{CgroupRoot: "/sys/fs/cgroup/nexus"}
	for _, username := range []string{"", ".", "..", "../nxu-user", "nxu/user"} {
		if _, err := runtimeCgroupPath(config, username); err == nil {
			t.Fatalf("username %q 应被拒绝", username)
		}
	}
}

func TestRuntimeCgroupPathUsesStableBasename(t *testing.T) {
	config := launcherConfig{CgroupRoot: "/sys/fs/cgroup/nexus"}
	path, err := runtimeCgroupPath(config, "nxu_abc123")
	if err != nil {
		t.Fatalf("runtime cgroup path: %v", err)
	}
	want := filepath.Join(config.CgroupRoot, "nxu_abc123")
	if path != want || strings.Contains(path, "..") {
		t.Fatalf("path=%q, want %q", path, want)
	}
}
