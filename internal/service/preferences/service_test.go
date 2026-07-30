package preferences

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestDefaultPreferencesAskByDefault(t *testing.T) {
	prefs := DefaultPreferences()
	if prefs.DefaultAgentOptions.PermissionMode != "default" {
		t.Fatalf("默认权限应为询问模式: %+v", prefs.DefaultAgentOptions)
	}
	if len(prefs.DefaultAgentOptions.AllowedTools) != 0 {
		t.Fatalf("默认不应预授权工具: %+v", prefs.DefaultAgentOptions.AllowedTools)
	}
	if prefs.AgentRuntimeKind != "nxs" {
		t.Fatalf("默认 runtime 应为 nxs: %+v", prefs)
	}
	if prefs.AgentSDKDiagnosticsEnabled {
		t.Fatalf("Agent SDK diagnostics 默认应关闭: %+v", prefs)
	}
	if prefs.ToolSearchEnabledForRuntime("nxs") {
		t.Fatalf("nxs ToolSearch 默认应关闭: %+v", prefs)
	}
	if !prefs.WebSearch.Enabled || prefs.WebSearch.Provider != "anysearch" {
		t.Fatalf("WebSearch 默认 provider 应为 anysearch: %+v", prefs.WebSearch)
	}

	normalized := normalizePreferences(Preferences{})
	if normalized.DefaultAgentOptions.PermissionMode != "default" {
		t.Fatalf("空偏好归一化后应为询问模式: %+v", normalized.DefaultAgentOptions)
	}
	if normalized.AgentRuntimeKind != "nxs" {
		t.Fatalf("空偏好归一化后 runtime 应为 nxs: %+v", normalized)
	}
	if normalized.AgentSDKDiagnosticsEnabled {
		t.Fatalf("空偏好归一化后 Agent SDK diagnostics 应关闭: %+v", normalized)
	}
	if normalized.ToolSearchEnabledForRuntime("nxs") {
		t.Fatalf("空偏好归一化后 nxs ToolSearch 应关闭: %+v", normalized)
	}
	if !normalized.WebSearch.Enabled || normalized.WebSearch.Provider != "anysearch" {
		t.Fatalf("空偏好归一化后 WebSearch provider 应为 anysearch: %+v", normalized.WebSearch)
	}
}

