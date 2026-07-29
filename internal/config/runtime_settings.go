// INPUT: UI 持久化的宿主 workspace 根、环境 workspace 与 runtime 调用方路径。
// OUTPUT: 私有 runtime-settings 文件及不会误把 Agent 当前目录当宿主根的配置。
// POS: config 包的宿主运行设置与 workspace 根规范化边界。
package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
)

const runtimeSettingsFileName = "runtime-settings.json"

// RuntimeSettings 表示可由 UI 持久化的主机级运行配置。
type RuntimeSettings struct {
	WorkspacePath string `json:"workspace_path,omitempty"`
	UpdatedAt     string `json:"updated_at,omitempty"`
}

// RuntimeSettingsPath 返回主机级运行配置文件路径。
func RuntimeSettingsPath() string {
	return filepath.Join(appfs.AppDir(), "config", runtimeSettingsFileName)
}

// LoadRuntimeSettings 读取主机级运行配置。
func LoadRuntimeSettings() (RuntimeSettings, error) {
	root, err := openRuntimeSettingsRoot(false)
	if errors.Is(err, os.ErrNotExist) {
		return RuntimeSettings{}, nil
	}
	if err != nil {
		return RuntimeSettings{}, err
	}
	defer root.Close()
	content, err := root.ReadFile(runtimeSettingsFileName)
	if errors.Is(err, os.ErrNotExist) {
		return RuntimeSettings{}, nil
	}
	if err != nil {
		return RuntimeSettings{}, err
	}
	var settings RuntimeSettings
	if err = json.Unmarshal(content, &settings); err != nil {
		return RuntimeSettings{}, err
	}
	return normalizeRuntimeSettings(settings), nil
}

// SaveRuntimeSettings 写入主机级运行配置。
func SaveRuntimeSettings(settings RuntimeSettings) (RuntimeSettings, error) {
	settings = normalizeRuntimeSettings(settings)
	settings.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	payload, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return RuntimeSettings{}, err
	}
	payload = append(payload, '\n')
	root, err := openRuntimeSettingsRoot(true)
	if err != nil {
		return RuntimeSettings{}, err
	}
	defer root.Close()
	if err = root.WriteFileAtomic(runtimeSettingsFileName, payload, 0o600); err != nil {
		return RuntimeSettings{}, err
	}
	return settings, nil
}

func openRuntimeSettingsRoot(create bool) (*confinedfs.Root, error) {
	stateRoot := appfs.StateRoot()
	if create {
		if err := os.MkdirAll(stateRoot, 0o700); err != nil {
			return nil, err
		}
	}
	root, err := confinedfs.Open(stateRoot)
	if err != nil {
		return nil, err
	}
	relative, err := filepath.Rel(stateRoot, filepath.Dir(RuntimeSettingsPath()))
	if err != nil {
		root.Close()
		return nil, err
	}
	relative = filepath.ToSlash(relative)
	if create {
		settingsRoot, openErr := root.OpenOrCreateRootNoSymlink(relative, 0o700)
		root.Close()
		return settingsRoot, openErr
	}
	settingsRoot, openErr := root.OpenRootNoSymlink(relative)
	root.Close()
	return settingsRoot, openErr
}

func normalizeRuntimeSettings(settings RuntimeSettings) RuntimeSettings {
	return RuntimeSettings{
		WorkspacePath: strings.TrimSpace(settings.WorkspacePath),
		UpdatedAt:     strings.TrimSpace(settings.UpdatedAt),
	}
}

func configuredWorkspacePath(envWorkspacePath string) string {
	if isAgentRuntimeWorkspacePath(envWorkspacePath) {
		return appfs.UsersRoot()
	}
	settings, err := LoadRuntimeSettings()
	if err != nil {
		return normalizeWorkspacePath(envWorkspacePath)
	}
	settingsWorkspacePath := strings.TrimSpace(settings.WorkspacePath)
	if settingsWorkspacePath == "" {
		return normalizeWorkspacePath(envWorkspacePath)
	}
	if shouldUseRuntimeSettingsWorkspacePath(envWorkspacePath) {
		return normalizeWorkspacePath(settingsWorkspacePath)
	}
	return normalizeWorkspacePath(envWorkspacePath)
}

func isAgentRuntimeWorkspacePath(envWorkspacePath string) bool {
	runtimeWorkspacePath := strings.TrimSpace(os.Getenv("NEXUSCTL_WORKSPACE_PATH"))
	return runtimeWorkspacePath != "" && sameCleanPath(envWorkspacePath, runtimeWorkspacePath)
}

func shouldUseRuntimeSettingsWorkspacePath(envWorkspacePath string) bool {
	value := strings.TrimSpace(envWorkspacePath)
	if value == "" {
		return true
	}
	if strings.TrimSpace(os.Getenv("NEXUS_APP_MODE")) != "desktop" {
		return false
	}
	return sameCleanPath(value, filepath.Join(appfs.StateRoot(), "workspace")) ||
		sameCleanPath(value, appfs.UsersRoot())
}

func normalizeWorkspacePath(path string) string {
	value := strings.TrimSpace(path)
	if value == "" {
		return ""
	}
	legacyDefault := filepath.Join(appfs.StateRoot(), "workspace")
	if sameCleanPath(value, legacyDefault) {
		return appfs.UsersRoot()
	}
	return value
}

func sameCleanPath(left string, right string) bool {
	left = filepath.Clean(expandLeadingHome(left))
	right = filepath.Clean(expandLeadingHome(right))
	if os.PathSeparator == '\\' {
		return strings.EqualFold(left, right)
	}
	return left == right
}

func expandLeadingHome(path string) string {
	value := strings.TrimSpace(path)
	switch {
	case value == "~":
		home, err := os.UserHomeDir()
		if err == nil {
			return home
		}
	case strings.HasPrefix(value, "~/"), strings.HasPrefix(value, `~\`):
		home, err := os.UserHomeDir()
		if err == nil {
			relative := strings.TrimLeft(value[2:], `/\`)
			relative = strings.ReplaceAll(relative, `\`, "/")
			return filepath.Join(home, filepath.FromSlash(relative))
		}
	}
	return value
}
