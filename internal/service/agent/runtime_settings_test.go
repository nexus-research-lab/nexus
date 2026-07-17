package agent_test

// 本文件验证 Agent 配置到 nxs workspace settings 的幂等投影。

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	agentpkg "github.com/nexus-research-lab/nexus/internal/service/agent"
)

func TestEnsureRuntimeSettingsProjectionWritesManagedRuntimeAndMemoryDefaults(t *testing.T) {
	workspace := t.TempDir()
	agentValue := protocol.Agent{
		AgentID:       "agent-settings",
		WorkspacePath: workspace,
		Options: protocol.Options{
			Provider: "provider-a",
			Model:    "model-a",
		},
	}

	if err := agentpkg.EnsureRuntimeSettingsProjection(agentValue); err != nil {
		t.Fatalf("EnsureRuntimeSettingsProjection() error = %v", err)
	}
	settings := readProjectedSettings(t, agentpkg.RuntimeSettingsPath(workspace))
	runtimeSettings := settings["runtime"].(map[string]any)
	if runtimeSettings["managedBy"] != "nexus" || runtimeSettings["providerRef"] != "provider-a" || runtimeSettings["model"] != "model-a" || runtimeSettings["backgroundModel"] != "model-a" {
		t.Fatalf("runtime settings = %#v, want managed Agent projection", runtimeSettings)
	}
	memorySettings := settings["memory"].(map[string]any)
	if _, exists := memorySettings["directory"]; exists {
		t.Fatalf("memory.directory = %#v, Nexus 不应改写 workspace 记忆根", memorySettings["directory"])
	}
	dreamSettings := memorySettings["dream"].(map[string]any)
	summarySettings := memorySettings["summary"].(map[string]any)
	if memorySettings["enabled"] != true || memorySettings["extractionEnabled"] != true || summarySettings["enabled"] != true || dreamSettings["enabled"] != true {
		t.Fatalf("memory settings = %#v, want enabled defaults", memorySettings)
	}
}

func TestEnsureRuntimeSettingsProjectionPreservesUserMemoryChoice(t *testing.T) {
	workspace := t.TempDir()
	path := agentpkg.RuntimeSettingsPath(workspace)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(path, []byte(`{
  "language": "zh-CN",
  "memory": {
    "enabled": true,
    "extractionEnabled": false,
    "summary": {"enabled": false},
    "dream": {"enabled": false, "minHours": 48}
  }
}`), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	agentValue := protocol.Agent{
		AgentID:       "agent-settings",
		WorkspacePath: workspace,
		Options:       protocol.Options{Provider: "provider-b", Model: "model-b"},
	}
	if err := agentpkg.EnsureRuntimeSettingsProjection(agentValue); err != nil {
		t.Fatalf("EnsureRuntimeSettingsProjection() error = %v", err)
	}
	settings := readProjectedSettings(t, path)
	if settings["language"] != "zh-CN" {
		t.Fatalf("language = %#v, want preserved", settings["language"])
	}
	memorySettings := settings["memory"].(map[string]any)
	dreamSettings := memorySettings["dream"].(map[string]any)
	summarySettings := memorySettings["summary"].(map[string]any)
	if memorySettings["extractionEnabled"] != false || summarySettings["enabled"] != false || dreamSettings["enabled"] != false || dreamSettings["minHours"] != float64(48) {
		t.Fatalf("memory settings = %#v, want preserved user choice", memorySettings)
	}
	runtimeSettings := settings["runtime"].(map[string]any)
	if runtimeSettings["providerRef"] != "provider-b" || runtimeSettings["model"] != "model-b" {
		t.Fatalf("runtime settings = %#v, want updated Agent projection", runtimeSettings)
	}
}

func TestEnsureRuntimeVisionSettingsProjectionPreservesOtherSettings(t *testing.T) {
	workspace := t.TempDir()
	path := agentpkg.RuntimeSettingsPath(workspace)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(path, []byte(`{"runtime":{"model":"main-model"},"memory":{"enabled":false}}`), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := agentpkg.EnsureRuntimeVisionSettingsProjection(workspace, "vision-provider", "vision-model"); err != nil {
		t.Fatalf("EnsureRuntimeVisionSettingsProjection() error = %v", err)
	}
	settings := readProjectedSettings(t, path)
	runtimeSettings := settings["runtime"].(map[string]any)
	visionSettings := runtimeSettings["vision"].(map[string]any)
	if runtimeSettings["model"] != "main-model" || visionSettings["providerRef"] != "vision-provider" || visionSettings["model"] != "vision-model" {
		t.Fatalf("runtime settings = %#v", runtimeSettings)
	}
	if settings["memory"].(map[string]any)["enabled"] != false {
		t.Fatalf("memory settings changed: %#v", settings["memory"])
	}
}

func readProjectedSettings(t *testing.T, path string) map[string]any {
	t.Helper()
	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	settings := map[string]any{}
	if err = json.Unmarshal(payload, &settings); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	return settings
}
