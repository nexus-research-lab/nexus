// INPUT: 宿主目录别名、任务环境覆盖和两后端权限模式。
// OUTPUT: 受限策略包含宿主拒绝规则，Full Access 保持无隔离例外。
// POS: 选项装配测试；原生执行另由显式集成门禁证明。
package clientopts

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
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
		if runtime.GOOS == "windows" {
			t.Skipf("Windows symlink aliases require the SeCreateSymbolicLinkPrivilege: %v", err)
		}
		t.Fatal(err)
	}
	for _, kind := range []agentclient.RuntimeKind{agentclient.RuntimeNXS, agentclient.RuntimeClaude} {
		before := agentclient.Options{Runtime: agentclient.RuntimeOptions{Kind: kind}, Sandbox: &agentclient.SandboxSettings{Filesystem: &agentclient.SandboxFilesystemConfig{DenyWrite: []string{"/existing"}}}}
		got, err := applyDesktopHostPaths(before, AgentClientOptionsInput{AppMode: "desktop", DesktopSandboxEnabled: true}, "darwin", filepath.Join(alias, "app"), "")
		if err != nil {
			t.Fatal(err)
		}
		for _, path := range []string{filepath.Join(alias, "app"), filepath.Join(state, "app")} {
			record := filepath.Join(filepath.Dir(path), "NexusSidecar.pid.json")
			if !slices.Contains(got.Sandbox.Filesystem.DenyRead, record) || !slices.Contains(got.Sandbox.Filesystem.DenyWrite, record) {
				t.Fatal("sidecar identity record unprotected")
			}
			if kind == agentclient.RuntimeClaude && !slices.Contains(got.Tools.Deny, "Edit(/"+filepath.ToSlash(record)+")") {
				t.Fatal("Claude record write rule missing")
			}
			if !slices.Contains(got.Sandbox.Filesystem.DenyRead, filepath.Join(path, "data")) || !slices.Contains(got.Sandbox.Filesystem.DenyWrite, path) {
				t.Fatalf("missing root %s", path)
			}
			if kind == agentclient.RuntimeClaude && !slices.Contains(got.Tools.Deny, "Read(/"+filepath.ToSlash(filepath.Join(path, "data"))+"/**)") {
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
		full, err := applyDesktopHostPaths(before, AgentClientOptionsInput{AppMode: "desktop", DesktopSandboxEnabled: true}, "darwin", "relative-is-not-consumed", "/Users/someone")
		if err != nil || full.Sandbox != before.Sandbox || len(full.Tools.Deny) != 0 {
			t.Fatalf("Full Access changed: %+v %v", full, err)
		}
	}
}

func TestDesktopHostPathsDenyUserSecretReads(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	home := filepath.Join(root, "home")
	if err := os.MkdirAll(filepath.Join(home, ".ssh"), 0700); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []agentclient.RuntimeKind{agentclient.RuntimeNXS, agentclient.RuntimeClaude} {
		before := agentclient.Options{Runtime: agentclient.RuntimeOptions{Kind: kind}, Sandbox: &agentclient.SandboxSettings{Filesystem: &agentclient.SandboxFilesystemConfig{}}}
		got, err := applyDesktopHostPaths(before, AgentClientOptionsInput{AppMode: "desktop", DesktopSandboxEnabled: true}, "darwin", filepath.Join(root, "state", "app"), home)
		if err != nil {
			t.Fatal(err)
		}
		for _, secret := range []string{filepath.Join(home, ".ssh"), filepath.Join(home, ".aws"), filepath.Join(home, "Library", "Keychains")} {
			if !slices.Contains(got.Sandbox.Filesystem.DenyRead, secret) {
				t.Fatalf("%s: secret path readable: %s", kind, secret)
			}
			if slices.Contains(got.Sandbox.Filesystem.DenyWrite, secret) {
				t.Fatalf("%s: secret path needlessly added to write denials: %s", kind, secret)
			}
			if kind == agentclient.RuntimeClaude && !slices.Contains(got.Tools.Deny, "Read(/"+filepath.ToSlash(secret)+"/**)") {
				t.Fatalf("Claude secret read rule missing: %s", secret)
			}
		}
	}
}

func TestDesktopHostPathsIgnoreTaskStateRoot(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("完整桌面沙箱装配仅在 macOS 启用；宿主路径规则由显式平台测试覆盖")
	}
	root := t.TempDir()
	t.Setenv(appfs.NexusStateRootEnvName, root)
	got, err := BuildAgentClientOptions(context.Background(), nil, AgentClientOptionsInput{AppMode: "desktop", DesktopSandboxEnabled: true, RuntimeKind: "nxs", WorkspacePath: t.TempDir(), OwnerUserID: "owner", ExtraEnv: map[string]string{appfs.NexusStateRootEnvName: "/task-controlled"}})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(got.Sandbox.Filesystem.DenyRead, filepath.Join(root, "app", "data")) || slices.Contains(got.Sandbox.Filesystem.DenyRead, "/task-controlled/app/data") {
		t.Fatal("task environment replaced host authority")
	}
}

