// INPUT: runtime workspace 与 launcher 只读根配置。
// OUTPUT: 共享临时根进入统一的读写 policy。
// POS: 覆盖 app/web runtime 基础根集合的纯策略装配。

//go:build linux

package runtimeidentity

import (
	"slices"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
)

func TestBaseRuntimePolicyRootsIncludesSharedTemporaryRoot(t *testing.T) {
	sharedTempRoot := appfs.RuntimeSharedTempRoot()
	if sharedTempRoot == "" {
		t.Skip("当前平台没有 Unix 共享临时根")
	}

	ownerRoot := "/users/owner-a"
	readRoots, writeRoots := baseRuntimePolicyRoots(ownerRoot, launcherConfig{
		ReadOnlyRoots: []string{"/opt/templates"},
	})
	if !slices.Contains(readRoots, ownerRoot) || !slices.Contains(writeRoots, ownerRoot) {
		t.Fatalf("owner root missing from policy: read=%v write=%v", readRoots, writeRoots)
	}
	if !slices.Contains(readRoots, sharedTempRoot) {
		t.Fatalf("read roots = %v, want shared temp root %q", readRoots, sharedTempRoot)
	}
	if !slices.Contains(writeRoots, sharedTempRoot) {
		t.Fatalf("write roots = %v, want shared temp root %q", writeRoots, sharedTempRoot)
	}
}

func TestPathWithinSharedTempMatchesOnlyTheSharedRoot(t *testing.T) {
	sharedTempRoot := appfs.RuntimeSharedTempRoot()
	if sharedTempRoot == "" {
		t.Skip("当前平台没有 Unix 共享临时根")
	}

	if !pathWithinSharedTemp(sharedTempRoot + "/nexus-output.txt") {
		t.Fatalf("共享临时根下的路径应被允许: %q", sharedTempRoot+"/nexus-output.txt")
	}
	if pathWithinSharedTemp(sharedTempRoot + "-not-a-child") {
		t.Fatalf("共享临时根同名前缀不应被允许: %q", sharedTempRoot+"-not-a-child")
	}
}
