package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
)

const runtimeSettingsFileName = "runtime-settings.json"

// RuntimeSettings 表示可由 UI 持久化的主机级运行配置。
type RuntimeSettings struct {
	WorkspacePath string `json:"workspace_path,omitempty"`
	UpdatedAt     string `json:"updated_at,omitempty"`
}

// RuntimeSettingsPath 返回主机级运行配置文件路径。
func RuntimeSettingsPath() string {
	return filepath.Join(appfs.ConfigDir(), "config", runtimeSettingsFileName)
}

// LoadRuntimeSettings 读取主机级运行配置。
func LoadRuntimeSettings() (RuntimeSettings, error) {
	content, err := os.ReadFile(RuntimeSettingsPath())
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
	path := RuntimeSettingsPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return RuntimeSettings{}, err
	}
	payload, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return RuntimeSettings{}, err
	}
	payload = append(payload, '\n')
	tmpPath := path + ".tmp"
	if err = os.WriteFile(tmpPath, payload, 0o644); err != nil {
		return RuntimeSettings{}, err
	}
	if err = os.Rename(tmpPath, path); err != nil {
		return RuntimeSettings{}, err
	}
	return settings, nil
}

func normalizeRuntimeSettings(settings RuntimeSettings) RuntimeSettings {
	return RuntimeSettings{
		WorkspacePath: strings.TrimSpace(settings.WorkspacePath),
		UpdatedAt:     strings.TrimSpace(settings.UpdatedAt),
	}
}

func configuredWorkspacePath(envWorkspacePath string) string {
	settings, err := LoadRuntimeSettings()
	if err != nil {
		return strings.TrimSpace(envWorkspacePath)
	}
	settingsWorkspacePath := strings.TrimSpace(settings.WorkspacePath)
	if settingsWorkspacePath == "" {
		return strings.TrimSpace(envWorkspacePath)
	}
	if shouldUseRuntimeSettingsWorkspacePath(envWorkspacePath) {
		return settingsWorkspacePath
	}
	return strings.TrimSpace(envWorkspacePath)
}

func shouldUseRuntimeSettingsWorkspacePath(envWorkspacePath string) bool {
	value := strings.TrimSpace(envWorkspacePath)
	if value == "" {
		return true
	}
	if strings.TrimSpace(os.Getenv("NEXUS_APP_MODE")) != "desktop" {
		return false
	}
	return sameCleanPath(value, filepath.Join(appfs.ConfigDir(), "workspace"))
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
