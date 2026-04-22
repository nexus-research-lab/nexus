package agent

import (
	"path/filepath"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
)

func TestResolveWorkspacePathKeepsSystemScopeFlat(t *testing.T) {
	cfg := config.Config{
		WorkspacePath: filepath.Join(t.TempDir(), "workspace"),
	}

	workspacePath := ResolveWorkspacePath(cfg, systemOwnerUserID, "nexus")

	expected := filepath.Join(cfg.WorkspacePath, "nexus")
	if workspacePath != expected {
		t.Fatalf("系统主路径不正确: got=%s want=%s", workspacePath, expected)
	}
}

func TestResolveWorkspacePathSimplifiesUserScope(t *testing.T) {
	cfg := config.Config{
		WorkspacePath: filepath.Join(t.TempDir(), "workspace"),
	}

	workspacePath := ResolveWorkspacePath(cfg, "user-123", "writer-agent")

	expected := filepath.Join(cfg.WorkspacePath, "user-123", "writer-agent")
	if workspacePath != expected {
		t.Fatalf("多用户路径不正确: got=%s want=%s", workspacePath, expected)
	}
}
