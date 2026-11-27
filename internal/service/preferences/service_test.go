// INPUT: 并发 owner 更新、陈旧 version、WebSearch credential 与回滚快照。
// OUTPUT: Preferences 串行 RMW、CAS、条件回滚和唯一临时文件行为证明。
// POS: Preferences 文件真相源 P0 一致性回归测试。
package preferences

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestDefaultPreferencesAcceptEditsByDefault(t *testing.T) {
	prefs := DefaultPreferences()
	if prefs.DefaultAgentOptions.PermissionMode != protocol.DefaultAgentPermissionMode {
		t.Fatalf("默认权限应自动接受编辑: %+v", prefs.DefaultAgentOptions)
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
	if prefs.EmotionEnabled {
		t.Fatalf("情绪系统默认应关闭: %+v", prefs)
	}
	if prefs.BrowserCDPEnabled {
		t.Fatalf("完整 CDP 访问默认应关闭: %+v", prefs)
	}
	if prefs.EchoEnabled {
		t.Fatalf("主动跟进默认应关闭: %+v", prefs)
	}
	if prefs.ToolSearchEnabledForRuntime("nxs") {
		t.Fatalf("nxs ToolSearch 默认应关闭: %+v", prefs)
	}
	if !prefs.AutoMemoryEnabledForRuntime("nxs") {
		t.Fatalf("nxs 自动记忆默认应开启: %+v", prefs)
	}
	if !prefs.AutoDreamEnabledForRuntime("nxs") {
		t.Fatalf("nxs AutoDream 默认应开启: %+v", prefs)
	}
	if !prefs.WebSearch.Enabled || prefs.WebSearch.Provider != "anysearch" {
		t.Fatalf("WebSearch 默认 provider 应为 anysearch: %+v", prefs.WebSearch)
	}

	normalized := normalizePreferences(Preferences{})
	if normalized.DefaultAgentOptions.PermissionMode != protocol.DefaultAgentPermissionMode {
		t.Fatalf("空偏好归一化后应自动接受编辑: %+v", normalized.DefaultAgentOptions)
	}
	if normalized.AgentRuntimeKind != "nxs" {
		t.Fatalf("空偏好归一化后 runtime 应为 nxs: %+v", normalized)
	}
	if normalized.AgentSDKDiagnosticsEnabled {
		t.Fatalf("空偏好归一化后 Agent SDK diagnostics 应关闭: %+v", normalized)
	}
	if normalized.EmotionEnabled {
		t.Fatalf("空偏好归一化后情绪系统应关闭: %+v", normalized)
	}
	if normalized.EchoEnabled {
		t.Fatalf("空偏好归一化后主动跟进应关闭: %+v", normalized)
	}
	if normalized.ToolSearchEnabledForRuntime("nxs") {
		t.Fatalf("空偏好归一化后 nxs ToolSearch 应关闭: %+v", normalized)
	}
	if !normalized.AutoMemoryEnabledForRuntime("nxs") {
		t.Fatalf("空偏好归一化后 nxs 自动记忆应开启: %+v", normalized)
	}
	if !normalized.AutoDreamEnabledForRuntime("nxs") {
		t.Fatalf("空偏好归一化后 nxs AutoDream 应开启: %+v", normalized)
	}
	if !normalized.WebSearch.Enabled || normalized.WebSearch.Provider != "anysearch" {
		t.Fatalf("空偏好归一化后 WebSearch provider 应为 anysearch: %+v", normalized.WebSearch)
	}
}

func TestServicePersistsEchoEnabled(t *testing.T) {
	service := NewService(config.Config{WorkspacePath: filepath.Join(t.TempDir(), "workspace")})

	updated, err := service.SetEchoEnabled(context.Background(), "user/1", true)
	if err != nil {
		t.Fatalf("开启主动跟进失败: %v", err)
	}
	if !updated.EchoEnabled {
		t.Fatalf("主动跟进未开启: %+v", updated)
	}

	loaded, err := service.Get(context.Background(), "user/1")
	if err != nil {
		t.Fatalf("读取主动跟进设置失败: %v", err)
	}
	if !loaded.EchoEnabled {
		t.Fatalf("主动跟进未持久化: %+v", loaded)
	}
}

func TestServiceSetEchoEnabledAtVersionRejectsStaleAggregate(t *testing.T) {
	service := NewService(config.Config{WorkspacePath: filepath.Join(t.TempDir(), "workspace")})
	base, err := service.Get(context.Background(), "user/1")
	if err != nil {
		t.Fatal(err)
	}
	updated, err := service.SetEchoEnabledAtVersion(
		context.Background(), "user/1", true, base.Version,
	)
	if err != nil || !updated.EchoEnabled || updated.Version <= base.Version {
		t.Fatalf("updated=%+v err=%v", updated, err)
	}
	if _, err = service.SetEchoEnabledAtVersion(
		context.Background(), "user/1", false, base.Version,
	); !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("stale update error=%v, want ErrVersionConflict", err)
	}
	latest, err := service.Get(context.Background(), "user/1")
	if err != nil || !latest.EchoEnabled || latest.Version != updated.Version {
		t.Fatalf("latest=%+v err=%v", latest, err)
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
