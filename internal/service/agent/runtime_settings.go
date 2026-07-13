package agent

// 本文件负责把 Agent 权威配置投影为 nxs 可独立读取的 workspace settings。

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

const runtimeSettingsRelativePath = ".nexus/settings.json"

// EnsureRuntimeSettingsProjection 幂等同步 Agent 的非敏感运行配置。
func EnsureRuntimeSettingsProjection(agentValue protocol.Agent) error {
	workspacePath := strings.TrimSpace(agentValue.WorkspacePath)
	if workspacePath == "" {
		return errors.New("Agent workspace 不能为空")
	}
	path := filepath.Join(workspacePath, filepath.FromSlash(runtimeSettingsRelativePath))
	settings, err := readRuntimeSettingsProjection(path)
	if err != nil {
		return err
	}
	original, err := json.Marshal(settings)
	if err != nil {
		return err
	}

	projectAgentRuntimeSettings(settings, agentValue)
	projectDefaultMemorySettings(settings)
	updated, err := json.Marshal(settings)
	if err != nil {
		return err
	}
	if string(original) == string(updated) {
		return nil
	}
	return writeRuntimeSettingsProjection(path, settings)
}

// RuntimeSettingsPath 返回指定 Agent 的 nxs project settings 路径。
func RuntimeSettingsPath(workspacePath string) string {
	workspacePath = strings.TrimSpace(workspacePath)
	if workspacePath == "" {
		return ""
	}
	return filepath.Join(workspacePath, filepath.FromSlash(runtimeSettingsRelativePath))
}

func readRuntimeSettingsProjection(path string) (map[string]any, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]any{}, nil
		}
		return nil, err
	}
	if len(strings.TrimSpace(string(payload))) == 0 {
		return map[string]any{}, nil
	}
	settings := map[string]any{}
	if err = json.Unmarshal(payload, &settings); err != nil {
		return nil, err
	}
	return settings, nil
}

func projectAgentRuntimeSettings(settings map[string]any, agentValue protocol.Agent) {
	runtimeSettings := objectSetting(settings, "runtime")
	runtimeSettings["managedBy"] = "nexus"
	runtimeSettings["version"] = 1
	setOptionalString(runtimeSettings, "providerRef", agentValue.Options.Provider)
	setOptionalString(runtimeSettings, "model", agentValue.Options.Model)
	setOptionalString(runtimeSettings, "backgroundModel", agentValue.Options.Model)
	settings["runtime"] = runtimeSettings
}

func projectDefaultMemorySettings(settings map[string]any) {
	memorySettings := objectSetting(settings, "memory")
	setDefault(memorySettings, "enabled", true)
	setDefault(memorySettings, "extractionEnabled", true)
	summarySettings := objectSetting(memorySettings, "summary")
	setDefault(summarySettings, "enabled", true)
	memorySettings["summary"] = summarySettings
	dreamSettings := objectSetting(memorySettings, "dream")
	setDefault(dreamSettings, "enabled", true)
	memorySettings["dream"] = dreamSettings
	settings["memory"] = memorySettings
}

func objectSetting(settings map[string]any, key string) map[string]any {
	if current, ok := settings[key].(map[string]any); ok {
		return current
	}
	return map[string]any{}
}

func setDefault(settings map[string]any, key string, value any) {
	if _, exists := settings[key]; !exists {
		settings[key] = value
	}
}

func setOptionalString(settings map[string]any, key string, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		delete(settings, key)
		return
	}
	settings[key] = value
}

func writeRuntimeSettingsProjection(path string, settings map[string]any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	payload, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".settings-*.json")
	if err != nil {
		return err
	}
	tempPath := file.Name()
	defer func() { _ = os.Remove(tempPath) }()
	if err = file.Chmod(0o600); err != nil {
		_ = file.Close()
		return err
	}
	if _, err = file.Write(append(payload, '\n')); err != nil {
		_ = file.Close()
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	return os.Rename(tempPath, path)
}
