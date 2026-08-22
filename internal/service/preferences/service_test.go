// INPUT: 并发 owner 更新、陈旧 version、WebSearch credential 与回滚快照。
// OUTPUT: Preferences 串行 RMW、CAS、条件回滚和唯一临时文件行为证明。
// POS: Preferences 文件真相源 P0 一致性回归测试。
package preferences

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

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

func TestServiceUpdatePersistsUserPreferences(t *testing.T) {
	root := t.TempDir()
	service := NewService(config.Config{WorkspacePath: filepath.Join(root, "workspace")})

	prefs, err := service.Update(context.Background(), "user/1", UpdateRequest{
		ChatDefaultDeliveryPolicy:  policyPointer(protocol.ChatDeliveryPolicyQueue),
		AgentRuntimeKind:           stringPointer("nxs"),
		AgentSDKDiagnosticsEnabled: boolPointer(true),
		EmotionEnabled:             boolPointer(true),
		BrowserCDPEnabled:          boolPointer(true),
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
	if !prefs.EmotionEnabled {
		t.Fatalf("情绪系统偏好未持久化: %+v", prefs)
	}
	if !prefs.BrowserCDPEnabled {
		t.Fatalf("完整 CDP 访问偏好未持久化: %+v", prefs)
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
		!loaded.EmotionEnabled ||
		!loaded.BrowserCDPEnabled ||
		!loaded.ToolSearchEnabledForRuntime("nxs") ||
		loaded.DefaultAgentOptions.PermissionMode != "default" {
		t.Fatalf("读取结果不正确: %+v", loaded)
	}
	if loaded.DefaultImageModelSelection.Model != "image-model" || loaded.DefaultVisionModelSelection.Model != "vision-model" || loaded.DefaultBackgroundModelSelection.Model != "background-model" {
		t.Fatalf("读取默认模型选择不正确: %+v", loaded)
	}
	emotionDisabled := false
	disabled, err := service.Update(context.Background(), "user/1", UpdateRequest{
		EmotionEnabled: &emotionDisabled,
	})
	if err != nil {
		t.Fatalf("关闭情绪系统失败: %v", err)
	}
	if disabled.EmotionEnabled {
		t.Fatalf("情绪系统关闭状态未生效: %+v", disabled)
	}
	reloaded, err := service.Get(context.Background(), "user/1")
	if err != nil {
		t.Fatalf("重新读取关闭后的偏好失败: %v", err)
	}
	if reloaded.EmotionEnabled {
		t.Fatalf("情绪系统关闭状态未持久化: %+v", reloaded)
	}
	preferencesPath := testUserSettingsPath(root, "user/1", "preferences.json")
	info, statErr := os.Stat(preferencesPath)
	if statErr != nil {
		t.Fatalf("偏好文件未写入安全路径: %v", statErr)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("偏好文件权限不正确: got=%#o want=%#o", info.Mode().Perm(), 0o600)
	}
	settingsInfo, statErr := os.Stat(filepath.Dir(preferencesPath))
	if statErr != nil {
		t.Fatalf("偏好目录权限不正确: info=%v err=%v", settingsInfo, statErr)
	}
	if runtime.GOOS != "windows" && settingsInfo.Mode().Perm() != 0o700 {
		t.Fatalf("偏好目录权限不正确: got=%#o want=%#o", settingsInfo.Mode().Perm(), 0o700)
	}
}

func TestServiceStoresWebSearchAPIKeySeparately(t *testing.T) {
	root := t.TempDir()
	service := NewService(config.Config{WorkspacePath: filepath.Join(root, "workspace")})
	apiKey := "secret-search-key"
	updated, err := service.Update(context.Background(), "user/1", UpdateRequest{
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
	info, err := os.Stat(keyPath)
	if err != nil {
		t.Fatalf("API key 文件权限不正确: info=%v err=%v", info, err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("API key 文件权限不正确: got=%#o want=%#o", info.Mode().Perm(), 0o600)
	}
	credentialContent, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatalf("读取 WebSearch 凭据文件失败: %v", err)
	}
	credential := decodeWebSearchCredentialBundle(credentialContent).
		credentialForVersion(updated.Version)
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

func TestServiceWebSearchCredentialCommitSurvivesEitherCrashPoint(t *testing.T) {
	root := t.TempDir()
	cfg := config.Config{WorkspacePath: filepath.Join(root, "workspace")}
	service := NewService(cfg)
	const ownerID = "owner-credential-crash"
	oldKey := "old-brave-key"
	current, err := service.Update(context.Background(), ownerID, UpdateRequest{
		WebSearch:       &WebSearchSettings{Enabled: true, Provider: "brave"},
		WebSearchAPIKey: &oldKey,
	})
	if err != nil {
		t.Fatal(err)
	}
	next := current
	next.Version++
	next.UpdatedAt = nowRFC3339()
	nextKey := "next-brave-key"
	next.WebSearch = next.WebSearch.WithWebSearchAPIKey(nextKey)

	// 模拟进程在凭据双代写入后、Preferences 发布前崩溃。
	if err = service.writeWebSearchCredentialBundleConfined(
		ownerID,
		credentialBundleForTransition(current, next),
	); err != nil {
		t.Fatal(err)
	}
	beforePublish, err := NewService(cfg).Get(context.Background(), ownerID)
	if err != nil {
		t.Fatal(err)
	}
	if beforePublish.Version != current.Version ||
		beforePublish.WebSearchAPIKey() != oldKey {
		t.Fatalf("Preferences 发布前必须继续读取旧代: %+v", beforePublish)
	}

	// 模拟 Preferences 已发布、旧凭据代尚未清理时再次崩溃。
	if err = service.writePreferencesConfined(ownerID, next); err != nil {
		t.Fatal(err)
	}
	afterPublish, err := NewService(cfg).Get(context.Background(), ownerID)
	if err != nil {
		t.Fatal(err)
	}
	if afterPublish.Version != next.Version ||
		afterPublish.WebSearchAPIKey() != nextKey {
		t.Fatalf("Preferences 发布后必须只读取新代: %+v", afterPublish)
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

func TestServiceSerializesOwnerReadModifyWrite(t *testing.T) {
	service := NewService(config.Config{WorkspacePath: t.TempDir()})
	const ownerID = "owner-serialized-rmw"

	firstEntered := make(chan struct{})
	releaseFirst := make(chan struct{})
	firstDone := make(chan error, 1)
	go func() {
		_, err := service.UpdatePrepared(
			context.Background(),
			ownerID,
			func(Preferences) (UpdateRequest, error) {
				close(firstEntered)
				<-releaseFirst
				enabled := true
				return UpdateRequest{AgentSDKDiagnosticsEnabled: &enabled}, nil
			},
		)
		firstDone <- err
	}()
	<-firstEntered

	secondStarted := make(chan struct{})
	secondDone := make(chan error, 1)
	go func() {
		close(secondStarted)
		policy := protocol.ChatDeliveryPolicyInterrupt
		_, err := service.Update(
			context.Background(),
			ownerID,
			UpdateRequest{ChatDefaultDeliveryPolicy: &policy},
		)
		secondDone <- err
	}()
	<-secondStarted
	select {
	case err := <-secondDone:
		t.Fatalf("第二项 owner 更新未等待首个 RMW 完成: %v", err)
	case <-time.After(25 * time.Millisecond):
	}
	close(releaseFirst)
	if err := <-firstDone; err != nil {
		t.Fatalf("首项 owner 更新失败: %v", err)
	}
	if err := <-secondDone; err != nil {
		t.Fatalf("第二项 owner 更新失败: %v", err)
	}

	stored, err := service.Get(context.Background(), ownerID)
	if err != nil {
		t.Fatal(err)
	}
	if !stored.AgentSDKDiagnosticsEnabled ||
		stored.ChatDefaultDeliveryPolicy != protocol.ChatDeliveryPolicyInterrupt {
		t.Fatalf("串行 RMW 丢失局部更新: %+v", stored)
	}
	if stored.Version != 3 {
		t.Fatalf("串行写入后的 version = %d, want 3", stored.Version)
	}
}

func TestServiceUpdateAtVersionRejectsStaleWrite(t *testing.T) {
	cfg := config.Config{WorkspacePath: t.TempDir()}
	service := NewService(cfg)
	const ownerID = "owner-preferences-cas"

	initial, err := service.Get(context.Background(), ownerID)
	if err != nil {
		t.Fatal(err)
	}
	enabled := true
	updated, err := service.UpdateAtVersion(
		context.Background(),
		ownerID,
		UpdateRequest{AgentSDKDiagnosticsEnabled: &enabled},
		initial.Version,
	)
	if err != nil {
		t.Fatalf("首个 CAS 更新失败: %v", err)
	}
	if updated.Version != initial.Version+1 {
		t.Fatalf("CAS version = %d, want %d", updated.Version, initial.Version+1)
	}

	policy := protocol.ChatDeliveryPolicyInterrupt
	_, err = service.UpdateAtVersion(
		context.Background(),
		ownerID,
		UpdateRequest{ChatDefaultDeliveryPolicy: &policy},
		initial.Version,
	)
	if !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("陈旧 CAS error = %v, want ErrVersionConflict", err)
	}
	stored, err := service.Get(context.Background(), ownerID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Version != updated.Version ||
		!stored.AgentSDKDiagnosticsEnabled ||
		stored.ChatDefaultDeliveryPolicy == protocol.ChatDeliveryPolicyInterrupt {
		t.Fatalf("陈旧 CAS 覆盖了当前值: %+v", stored)
	}
	restarted, err := NewService(cfg).Get(context.Background(), ownerID)
	if err != nil {
		t.Fatal(err)
	}
	if restarted.Version != updated.Version {
		t.Fatalf("Service 重建后 version = %d, want %d", restarted.Version, updated.Version)
	}
}

func TestServicePreparedCASMergesFromLockedLatestValue(t *testing.T) {
	service := NewService(config.Config{WorkspacePath: t.TempDir()})
	const ownerID = "owner-prepared-cas"

	enabled := true
	latest, err := service.Update(
		context.Background(),
		ownerID,
		UpdateRequest{AgentSDKDiagnosticsEnabled: &enabled},
	)
	if err != nil {
		t.Fatal(err)
	}
	policy := protocol.ChatDeliveryPolicyInterrupt
	updated, err := service.UpdatePreparedAtVersion(
		context.Background(),
		ownerID,
		latest.Version,
		func(current Preferences) (UpdateRequest, error) {
			if !current.AgentSDKDiagnosticsEnabled || current.Version != latest.Version {
				t.Fatalf("builder 未读取锁内最新 Preferences: %+v", current)
			}
			return UpdateRequest{ChatDefaultDeliveryPolicy: &policy}, nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !updated.AgentSDKDiagnosticsEnabled ||
		updated.ChatDefaultDeliveryPolicy != protocol.ChatDeliveryPolicyInterrupt ||
		updated.Version != latest.Version+1 {
		t.Fatalf("锁内 merge 丢失最新字段: %+v", updated)
	}
}

func TestServiceRestoreIfVersionPreservesLaterWriteAndCredential(t *testing.T) {
	root := t.TempDir()
	service := NewService(config.Config{WorkspacePath: filepath.Join(root, "workspace")})
	const ownerID = "owner-conditional-restore"

	initial, err := service.Get(context.Background(), ownerID)
	if err != nil {
		t.Fatal(err)
	}
	apiKey := "rollback-brave-key"
	webSearchUpdate, err := service.UpdateAtVersion(
		context.Background(),
		ownerID,
		UpdateRequest{
			WebSearch:       &WebSearchSettings{Enabled: true, Provider: "brave"},
			WebSearchAPIKey: &apiKey,
		},
		initial.Version,
	)
	if err != nil {
		t.Fatal(err)
	}
	enabled := true
	later, err := service.Update(
		context.Background(),
		ownerID,
		UpdateRequest{AgentSDKDiagnosticsEnabled: &enabled},
	)
	if err != nil {
		t.Fatal(err)
	}

	current, restored, err := service.RestoreIfVersion(
		context.Background(),
		ownerID,
		webSearchUpdate.Version,
		initial,
	)
	if err != nil {
		t.Fatal(err)
	}
	if restored {
		t.Fatal("条件回滚不应覆盖后续写入")
	}
	if current.Version != later.Version ||
		!current.AgentSDKDiagnosticsEnabled ||
		current.WebSearchAPIKey() != apiKey {
		t.Fatalf("跳过回滚后当前值被破坏: %+v", current)
	}

	restoredValue, restored, err := service.RestoreIfVersion(
		context.Background(),
		ownerID,
		later.Version,
		initial,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !restored || restoredValue.Version != later.Version+1 {
		t.Fatalf("无后续写入时应恢复并推进 version: restored=%v value=%+v", restored, restoredValue)
	}
	if restoredValue.AgentSDKDiagnosticsEnabled ||
		restoredValue.WebSearch.Provider != initial.WebSearch.Provider ||
		restoredValue.WebSearchAPIKey() != "" {
		t.Fatalf("恢复结果不等于旧配置: %+v", restoredValue)
	}
	keyPath := filepath.Join(root, "workspace", ownerID, ".settings", "web-search-api-key")
	if _, err = os.Stat(keyPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("恢复旧 WebSearch 配置后 credential 仍存在: %v", err)
	}
}

func TestServiceUsesUniqueTemporaryFiles(t *testing.T) {
	root := t.TempDir()
	service := NewService(config.Config{WorkspacePath: filepath.Join(root, "workspace")})
	const ownerID = "owner-unique-temp"
	settingsDir := filepath.Join(root, "workspace", ownerID, ".settings")
	if err := os.MkdirAll(settingsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	sentinels := []string{
		filepath.Join(settingsDir, "preferences.json.tmp"),
		filepath.Join(settingsDir, "web-search-api-key.tmp"),
	}
	for _, path := range sentinels {
		if err := os.WriteFile(path, []byte("sentinel"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	apiKey := "unique-temp-key"
	if _, err := service.Update(context.Background(), ownerID, UpdateRequest{
		WebSearch:       &WebSearchSettings{Enabled: true, Provider: "brave"},
		WebSearchAPIKey: &apiKey,
	}); err != nil {
		t.Fatal(err)
	}
	for _, path := range sentinels {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if string(content) != "sentinel" {
			t.Fatalf("固定临时路径被覆盖: %s = %q", path, content)
		}
	}
	matches, err := filepath.Glob(filepath.Join(settingsDir, ".*.tmp"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("唯一临时文件未清理: %v", matches)
	}
}

func TestServiceConcurrentPartialUpdatesKeepMonotonicVersion(t *testing.T) {
	service := NewService(config.Config{WorkspacePath: t.TempDir()})
	const ownerID = "owner-concurrent-version"
	const writes = 24

	var wait sync.WaitGroup
	errorsCh := make(chan error, writes)
	for index := 0; index < writes; index++ {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			enabled := index%2 == 0
			_, err := service.Update(
				context.Background(),
				ownerID,
				UpdateRequest{AgentSDKDiagnosticsEnabled: &enabled},
			)
			errorsCh <- err
		}(index)
	}
	wait.Wait()
	close(errorsCh)
	for err := range errorsCh {
		if err != nil {
			t.Fatalf("并发 Preferences 写入失败: %v", err)
		}
	}
	stored, err := service.Get(context.Background(), ownerID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Version != writes+1 {
		t.Fatalf("并发写入 version = %d, want %d", stored.Version, writes+1)
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
