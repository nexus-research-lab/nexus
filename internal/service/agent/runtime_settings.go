package agent

// 本文件负责把 Agent 权威配置投影为 nxs 可独立读取的 workspace settings。

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

const runtimeSettingsRelativePath = ".nexus/settings.json"

// EnsureRuntimeSettingsProjection 幂等同步 Agent 的非敏感运行配置。
func EnsureRuntimeSettingsProjection(agentValue protocol.Agent) error {
	workspacePath := strings.TrimSpace(agentValue.WorkspacePath)
	if workspacePath == "" {
		return errors.New("Agent workspace 不能为空")
	}
	root, err := confinedfs.Open(workspacePath)
	if err != nil {
		return err
	}
	defer root.Close()
	return ensureRuntimeSettingsProjectionAt(root, agentValue)
}

func ensureRuntimeSettingsProjectionAt(
	root *confinedfs.Root,
	agentValue protocol.Agent,
) error {
	settings, err := readRuntimeSettingsProjectionAt(root)
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
	return writeRuntimeSettingsProjectionAt(root, settings)
}

// LoadRuntimeSettingsProjection 从受限 workspace 根读取 nxs settings。
func LoadRuntimeSettingsProjection(workspacePath string) (map[string]any, error) {
	root, err := confinedfs.Open(workspacePath)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	return readRuntimeSettingsProjectionAt(root)
}

// RuntimeSettingsPath 返回指定 Agent 的 nxs project settings 路径。
func RuntimeSettingsPath(workspacePath string) string {
	workspacePath = strings.TrimSpace(workspacePath)
	if workspacePath == "" {
		return ""
	}
	return filepath.Join(workspacePath, filepath.FromSlash(runtimeSettingsRelativePath))
}

// EnsureRuntimeVisionSettingsProjection 把用户选择的非敏感视觉路由同步给 nxs。
func EnsureRuntimeVisionSettingsProjection(workspacePath string, providerRef string, model string) error {
	if strings.TrimSpace(workspacePath) == "" {
		return errors.New("Agent workspace 不能为空")
	}
	root, err := confinedfs.Open(workspacePath)
	if err != nil {
		return err
	}
	defer root.Close()
	return ensureRuntimeVisionSettingsProjectionAt(root, providerRef, model)
}

func ensureRuntimeVisionSettingsProjectionAt(
	root *confinedfs.Root,
	providerRef string,
	model string,
) error {
	settings, err := readRuntimeSettingsProjectionAt(root)
	if err != nil {
		return err
	}
	original, err := json.Marshal(settings)
	if err != nil {
		return err
	}
	runtimeSettings := objectSetting(settings, "runtime")
	visionSettings := objectSetting(runtimeSettings, "vision")
	setOptionalString(visionSettings, "providerRef", providerRef)
	setOptionalString(visionSettings, "model", model)
	if len(visionSettings) == 0 {
		delete(runtimeSettings, "vision")
	} else {
		runtimeSettings["vision"] = visionSettings
	}
	settings["runtime"] = runtimeSettings
	updated, err := json.Marshal(settings)
	if err != nil {
		return err
	}
	if string(original) == string(updated) {
		return nil
	}
	return writeRuntimeSettingsProjectionAt(root, settings)
}

func readRuntimeSettingsProjectionAt(root *confinedfs.Root) (map[string]any, error) {
	payload, err := root.ReadFile(runtimeSettingsRelativePath)
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
	// 后台模型属于 owner 偏好，可能与 Agent 主模型不同；由宿主在每次启动
	// bridge 时按当前选择写入环境，不能在 workspace 固化成旧的 Agent 模型。
	delete(runtimeSettings, "backgroundModel")
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

func writeRuntimeSettingsProjectionAt(
	root *confinedfs.Root,
	settings map[string]any,
) error {
	payload, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	return root.WriteFileAtomic(
		runtimeSettingsRelativePath,
		append(payload, '\n'),
		agentWorkspaceFileMode(0o600),
	)
}

// EnsureRuntimeSettingsProjection 在 owner workspace fd 内同步运行时配置。
func (s *Service) EnsureRuntimeSettingsProjection(agentValue protocol.Agent) error {
	root, err := s.openAgentWorkspace(agentValue, false)
	if err != nil {
		return err
	}
	defer root.Close()
	return ensureRuntimeSettingsProjectionAt(root, agentValue)
}

// EnsureRuntimeVisionSettingsProjection 在 owner workspace fd 内同步视觉路由。
func (s *Service) EnsureRuntimeVisionSettingsProjection(
	agentValue protocol.Agent,
	providerRef string,
	model string,
) error {
	root, err := s.openAgentWorkspace(agentValue, false)
	if err != nil {
		return err
	}
	defer root.Close()
	return ensureRuntimeVisionSettingsProjectionAt(root, providerRef, model)
}

// LoadRuntimeSettingsProjection 从 owner workspace fd 读取运行时配置。
func (s *Service) LoadRuntimeSettingsProjection(
	agentValue protocol.Agent,
) (map[string]any, error) {
	root, err := s.openAgentWorkspace(agentValue, false)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	return readRuntimeSettingsProjectionAt(root)
}
