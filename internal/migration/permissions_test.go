package migration

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestMigratedPermissionsNeedHardening(t *testing.T) {
	tests := []struct {
		name                       string
		appMode                    string
		launcherManagesPermissions bool
		want                       bool
	}{
		{name: "desktop", appMode: "desktop", want: false},
		{name: "desktop mixed case", appMode: " Desktop ", want: false},
		{name: "linux launcher", appMode: "server", launcherManagesPermissions: true, want: false},
		{name: "ordinary server", appMode: "server", want: true},
		{name: "unspecified server", want: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv(nexusAppModeEnvironment, test.appMode)
			if got := shouldHardenMigratedPermissions(test.launcherManagesPermissions); got != test.want {
				t.Fatalf("shouldHardenMigratedPermissions() = %t, want %t", got, test.want)
			}
		})
	}
}

func TestDesktopMigrationPreservesNativeFilesystemModes(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows 不提供 Unix 权限位语义")
	}
	t.Setenv(nexusAppModeEnvironment, "desktop")
	stateRoot := filepath.Join(t.TempDir(), ".nexus")
	appRoot := filepath.Join(stateRoot, "app")
	usersRoot := filepath.Join(stateRoot, "users")
	sharedRoot := filepath.Join(stateRoot, "shared-workspaces")
	stateTarget := filepath.Join(usersRoot, "__system__", "state", "rooms", "conversation-a")
	assetTarget := filepath.Join(usersRoot, "__system__", "workspace", ".rooms", "conversation-a")
	testFile := filepath.Join(stateTarget, "overlay.jsonl")
	assetFile := filepath.Join(assetTarget, "asset.txt")
	for _, path := range []string{
		stateRoot,
		appRoot,
		usersRoot,
		sharedRoot,
		stateTarget,
		assetTarget,
	} {
		if err := os.MkdirAll(path, 0o770); err != nil {
			t.Fatalf("创建桌面迁移目录 %q: %v", path, err)
		}
		if err := os.Chmod(path, 0o770); err != nil {
			t.Fatalf("设置桌面迁移目录权限 %q: %v", path, err)
		}
	}
	for _, path := range []string{testFile, assetFile} {
		if err := os.WriteFile(path, []byte("data\n"), 0o660); err != nil {
			t.Fatalf("创建桌面迁移文件 %q: %v", path, err)
		}
		if err := os.Chmod(path, 0o660); err != nil {
			t.Fatalf("设置桌面迁移文件权限 %q: %v", path, err)
		}
	}

	if err := hardenMigratedStateLayout(
		stateRoot,
		appRoot,
		usersRoot,
		sharedRoot,
		false,
	); err != nil {
		t.Fatalf("桌面状态迁移不应修改原生权限: %v", err)
	}
	if err := hardenMigratedWorkspaceLayout(usersRoot, false); err != nil {
		t.Fatalf("桌面 workspace 迁移不应修改原生权限: %v", err)
	}
	if err := hardenMigratedRoomFiles(stateTarget, assetTarget, false); err != nil {
		t.Fatalf("桌面 Room 迁移不应修改原生权限: %v", err)
	}

	for _, expectation := range []struct {
		path string
		mode os.FileMode
	}{
		{path: stateRoot, mode: 0o770},
		{path: appRoot, mode: 0o770},
		{path: usersRoot, mode: 0o770},
		{path: stateTarget, mode: 0o770},
		{path: assetTarget, mode: 0o770},
		{path: testFile, mode: 0o660},
		{path: assetFile, mode: 0o660},
	} {
		info, err := os.Stat(expectation.path)
		if err != nil {
			t.Fatalf("读取桌面迁移权限 %q: %v", expectation.path, err)
		}
		if info.Mode().Perm() != expectation.mode {
			t.Fatalf("桌面迁移改写了 %q 权限: %o, want %o", expectation.path, info.Mode().Perm(), expectation.mode)
		}
	}
}
