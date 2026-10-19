// INPUT: darwin-v1 身份串样本与持久绑定字段。
// OUTPUT: 稳定字段判等的接受与拒绝边界；device 漂移放行、格式或稳定字段差异拒绝。
// POS: 身份比较语义单测；不读取文件系统，不覆盖恢复编排链路。
package runtime

import (
	"strconv"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// driftScratchIdentityDevice 只改写 device 字段，模拟 macOS 升级后的卷设备重编号。
func driftScratchIdentityDevice(identity string, delta uint64) string {
	fields := strings.Split(identity, ":")
	device, err := strconv.ParseUint(fields[1], 16, 64)
	if err != nil {
		return identity
	}
	fields[1] = strconv.FormatUint(device+delta, 16)
	return strings.Join(fields, ":")
}

// bumpScratchIdentityInode 只改写 inode 字段，模拟目录被替换为不同对象。
func bumpScratchIdentityInode(identity string) string {
	fields := strings.Split(identity, ":")
	ino, err := strconv.ParseUint(fields[2], 16, 64)
	if err != nil {
		return identity
	}
	fields[2] = strconv.FormatUint(ino+1, 16)
	return strings.Join(fields, ":")
}

func TestScratchIdentityStableEqual(t *testing.T) {
	recorded := "darwin-v1:100000e:ac16092:0:6ab96c0e:96d1941"
	for _, tc := range []struct {
		name    string
		current string
		want    bool
	}{
		{"identical", recorded, true},
		{"device drift", "darwin-v1:1000010:ac16092:0:6ab96c0e:96d1941", true},
		{"zero device", "darwin-v1:0:ac16092:0:6ab96c0e:96d1941", true},
		{"inode differs", "darwin-v1:100000e:ac16093:0:6ab96c0e:96d1941", false},
		{"generation differs", "darwin-v1:100000e:ac16092:1:6ab96c0e:96d1941", false},
		{"birth seconds differ", "darwin-v1:100000e:ac16092:0:6ab96c0f:96d1941", false},
		{"birth nanoseconds differ", "darwin-v1:100000e:ac16092:0:6ab96c0e:96d1942", false},
		{"unknown version", "darwin-v2:100000e:ac16092:0:6ab96c0e:96d1941", false},
		{"missing prefix", "100000e:ac16092:0:6ab96c0e:96d1941", false},
		{"missing field", "darwin-v1:100000e:ac16092:0:6ab96c0e", false},
		{"extra field", "darwin-v1:100000e:ac16092:0:6ab96c0e:96d1941:1", false},
		{"non-hex field", "darwin-v1:100000e:zz16092:0:6ab96c0e:96d1941", false},
		{"empty field", "darwin-v1:100000e::0:6ab96c0e:96d1941", false},
		{"current empty", "", false},
	} {
		if got := sandboxIdentityStableEqual(recorded, tc.current); got != tc.want {
			t.Fatalf("%s: sandboxIdentityStableEqual=%v want %v", tc.name, got, tc.want)
		}
	}
	if !sandboxIdentityStableEqual("", "") {
		t.Fatal("empty identities must stay equal (zero-proof semantics)")
	}
}

func TestScratchBindingEqualToleratesOnlyDeviceDrift(t *testing.T) {
	recorded := protocol.SandboxProcessScratch{
		BasePath:     "/state/users/owner/runtime/sandbox",
		LeafName:     ".scratch-abc",
		BaseIdentity: "darwin-v1:100000e:ac16092:0:6ab96c0e:96d1941",
		LeafIdentity: "darwin-v1:100000e:ae7f41e:0:6abcadd4:1d1f3c7e",
	}
	current := protocol.SandboxProcessScratch{
		BasePath:     recorded.BasePath,
		LeafName:     recorded.LeafName,
		BaseIdentity: driftScratchIdentityDevice(recorded.BaseIdentity, 2),
		LeafIdentity: driftScratchIdentityDevice(recorded.LeafIdentity, 2),
	}
	if !sandboxScratchBindingEqual(recorded, current) {
		t.Fatal("device drift rejected")
	}
	for name, mutate := range map[string]func(protocol.SandboxProcessScratch) protocol.SandboxProcessScratch{
		"base path":  func(s protocol.SandboxProcessScratch) protocol.SandboxProcessScratch { s.BasePath += "x"; return s },
		"leaf name":  func(s protocol.SandboxProcessScratch) protocol.SandboxProcessScratch { s.LeafName = ".scratch-other"; return s },
		"base inode": func(s protocol.SandboxProcessScratch) protocol.SandboxProcessScratch { s.BaseIdentity = bumpScratchIdentityInode(s.BaseIdentity); return s },
		"leaf inode": func(s protocol.SandboxProcessScratch) protocol.SandboxProcessScratch { s.LeafIdentity = bumpScratchIdentityInode(s.LeafIdentity); return s },
		"malformed":  func(s protocol.SandboxProcessScratch) protocol.SandboxProcessScratch { s.BaseIdentity = "darwin-v1:zz"; return s },
		"future ver": func(s protocol.SandboxProcessScratch) protocol.SandboxProcessScratch { s.LeafIdentity = "darwin-v2:1:2:0:3:4"; return s },
	} {
		if sandboxScratchBindingEqual(recorded, mutate(current)) {
			t.Fatalf("%s drift accepted", name)
		}
	}
}