func TestServiceUpdatePersistsUserPreferences(t *testing.T) {
	root := t.TempDir()
	service := NewService(config.Config{WorkspacePath: filepath.Join(root, "workspace")})

	prefs, err := service.Update(context.Background(), "user/1", UpdateRequest{
		ChatDefaultDeliveryPolicy:  policyPointer(protocol.ChatDeliveryPolicyQueue),
		AgentRuntimeKind:           stringPointer("nxs"),
		AgentSDKDiagnosticsEnabled: boolPointer(true),
		RuntimeSettings: &RuntimeSettings{
			"nxs":    {ToolSearch: true},
			"claude": {ToolSearch: true},
		},
		DefaultAgentOptions: &protocol.Options{
			PermissionMode: "default",
			Provider:       "glm-coding-plan",
			Model:          "glm-5.1",
			AllowedTools:   []string{"Read", "Read", "Write"},
		},
		DefaultImageModelSelection: &ModelSelection{
			Provider: "image-provider",
			Model:    "image-model",
		},
		DefaultVisionModelSelection: &ModelSelection{
			Provider: "vision-provider",
			Model:    "vision-model",
		},
		DefaultBackgroundModelSelection: &ModelSelection{
			Provider: "background-provider",
			Model:    "background-model",
		},
	})
	if err != nil {
		t.Fatalf("更新偏好失败: %v", err)
	}
	if prefs.ChatDefaultDeliveryPolicy != protocol.ChatDeliveryPolicyQueue {
		t.Fatalf("消息行为未持久化: %+v", prefs)
	}
	if prefs.AgentRuntimeKind != "nxs" {
		t.Fatalf("runtime 偏好未持久化: %+v", prefs)
	}
	if !prefs.AgentSDKDiagnosticsEnabled {
		t.Fatalf("Agent SDK diagnostics 偏好未持久化: %+v", prefs)
	}
	if !prefs.ToolSearchEnabledForRuntime("nxs") || prefs.ToolSearchEnabledForRuntime("claude") {
		t.Fatalf("ToolSearch 应只在 nxs runtime 生效: %+v", prefs.RuntimeSettings)
	}
	if prefs.DefaultAgentOptions.PermissionMode != "default" {
		t.Fatalf("权限模式未持久化: %+v", prefs.DefaultAgentOptions)
	}
	if len(prefs.DefaultAgentOptions.AllowedTools) != 2 {
		t.Fatalf("工具列表应去重: %+v", prefs.DefaultAgentOptions.AllowedTools)
	}
	if prefs.DefaultAgentOptions.Provider != "glm-coding-plan" || prefs.DefaultAgentOptions.Model != "glm-5.1" {
		t.Fatalf("默认 Agent 模型未持久化: %+v", prefs.DefaultAgentOptions)
	}
	if prefs.DefaultImageModelSelection.Provider != "image-provider" || prefs.DefaultImageModelSelection.Model != "image-model" {
		t.Fatalf("默认生图模型未持久化: %+v", prefs.DefaultImageModelSelection)
	}
	if prefs.DefaultVisionModelSelection.Provider != "vision-provider" || prefs.DefaultVisionModelSelection.Model != "vision-model" {
		t.Fatalf("视觉模型未持久化: %+v", prefs.DefaultVisionModelSelection)
	}
	if prefs.DefaultBackgroundModelSelection.Provider != "background-provider" || prefs.DefaultBackgroundModelSelection.Model != "background-model" {
		t.Fatalf("后台任务模型未持久化: %+v", prefs.DefaultBackgroundModelSelection)
	}

	loaded, err := service.Get(context.Background(), "user/1")
	if err != nil {
		t.Fatalf("读取偏好失败: %v", err)
	}
	if loaded.ChatDefaultDeliveryPolicy != protocol.ChatDeliveryPolicyQueue ||
		loaded.AgentRuntimeKind != "nxs" ||
		!loaded.AgentSDKDiagnosticsEnabled ||
		!loaded.ToolSearchEnabledForRuntime("nxs") ||
		loaded.DefaultAgentOptions.PermissionMode != "default" {
		t.Fatalf("读取结果不正确: %+v", loaded)
	}
	if loaded.DefaultImageModelSelection.Model != "image-model" || loaded.DefaultVisionModelSelection.Model != "vision-model" || loaded.DefaultBackgroundModelSelection.Model != "background-model" {
		t.Fatalf("读取默认模型选择不正确: %+v", loaded)
	}
	preferencesPath := testUserSettingsPath(root, "user/1", "preferences.json")
	info, statErr := os.Stat(preferencesPath)
	if statErr != nil {
		t.Fatalf("偏好文件未写入安全路径: %v", statErr)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("偏好文件权限不正确: got=%#o want=%#o", info.Mode().Perm(), 0o600)
	}
	settingsInfo, statErr := os.Stat(filepath.Dir(preferencesPath))
	if statErr != nil || settingsInfo.Mode().Perm() != 0o700 {
		t.Fatalf("偏好目录权限不正确: info=%v err=%v", settingsInfo, statErr)
	}
}

func TestServiceStoresWebSearchAPIKeySeparately(t *testing.T) {
	root := t.TempDir()
	service := NewService(config.Config{WorkspacePath: filepath.Join(root, "workspace")})
	apiKey := "secret-search-key"
	_, err := service.Update(context.Background(), "user/1", UpdateRequest{
		WebSearch: &WebSearchSettings{
			Enabled:  true,
			Provider: "brave",
		},
		WebSearchAPIKey: &apiKey,
	})
	if err != nil {
		t.Fatalf("更新 WebSearch 偏好失败: %v", err)
	}
	preferencesPath := testUserSettingsPath(root, "user/1", "preferences.json")
	content, err := os.ReadFile(preferencesPath)
	if err != nil {
		t.Fatalf("读取偏好文件失败: %v", err)
	}
	if string(content) == "" || strings.Contains(string(content), apiKey) {
		t.Fatalf("偏好文件不应包含 API key: %s", content)
	}
	loaded, err := service.Get(context.Background(), "user/1")
	if err != nil {
		t.Fatalf("读取 WebSearch 偏好失败: %v", err)
	}
	if loaded.WebSearch.Provider != "brave" || !loaded.WebSearch.APIKeyConfigured || loaded.WebSearchAPIKey() != apiKey {
		t.Fatalf("WebSearch 凭据未恢复: %+v", loaded.WebSearch)
	}
	keyPath := testUserSettingsPath(root, "user/1", "web-search-api-key")
	if info, err := os.Stat(keyPath); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("API key 文件权限不正确: info=%v err=%v", info, err)
	}
	credentialContent, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatalf("读取 WebSearch 凭据文件失败: %v", err)
	}
	var credential storedWebSearchCredential
	if err = json.Unmarshal(credentialContent, &credential); err != nil {
		t.Fatalf("WebSearch 凭据文件格式不正确: %v", err)
	}
	if credential.Provider != "brave" || credential.APIKey != apiKey {
		t.Fatalf("WebSearch 凭据未绑定 provider: %+v", credential)
	}

	empty := ""
	if _, err := service.Update(context.Background(), "user/1", UpdateRequest{WebSearchAPIKey: &empty}); err != nil {
		t.Fatalf("清除 WebSearch API key 失败: %v", err)
	}
	loaded, err = service.Get(context.Background(), "user/1")
	if err != nil {
		t.Fatalf("读取清除后的 WebSearch 偏好失败: %v", err)
	}
	if loaded.WebSearch.APIKeyConfigured || loaded.WebSearchAPIKey() != "" {
		t.Fatalf("WebSearch API key 未清除: %+v", loaded.WebSearch)
	}
}

