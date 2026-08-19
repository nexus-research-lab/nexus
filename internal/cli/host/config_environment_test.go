// INPUT: host、owner runtime 与 Agent workspace 的不同环境组合。
// OUTPUT: nexusctl 目录还原及宿主环境保持行为的回归验证。
// POS: config_environment.go 的目录布局适配测试。
package host

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestLoadConfigRestoresHostRootsFromUserRuntime(t *testing.T) {
	root := t.TempDir()
	stateRoot := filepath.Join(root, ".nexus")
	ownerSegment := "user_demo"
	runtimeRoot := appfs.UserRuntimeRootAt(stateRoot, ownerSegment)
	agentWorkspace := filepath.Join(
		root,
		"custom-workspaces",
		ownerSegment,
		"workspace",
		"nexus",
		"src",
	)
	t.Setenv(appfs.NexusStateRootEnvName, "")
	t.Setenv(nexusConfigDirEnvName, runtimeRoot)
	t.Setenv(nexusctlWorkspacePathEnvName, agentWorkspace)
	t.Setenv(workspacePathEnvName, agentWorkspace)
	t.Setenv("DATABASE_DRIVER", "")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("NEXUS_APP_MODE", "")

	cfg := LoadConfig()

	if got, want := cfg.DatabaseURL, filepath.Join(stateRoot, "app", "data", "nexus.db"); got != want {
		t.Fatalf("nexusctl database path = %q, want %q", got, want)
	}
	if got, want := cfg.WorkspacePath, filepath.Join(root, "custom-workspaces"); got != want {
		t.Fatalf("nexusctl workspace base = %q, want %q", got, want)
	}
	if got := os.Getenv(nexusConfigDirEnvName); got != runtimeRoot {
		t.Fatalf("NEXUS_CONFIG_DIR 被改写: got=%q want=%q", got, runtimeRoot)
	}
	if got := os.Getenv(nexusctlWorkspacePathEnvName); got != agentWorkspace {
		t.Fatalf("NEXUSCTL_WORKSPACE_PATH 被改写: got=%q want=%q", got, agentWorkspace)
	}
}

func TestLoadConfigFallsBackToCanonicalUsersRoot(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), ".nexus")
	runtimeRoot := appfs.UserRuntimeRootAt(stateRoot, "user_demo")
	t.Setenv(appfs.NexusStateRootEnvName, "")
	t.Setenv(nexusConfigDirEnvName, runtimeRoot)
	t.Setenv(nexusctlWorkspacePathEnvName, filepath.Join(stateRoot, "shared-workspaces", "project-a"))
	t.Setenv(workspacePathEnvName, filepath.Join(stateRoot, "shared-workspaces", "project-a"))
	t.Setenv("DATABASE_DRIVER", "")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("NEXUS_APP_MODE", "")

	cfg := LoadConfig()

	if got, want := cfg.WorkspacePath, filepath.Join(stateRoot, "users"); got != want {
		t.Fatalf("nexusctl workspace base = %q, want %q", got, want)
	}
}

func TestLoadConfigLeavesHostEnvironmentUntouched(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), ".nexus")
	workspaceRoot := filepath.Join(t.TempDir(), "workspaces")
	t.Setenv(appfs.NexusStateRootEnvName, stateRoot)
	t.Setenv(nexusConfigDirEnvName, stateRoot)
	t.Setenv(nexusctlWorkspacePathEnvName, filepath.Join(workspaceRoot, "current"))
	t.Setenv(workspacePathEnvName, workspaceRoot)
	t.Setenv("DATABASE_DRIVER", "")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("NEXUS_APP_MODE", "")

	cfg := LoadConfig()

	if cfg.WorkspacePath != workspaceRoot {
		t.Fatalf("宿主 WORKSPACE_PATH 被改写: got=%q want=%q", cfg.WorkspacePath, workspaceRoot)
	}
}

func TestLoadConfigurationConfigDoesNotRestoreHostRootsForRuntimeBroker(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), ".nexus")
	runtimeRoot := appfs.UserRuntimeRootAt(stateRoot, "user_demo")
	t.Setenv(appfs.NexusStateRootEnvName, "")
	t.Setenv(nexusConfigDirEnvName, runtimeRoot)
	t.Setenv(workspacePathEnvName, filepath.Join(stateRoot, "users", "user_demo", "workspace", "agent-a"))
	t.Setenv(protocol.NexusConfigBrokerURLEnvName, "http://127.0.0.1:8010/nexus/v1/internal/runtime/configuration")
	t.Setenv(protocol.NexusConfigCapabilityTokenEnvName, "runtime-token")

	_ = LoadConfigurationConfig()

	if got := os.Getenv(appfs.NexusStateRootEnvName); got != "" {
		t.Fatalf("broker 模式不应恢复宿主状态根: %q", got)
	}
}
