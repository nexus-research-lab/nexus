// INPUT: 宿主目录别名、任务环境覆盖和两后端权限模式。
// OUTPUT: 受限策略包含宿主拒绝规则，Full Access 保持无隔离例外。
// POS: 选项装配测试；原生执行另由显式集成门禁证明。
package clientopts

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
)

func TestDesktopHostPathsPreserveAliasesAndFullAccess(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	state := filepath.Join(root, "state")
	if err := os.Mkdir(state, 0700); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(root, "alias")
	if err := os.Symlink(state, alias); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []agentclient.RuntimeKind{agentclient.RuntimeNXS, agentclient.RuntimeClaude} {
		before := agentclient.Options{Runtime: agentclient.RuntimeOptions{Kind: kind}, Sandbox: &agentclient.SandboxSettings{Filesystem: &agentclient.SandboxFilesystemConfig{DenyWrite: []string{"/existing"}}}}
		got, err := applyDesktopHostPaths(before, AgentClientOptionsInput{AppMode: "desktop"}, "darwin", filepath.Join(alias, "app"))
		if err != nil {
			t.Fatal(err)
		}
		for _, path := range []string{filepath.Join(alias, "app"), filepath.Join(state, "app")} {
			if !slices.Contains(got.Sandbox.Filesystem.DenyRead, filepath.Join(path, "data")) || !slices.Contains(got.Sandbox.Filesystem.DenyWrite, path) {
				t.Fatalf("missing root %s", path)
			}
			if kind == agentclient.RuntimeClaude && !slices.Contains(got.Tools.Deny, "Read(/"+filepath.Join(path, "data")+"/**)") {
				t.Fatal("Claude native file rule missing")
			}
		}
		if slices.Contains(got.Sandbox.Filesystem.DenyRead, filepath.Join(alias, "app")) {
			t.Fatal("private rule denies public Skill projection")
		}
		if len(before.Sandbox.Filesystem.DenyWrite) != 1 || len(before.Sandbox.Filesystem.DenyRead) != 0 {
			t.Fatal("caller policy mutated")
		}
		before.Runtime.PermissionMode = sdkpermission.ModeBypassPermissions
		full, err := applyDesktopHostPaths(before, AgentClientOptionsInput{AppMode: "desktop"}, "darwin", "relative-is-not-consumed")
		if err != nil || full.Sandbox != before.Sandbox || len(full.Tools.Deny) != 0 {
			t.Fatalf("Full Access changed: %+v %v", full, err)
		}
	}
}

func TestDesktopHostPathsIgnoreTaskStateRoot(t *testing.T) {
	root := t.TempDir()
	t.Setenv(appfs.NexusStateRootEnvName, root)
	got, err := BuildAgentClientOptions(context.Background(), nil, AgentClientOptionsInput{AppMode: "desktop", RuntimeKind: "nxs", WorkspacePath: t.TempDir(), OwnerUserID: "owner", ExtraEnv: map[string]string{appfs.NexusStateRootEnvName: "/task-controlled"}})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(got.Sandbox.Filesystem.DenyRead, filepath.Join(root, "app", "data")) || slices.Contains(got.Sandbox.Filesystem.DenyRead, "/task-controlled/app/data") {
		t.Fatal("task environment replaced host authority")
	}
}

func TestDesktopHostPathsResolveRelativeHostConfiguration(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	expected := filepath.Join(t.TempDir(), "state", "app")
	relative, err := filepath.Rel(cwd, expected)
	if err != nil {
		t.Fatal(err)
	}
	roots, err := desktopHostRootAliases(relative)
	if err != nil || !slices.Contains(roots, expected) {
		t.Fatalf("relative host root lost: %v %v", roots, err)
	}
}

func TestDesktopHostPathsProtectExistingPrivateDirectoryTarget(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	app := filepath.Join(root, "app")
	target := filepath.Join(root, "private data ")
	for _, path := range []string{app, target} {
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(target, filepath.Join(app, "data")); err != nil {
		t.Fatal(err)
	}
	options := agentclient.Options{Runtime: agentclient.RuntimeOptions{Kind: agentclient.RuntimeClaude}, Sandbox: &agentclient.SandboxSettings{Filesystem: &agentclient.SandboxFilesystemConfig{}}}
	got, err := applyDesktopHostPaths(options, AgentClientOptionsInput{AppMode: "desktop"}, "darwin", app)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(got.Sandbox.Filesystem.DenyRead, target) || !slices.Contains(got.Sandbox.Filesystem.DenyWrite, target) || !slices.Contains(got.Tools.Deny, "Edit(/"+target+"/**)") {
		t.Fatal("private target alias or trailing space lost")
	}
}
