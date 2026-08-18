package workspace

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/handler/handlertest"

	_ "modernc.org/sqlite"
)

func TestMain(m *testing.M) {
	os.Exit(handlertest.RunWithSelectedAppSkills(
		m,
		"execution-orchestrator",
		"goal-manager",
		"ima-skill",
		"imagegen",
		"visualize",
		"automation",
		"nexus-manager",
		"nexus-configuration",
		"room-playbook",
		"wechat-article-search",
		"werewolf-6p",
	))
}

func containsWorkspacePath(items []FileEntry, target string) bool {
	return slices.ContainsFunc(items, func(item FileEntry) bool {
		return item.Path == target
	})
}

func newWorkspaceTestConfig(t *testing.T) config.Config {
	t.Helper()

	root := t.TempDir()
	stateRoot := filepath.Join(root, ".nexus")
	t.Setenv("HOME", filepath.Join(root, "home"))
	t.Setenv("NEXUS_STATE_ROOT", stateRoot)
	t.Setenv("NEXUS_CONFIG_DIR", stateRoot)
	return config.Config{
		Host:                      "127.0.0.1",
		Port:                      18011,
		ProjectName:               "nexus-workspace-test",
		APIPrefix:                 "/nexus/v1",
		WebSocketPath:             "/nexus/v1/chat/ws",
		DefaultAgentID:            "nexus",
		WorkspacePath:             filepath.Join(root, "workspace"),
		CacheFileDir:              filepath.Join(root, "cache"),
		DatabaseDriver:            "sqlite",
		DatabaseURL:               filepath.Join(root, "nexus.db"),
		ConnectorOAuthRedirectURI: "http://localhost:3000/capability/connectors",
	}
}

func migrateWorkspaceSQLite(t *testing.T, databaseURL string) {
	t.Helper()
	handlertest.MigrateSQLiteFromDir(t, databaseURL, workspaceTestMigrationDir(t))
}

func workspaceTestMigrationDir(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("定位测试文件失败")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "db", "migrations", "sqlite")
}