func TestDesktopHostPathsProtectWindowsNXSState(t *testing.T) {
	root := filepath.Clean(t.TempDir())
	appRoot := filepath.Join(root, "app")
	if err := os.MkdirAll(filepath.Join(appRoot, "data"), 0700); err != nil {
		t.Fatal(err)
	}
	options, err := applyDesktopHostPaths(
		agentclient.Options{
			Runtime: agentclient.RuntimeOptions{Kind: agentclient.RuntimeNXS},
			Sandbox: &agentclient.SandboxSettings{Filesystem: &agentclient.SandboxFilesystemConfig{}},
		},
		AgentClientOptionsInput{AppMode: "desktop", DesktopSandboxEnabled: true},
		"windows",
		appRoot, "",
	)
	if err != nil {
		t.Fatal(err)
	}
	filesystem := options.Sandbox.Filesystem
	if !slices.Contains(filesystem.DenyRead, filepath.Join(appRoot, "data")) {
		t.Fatalf("Windows state data directory is readable: %#v", filesystem.DenyRead)
	}
	if !slices.Contains(filesystem.DenyWrite, appRoot) {
		t.Fatalf("Windows app root is writable: %#v", filesystem.DenyWrite)
	}
}

func TestDesktopHostPathsApplyToWindowsRestrictedNXS(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows MXC host protection")
	}
	appRoot := filepath.Join(t.TempDir(), "state", "app")
	options := agentclient.Options{
		Runtime: agentclient.RuntimeOptions{Kind: agentclient.RuntimeNXS},
		Sandbox: &agentclient.SandboxSettings{Filesystem: &agentclient.SandboxFilesystemConfig{}},
	}
	got, err := applyDesktopHostPaths(options, AgentClientOptionsInput{AppMode: "desktop", DesktopSandboxEnabled: true}, "windows", appRoot, "")
	if err != nil {
		t.Fatal(err)
	}
	data := filepath.Join(appRoot, "data")
	record := filepath.Join(filepath.Dir(appRoot), "NexusSidecar.pid.json")
	if !slices.Contains(got.Sandbox.Filesystem.DenyRead, data) || !slices.Contains(got.Sandbox.Filesystem.DenyWrite, appRoot) {
		t.Fatalf("Windows host app paths are not protected: %#v", got.Sandbox.Filesystem)
	}
	if !slices.Contains(got.Sandbox.Filesystem.DenyRead, record) || !slices.Contains(got.Sandbox.Filesystem.DenyWrite, record) {
		t.Fatalf("Windows sidecar record is not protected: %#v", got.Sandbox.Filesystem)
	}
}

func TestDesktopHostPathsDenyUnregisteredAppEntriesAndControl(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	app := filepath.Join(root, "app")
	for _, name := range []string{"debug", "computer-use", "platform-skills", "host-skills"} {
		if err := os.MkdirAll(filepath.Join(app, name), 0700); err != nil {
			t.Fatal(err)
		}
	}
	before := agentclient.Options{Runtime: agentclient.RuntimeOptions{Kind: agentclient.RuntimeNXS}, Sandbox: &agentclient.SandboxSettings{Filesystem: &agentclient.SandboxFilesystemConfig{}}}
	got, err := applyDesktopHostPaths(before, AgentClientOptionsInput{AppMode: "desktop", DesktopSandboxEnabled: true}, "darwin", app, "")
	if err != nil {
		t.Fatal(err)
	}
	denyRead := got.Sandbox.Filesystem.DenyRead
	for _, path := range []string{filepath.Join(app, "debug"), filepath.Join(app, "computer-use"), filepath.Join(app, "backups"), filepath.Join(root, "control")} {
		if !slices.Contains(denyRead, path) {
			t.Fatalf("%s readable", path)
		}
	}
	if !slices.Contains(got.Sandbox.Filesystem.DenyWrite, filepath.Join(root, "control")) {
		t.Fatal("Control directory writable")
	}
	for _, path := range []string{app, filepath.Join(app, "platform-skills"), filepath.Join(app, "host-skills")} {
		if slices.Contains(denyRead, path) {
			t.Fatalf("Skill projection %s denied", path)
		}
	}
}