func TestServicePersistsWebSearchSettings(t *testing.T) {
	root := t.TempDir()
	service := NewService(config.Config{WorkspacePath: filepath.Join(root, "workspace")})
	apiKey := "secret-search-key"

	if _, err := service.Update(context.Background(), "user/1", UpdateRequest{
		WebSearch: &WebSearchSettings{
			Enabled:  true,
			Provider: "brave",
		},
		WebSearchAPIKey: &apiKey,
	}); err != nil {
		t.Fatalf("写入 WebSearch 凭据失败: %v", err)
	}

	prefs, err := service.Update(context.Background(), "user/1", UpdateRequest{
		WebSearch: &WebSearchSettings{
			Enabled:  true,
			Provider: "anysearch",
			BaseURL:  " https://ignored.example.com ",
		},
	})
	if err != nil {
		t.Fatalf("切换 AnySearch 失败: %v", err)
	}
	if prefs.WebSearch.Provider != "anysearch" || prefs.WebSearch.BaseURL != "https://ignored.example.com" || prefs.WebSearch.APIKeyConfigured || prefs.WebSearchAPIKey() != "" {
		t.Fatalf("AnySearch 配置未正确归一化: %+v", prefs.WebSearch)
	}
	keyPath := testUserSettingsPath(root, "user/1", "web-search-api-key")
	if _, err := os.Stat(keyPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("切换无凭据 provider 后应删除旧 API key: %v", err)
	}

	prefs, err = service.Update(context.Background(), "user/1", UpdateRequest{
		WebSearch: &WebSearchSettings{
			Enabled:  true,
			Provider: "searxng",
			BaseURL:  " https://search.example.com ",
		},
		WebSearchAPIKey: &apiKey,
	})
	if err != nil {
		t.Fatalf("更新 SearXNG 配置失败: %v", err)
	}
	if prefs.WebSearch.BaseURL != "https://search.example.com" || prefs.WebSearch.APIKeyConfigured || prefs.WebSearchAPIKey() != "" {
		t.Fatalf("SearXNG 应只保留 Base URL: %+v", prefs.WebSearch)
	}
}

func TestServiceStoresOptionalAnySearchAPIKey(t *testing.T) {
	root := t.TempDir()
	service := NewService(config.Config{WorkspacePath: filepath.Join(root, "workspace")})
	apiKey := "anysearch-key"

	prefs, err := service.Update(context.Background(), "user/1", UpdateRequest{
		WebSearch:       &WebSearchSettings{Enabled: true, Provider: "anysearch"},
		WebSearchAPIKey: &apiKey,
	})
	if err != nil {
		t.Fatalf("写入 AnySearch API key 失败: %v", err)
	}
	if !prefs.WebSearch.Enabled || !prefs.WebSearch.APIKeyConfigured || prefs.WebSearchAPIKey() != apiKey || prefs.WebSearch.APIKeyMasked != "anyse************************h-key" {
		t.Fatalf("AnySearch API key 未保存: %+v", prefs.WebSearch)
	}

	loaded, err := service.Get(context.Background(), "user/1")
	if err != nil {
		t.Fatalf("读取 AnySearch API key 失败: %v", err)
	}
	if !loaded.WebSearch.APIKeyConfigured || loaded.WebSearchAPIKey() != apiKey || loaded.WebSearch.APIKeyMasked != "anyse************************h-key" {
		t.Fatalf("AnySearch API key 未恢复: %+v", loaded.WebSearch)
	}

	content, err := os.ReadFile(testUserSettingsPath(root, "user/1", "preferences.json"))
	if err != nil {
		t.Fatalf("读取偏好文件失败: %v", err)
	}
	if strings.Contains(string(content), apiKey) {
		t.Fatalf("AnySearch API key 不应写入偏好文件: %s", content)
	}
}

