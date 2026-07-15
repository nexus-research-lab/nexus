package preferences

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
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
	if prefs.WebSearch.Provider != "brave" {
		t.Fatalf("WebSearch 默认 provider 应为 brave: %+v", prefs.WebSearch)
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
	if normalized.WebSearch.Provider != "brave" {
		t.Fatalf("空偏好归一化后 WebSearch provider 应为 brave: %+v", normalized.WebSearch)
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
	if _, statErr := os.Stat(filepath.Join(root, "workspace", "user_1", ".settings", "preferences.json")); statErr != nil {
		t.Fatalf("偏好文件未写入安全路径: %v", statErr)
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
	preferencesPath := filepath.Join(root, "workspace", "user_1", ".settings", "preferences.json")
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
	keyPath := filepath.Join(root, "workspace", "user_1", ".settings", "web-search-api-key")
	if info, err := os.Stat(keyPath); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("API key 文件权限不正确: info=%v err=%v", info, err)
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

func TestServicePersistsAnySearchSettings(t *testing.T) {
	root := t.TempDir()
	service := NewService(config.Config{WorkspacePath: filepath.Join(root, "workspace")})

	prefs, err := service.Update(context.Background(), "user/1", UpdateRequest{
		WebSearch: &WebSearchSettings{
			Enabled:  true,
			Provider: "anysearch",
			AnySearch: AnySearchSettings{
				Domain:       " code ",
				Tag:          " code.doc ",
				ContentTypes: []string{"web", " web ", "news"},
				Params:       map[string]any{"language": "go"},
			},
		},
	})
	if err != nil {
		t.Fatalf("更新 AnySearch 偏好失败: %v", err)
	}
	if prefs.WebSearch.Provider != "anysearch" || prefs.WebSearch.AnySearch.Domain != "code" || prefs.WebSearch.AnySearch.Tag != "code.doc" {
		t.Fatalf("AnySearch 基础配置未归一化: %+v", prefs.WebSearch)
	}
	if len(prefs.WebSearch.AnySearch.ContentTypes) != 2 || prefs.WebSearch.AnySearch.ContentTypes[0] != "web" || prefs.WebSearch.AnySearch.ContentTypes[1] != "news" {
		t.Fatalf("AnySearch content_types 未归一化: %+v", prefs.WebSearch.AnySearch.ContentTypes)
	}
	loaded, err := service.Get(context.Background(), "user/1")
	if err != nil {
		t.Fatalf("读取 AnySearch 偏好失败: %v", err)
	}
	if loaded.WebSearch.AnySearch.Params["language"] != "go" {
		t.Fatalf("AnySearch params 未持久化: %+v", loaded.WebSearch.AnySearch.Params)
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
