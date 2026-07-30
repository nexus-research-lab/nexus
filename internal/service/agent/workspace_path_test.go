package agent

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
)

func TestResolveWorkspacePathUsesSystemUserScope(t *testing.T) {
	cfg := config.Config{
		WorkspacePath: filepath.Join(t.TempDir(), "workspace"),
	}

	workspacePath := ResolveWorkspacePath(cfg, systemOwnerUserID, "nexus")

	expected := filepath.Join(cfg.WorkspacePath, "__system__", "workspace", "nexus")
	if workspacePath != expected {
		t.Fatalf("系统主路径不正确: got=%s want=%s", workspacePath, expected)
	}
}

func TestResolveWorkspacePathUsesUserRuntimeScope(t *testing.T) {
	cfg := config.Config{
		WorkspacePath: filepath.Join(t.TempDir(), "workspace"),
	}

	workspacePath := ResolveWorkspacePath(cfg, "user-123", "agent_abc123")

	expected := filepath.Join(cfg.WorkspacePath, "user-123", "workspace", "agent_abc123")
	if workspacePath != expected {
		t.Fatalf("多用户路径不正确: got=%s want=%s", workspacePath, expected)
	}
}

func TestWorkspaceBasePathDefaultsToUsersRoot(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), ".nexus")
	t.Setenv("NEXUS_STATE_ROOT", stateRoot)
	t.Setenv("NEXUS_CONFIG_DIR", "")

	got := WorkspaceBasePath(config.Config{})
	want := filepath.Join(stateRoot, "users")
	if got != want {
		t.Fatalf("workspace 默认根不正确: got=%q want=%q", got, want)
	}
	if got := ResolveWorkspacePath(config.Config{}, "user-123", "agent"); got != filepath.Join(want, "user-123", "workspace", "agent") {
		t.Fatalf("默认用户 workspace 路径不正确: got=%q", got)
	}
}

func TestWorkspaceBasePathExpandsHomeForms(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if os.Getenv("USERPROFILE") == "" {
		t.Setenv("USERPROFILE", home)
	}

	cases := []struct {
		name string
		path string
		want string
	}{
		{
			name: "slash",
			path: "~/.nexus/workspace",
			want: filepath.Join(home, ".nexus", "workspace"),
		},
		{
			name: "backslash",
			path: `~\.nexus\workspace`,
			want: filepath.Join(home, ".nexus", "workspace"),
		},
		{
			name: "home",
			path: "~",
			want: home,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got := WorkspaceBasePath(config.Config{WorkspacePath: tt.path})
			if got != tt.want {
				t.Fatalf("workspace home 展开不正确: got=%q want=%q", got, tt.want)
			}
		})
	}
}