func TestServiceDoesNotReuseCredentialAcrossWebSearchProviders(t *testing.T) {
	root := t.TempDir()
	service := NewService(config.Config{WorkspacePath: filepath.Join(root, "workspace")})
	if _, err := service.Update(context.Background(), "user/1", UpdateRequest{
		WebSearch: &WebSearchSettings{Enabled: true, Provider: "anysearch"},
	}); err != nil {
		t.Fatalf("写入 AnySearch 配置失败: %v", err)
	}

	keyPath := testUserSettingsPath(root, "user/1", "web-search-api-key")
	credential := `{"provider":"tavily","api_key":"provider-scoped-key"}`
	if err := os.WriteFile(keyPath, []byte(credential), 0o600); err != nil {
		t.Fatalf("写入 provider 凭据失败: %v", err)
	}
	loaded, err := service.Get(context.Background(), "user/1")
	if err != nil {
		t.Fatalf("读取 AnySearch 配置失败: %v", err)
	}
	if loaded.WebSearch.APIKeyConfigured || loaded.WebSearchAPIKey() != "" || loaded.WebSearch.APIKeyMasked != "" {
		t.Fatalf("AnySearch 不应复用 Tavily 凭据: %+v", loaded.WebSearch)
	}

	if err := os.WriteFile(keyPath, []byte("provider-scoped-key\n"), 0o600); err != nil {
		t.Fatalf("写入旧格式凭据失败: %v", err)
	}
	loaded, err = service.Get(context.Background(), "user/1")
	if err != nil {
		t.Fatalf("读取旧格式凭据失败: %v", err)
	}
	if loaded.WebSearch.APIKeyConfigured || loaded.WebSearchAPIKey() != "" {
		t.Fatalf("旧格式凭据不应被读取: %+v", loaded.WebSearch)
	}
}

func TestServiceClearsWebSearchAPIKeyWhenProviderChanges(t *testing.T) {
	root := t.TempDir()
	service := NewService(config.Config{WorkspacePath: filepath.Join(root, "workspace")})
	apiKey := "brave-key"
	if _, err := service.Update(context.Background(), "user/1", UpdateRequest{
		WebSearch:       &WebSearchSettings{Enabled: true, Provider: "brave"},
		WebSearchAPIKey: &apiKey,
	}); err != nil {
		t.Fatalf("写入 Brave 配置失败: %v", err)
	}

	prefs, err := service.Update(context.Background(), "user/1", UpdateRequest{
		WebSearch: &WebSearchSettings{Provider: "tavily"},
	})
	if err != nil {
		t.Fatalf("切换 Tavily 失败: %v", err)
	}
	if prefs.WebSearch.APIKeyConfigured || prefs.WebSearchAPIKey() != "" {
		t.Fatalf("切换 provider 后不应复用旧 API key: %+v", prefs.WebSearch)
	}
	keyPath := testUserSettingsPath(root, "user/1", "web-search-api-key")
	if _, err := os.Stat(keyPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("切换 provider 后应删除旧 API key: %v", err)
	}
}

func TestServiceRejectsIncompleteWebSearchSettings(t *testing.T) {
	service := NewService(config.Config{WorkspacePath: t.TempDir()})
	tests := []WebSearchSettings{
		{Enabled: true, Provider: "brave"},
		{Enabled: true, Provider: "searxng"},
		{Enabled: true, Provider: "unsupported"},
	}
	for _, settings := range tests {
		if _, err := service.Update(context.Background(), "user/1", UpdateRequest{WebSearch: &settings}); err == nil {
			t.Fatalf("无效 WebSearch 配置应被拒绝: %+v", settings)
		}
	}
}

func TestServiceUpdateNormalizesRuntimeKindAlias(t *testing.T) {
	root := t.TempDir()
	service := NewService(config.Config{WorkspacePath: filepath.Join(root, "workspace")})

	prefs, err := service.Update(context.Background(), "user/1", UpdateRequest{
		AgentRuntimeKind: stringPointer("NXS"),
	})
	if err != nil {
		t.Fatalf("更新 runtime 偏好失败: %v", err)
	}
	if prefs.AgentRuntimeKind != "nxs" {
		t.Fatalf("runtime 别名未归一化: %+v", prefs)
	}
}

func TestServiceUpdatePersistsInterruptDefaultDeliveryPolicy(t *testing.T) {
	root := t.TempDir()
	service := NewService(config.Config{WorkspacePath: filepath.Join(root, "workspace")})

	prefs, err := service.Update(context.Background(), "user/1", UpdateRequest{
		ChatDefaultDeliveryPolicy: policyPointer(protocol.ChatDeliveryPolicyInterrupt),
	})
	if err != nil {
		t.Fatalf("更新偏好失败: %v", err)
	}
	if prefs.ChatDefaultDeliveryPolicy != protocol.ChatDeliveryPolicyInterrupt {
		t.Fatalf("打断默认行为未持久化: %+v", prefs)
	}
}

func policyPointer(value protocol.ChatDeliveryPolicy) *protocol.ChatDeliveryPolicy {
	return &value
}

func stringPointer(value string) *string {
	return &value
}

func boolPointer(value bool) *bool {
	return &value
}

func testUserSettingsPath(root string, ownerUserID string, fileName string) string {
	return filepath.Join(
		root,
		"workspace",
		appfs.UserPathSegment(ownerUserID),
		"workspace",
		".settings",
		fileName,
	)
}
