package provider

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	ccSwitchClaudeToken = "sk-ant-ccswitch-secret"
	ccSwitchCodexToken  = "sk-codex-ccswitch-secret"
)

func TestPreviewCCSwitchMapsCurrentProvidersWithoutExposingSecrets(t *testing.T) {
	service, _ := newTestService(t)
	service.desktopMode = true
	configDir := newCCSwitchFixture(t)

	preview, err := service.PreviewCCSwitch(context.Background(), CCSwitchPreviewInput{
		ConfigDir:   configDir,
		RuntimeKind: "nxs",
	})
	if err != nil {
		t.Fatalf("预览 CC Switch 失败: %v", err)
	}
	if !preview.Detected || preview.SchemaVersion != 16 {
		t.Fatalf("应检测到 schema v16: %+v", preview)
	}
	if preview.ProviderCount != 3 || preview.ReadyCount != 2 || preview.ModelCount != 4 {
		t.Fatalf("预览统计不正确: %+v", preview)
	}
	if preview.RecommendedSource != "codex:codex-main" {
		t.Fatalf("nxs 应优先推荐当前 Codex 服务: %+v", preview)
	}
	claude := ccSwitchPreviewProvider(t, preview, "claude:claude-main")
	if !claude.Current || !claude.CanSync || claude.APIFormat != APIFormatAnthropicMessages {
		t.Fatalf("Claude 映射不正确: %+v", claude)
	}
	if len(claude.Models) != 1 || claude.Models[0].ContextWindow == nil || *claude.Models[0].ContextWindow != 1_000_000 {
		t.Fatalf("Claude [1M] 模型未正确映射: %+v", claude.Models)
	}
	backup := ccSwitchPreviewProvider(t, preview, "claude:claude-managed")
	if backup.Current || backup.CanSync || backup.Status != ccSwitchStatusUnsupported {
		t.Fatalf("settings.json 应覆盖数据库 current，托管账号应不可同步: %+v", backup)
	}
	codex := ccSwitchPreviewProvider(t, preview, "codex:codex-main")
	if !codex.Current || !codex.CanSync || codex.APIFormat != APIFormatResponses || len(codex.Models) != 2 {
		t.Fatalf("Codex 映射不正确: %+v", codex)
	}

	payload, err := json.Marshal(preview)
	if err != nil {
		t.Fatalf("编码预览失败: %v", err)
	}
	for _, secret := range []string{ccSwitchClaudeToken, ccSwitchCodexToken} {
		if strings.Contains(string(payload), secret) {
			t.Fatalf("预览泄露了凭据: %s", payload)
		}
	}
}

func TestBuildCCSwitchCandidateSupportsKimiForCoding(t *testing.T) {
	candidate := buildCCSwitchCandidate(ccSwitchProviderRow{
		id:        "kimi-for-coding",
		appType:   ccSwitchAppClaude,
		name:      "Kimi For Coding",
		isCurrent: true,
		settingsConfig: mustJSON(t, map[string]any{"env": map[string]any{
			"ANTHROPIC_AUTH_TOKEN": "sk-kimi-test",
			"ANTHROPIC_BASE_URL":   "https://api.kimi.com/coding/",
			"ANTHROPIC_MODEL":      "kimi-for-coding",
		}}),
		meta: `{"apiFormat":"anthropic"}`,
	})

	preview := candidate.preview
	if !preview.CanSync || !preview.Current || preview.APIFormat != APIFormatAnthropicMessages {
		t.Fatalf("Kimi For Coding 应可按 Anthropic 协议同步: %+v", preview)
	}
	if preview.BaseURL != "https://api.kimi.com/coding/" || preview.DefaultModel != "kimi-for-coding" {
		t.Fatalf("Kimi 服务地址或默认模型映射不正确: %+v", preview)
	}
	if len(preview.Models) != 1 || preview.Models[0].ModelID != "kimi-for-coding" {
		t.Fatalf("Kimi 模型映射不正确: %+v", preview.Models)
	}
	if !ccSwitchRuntimeSupports(preview.APIFormat, "claude") || !ccSwitchRuntimeSupports(preview.APIFormat, "nxs") {
		t.Fatalf("Kimi Anthropic 配置应同时兼容 Claude Code 与 nxs: %+v", preview.RuntimeSupport)
	}
}

func TestSyncCCSwitchIsIdempotentAndSetsRuntimeCompatibleDefault(t *testing.T) {
	service, database := newTestService(t)
	service.desktopMode = true
	configDir := newCCSwitchFixture(t)
	input := CCSwitchSyncInput{
		ConfigDir:   configDir,
		SourceKeys:  []string{"claude:claude-main", "codex:codex-main"},
		SetDefault:  true,
		RuntimeKind: "nxs",
	}

	first, err := service.SyncCCSwitch(context.Background(), input)
	if err != nil {
		t.Fatalf("首次同步失败: %v", err)
	}
	if first.Created != 2 || first.Updated != 0 || first.ModelCount != 3 {
		t.Fatalf("首次同步统计不正确: %+v", first)
	}
	if first.DefaultSelection == nil || !strings.Contains(first.DefaultSelection.Provider, "ccswitch-codex-") || first.DefaultSelection.Model != "gpt-5.4" {
		t.Fatalf("nxs 默认模型应来自当前 Codex 服务: %+v", first.DefaultSelection)
	}

	second, err := service.SyncCCSwitch(context.Background(), input)
	if err != nil {
		t.Fatalf("重复同步失败: %v", err)
	}
	if second.Created != 0 || second.Updated != 2 {
		t.Fatalf("重复同步应更新原记录: %+v", second)
	}
	var providerCount int
	if err = database.QueryRow(`SELECT COUNT(*) FROM provider WHERE provider LIKE 'ccswitch-%'`).Scan(&providerCount); err != nil {
		t.Fatalf("统计 Provider 失败: %v", err)
	}
	if providerCount != 2 {
		t.Fatalf("重复同步不应创建 Provider，实际=%d", providerCount)
	}
	var modelCount int
	if err = database.QueryRow(`
SELECT COUNT(*)
FROM provider_models model
JOIN provider item ON item.id = model.provider_id
WHERE item.provider LIKE 'ccswitch-%'`).Scan(&modelCount); err != nil {
		t.Fatalf("统计模型失败: %v", err)
	}
	if modelCount != 3 {
		t.Fatalf("重复同步不应创建重复模型，实际=%d", modelCount)
	}

	options, err := service.ListOptionsForRuntime(context.Background(), "nxs")
	if err != nil {
		t.Fatalf("读取 nxs 选项失败: %v", err)
	}
	if options.DefaultSelection == nil || options.DefaultSelection.Model != "gpt-5.4" {
		t.Fatalf("默认模型未生效: %+v", options.DefaultSelection)
	}
	claudeOptions, err := service.ListOptionsForRuntime(context.Background(), "claude")
	if err != nil {
		t.Fatalf("读取 Claude runtime 选项失败: %v", err)
	}
	if len(claudeOptions.Items) != 1 || !strings.Contains(claudeOptions.Items[0].Provider, "ccswitch-claude-") {
		t.Fatalf("Claude runtime 不应暴露 Responses Provider: %+v", claudeOptions.Items)
	}

	preview, err := service.PreviewCCSwitch(context.Background(), CCSwitchPreviewInput{
		ConfigDir:   configDir,
		RuntimeKind: "nxs",
	})
	if err != nil {
		t.Fatalf("同步后预览失败: %v", err)
	}
	if !ccSwitchPreviewProvider(t, preview, "claude:claude-main").Existing ||
		!ccSwitchPreviewProvider(t, preview, "codex:codex-main").Existing {
		t.Fatalf("同步后应标记为更新现有配置: %+v", preview.Providers)
	}
}

func TestPreviewCCSwitchRejectsServerMode(t *testing.T) {
	service, _ := newTestService(t)
	_, err := service.PreviewCCSwitch(context.Background(), CCSwitchPreviewInput{})
	if err == nil || !strings.Contains(err.Error(), "桌面版") {
		t.Fatalf("服务端模式应拒绝本地读取: %v", err)
	}
}

func newCCSwitchFixture(t *testing.T) string {
	t.Helper()
	configDir := t.TempDir()
	databasePath := filepath.Join(configDir, ccSwitchDatabaseName)
	database, err := sql.Open("sqlite", databasePath)
	if err != nil {
		t.Fatalf("创建 CC Switch fixture 失败: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	if _, err = database.Exec(`
CREATE TABLE providers (
    id TEXT NOT NULL,
    app_type TEXT NOT NULL,
    name TEXT NOT NULL,
    settings_config TEXT NOT NULL,
    meta TEXT NOT NULL DEFAULT '{}',
    is_current BOOLEAN NOT NULL DEFAULT 0,
    PRIMARY KEY (id, app_type)
);
PRAGMA user_version = 16;`); err != nil {
		t.Fatalf("创建 CC Switch 表失败: %v", err)
	}
	insertCCSwitchProvider(t, database, ccSwitchProviderRow{
		id:      "claude-main",
		appType: ccSwitchAppClaude,
		name:    "Claude Gateway",
		settingsConfig: mustJSON(t, map[string]any{"env": map[string]any{
			"ANTHROPIC_AUTH_TOKEN":           ccSwitchClaudeToken,
			"ANTHROPIC_BASE_URL":             "https://claude.example.com",
			"ANTHROPIC_MODEL":                "claude-sonnet-4-5 [1M]",
			"ANTHROPIC_MODEL_NAME":           "Sonnet Long",
			"ANTHROPIC_DEFAULT_SONNET_MODEL": "claude-sonnet-4-5 [1M]",
		}}),
		meta:      `{"apiFormat":"anthropic"}`,
		isCurrent: false,
	})
	insertCCSwitchProvider(t, database, ccSwitchProviderRow{
		id:      "claude-managed",
		appType: ccSwitchAppClaude,
		name:    "Managed Claude",
		settingsConfig: mustJSON(t, map[string]any{"env": map[string]any{
			"ANTHROPIC_AUTH_TOKEN": "PROXY_MANAGED",
			"ANTHROPIC_BASE_URL":   "https://managed.example.com",
			"ANTHROPIC_MODEL":      "managed-model",
		}}),
		meta:      `{"providerType":"github_copilot"}`,
		isCurrent: true,
	})
	insertCCSwitchProvider(t, database, ccSwitchProviderRow{
		id:      "codex-main",
		appType: ccSwitchAppCodex,
		name:    "Codex Gateway",
		settingsConfig: mustJSON(t, map[string]any{
			"auth": map[string]any{"OPENAI_API_KEY": ccSwitchCodexToken},
			"config": `model_provider = "gateway"
model = "gpt-5.4"

[model_providers.gateway]
base_url = "https://codex.example.com/v1"
wire_api = "responses"
`,
			"modelCatalog": map[string]any{"models": []map[string]any{
				{"model": "gpt-5.4", "displayName": "GPT 5.4", "contextWindow": 272000, "inputModalities": []string{"text", "image"}},
				{"model": "gpt-5.4-mini", "displayName": "GPT 5.4 Mini", "contextWindow": "128000"},
			}},
		}),
		meta:      `{}`,
		isCurrent: true,
	})
	insertCCSwitchProvider(t, database, ccSwitchProviderRow{
		id:             "codex-official",
		appType:        ccSwitchAppCodex,
		name:           "OpenAI Official",
		settingsConfig: mustJSON(t, map[string]any{"config": `model = "gpt-placeholder"`}),
		meta:           `{}`,
		isCurrent:      false,
	})
	settings := mustJSON(t, map[string]any{
		"currentProviderClaude": "claude-main",
	})
	if err = os.WriteFile(filepath.Join(configDir, ccSwitchSettingsName), []byte(settings), 0o600); err != nil {
		t.Fatalf("写入 CC Switch settings fixture 失败: %v", err)
	}
	return configDir
}

func insertCCSwitchProvider(t *testing.T, database *sql.DB, item ccSwitchProviderRow) {
	t.Helper()
	if _, err := database.Exec(`
INSERT INTO providers (id, app_type, name, settings_config, meta, is_current)
VALUES (?, ?, ?, ?, ?, ?)`, item.id, item.appType, item.name, item.settingsConfig, item.meta, item.isCurrent); err != nil {
		t.Fatalf("插入 CC Switch Provider fixture 失败: %v", err)
	}
}

func mustJSON(t *testing.T, value any) string {
	t.Helper()
	payload, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("编码 fixture JSON 失败: %v", err)
	}
	return string(payload)
}

func ccSwitchPreviewProvider(t *testing.T, preview *CCSwitchPreview, sourceKey string) *CCSwitchProviderPreview {
	t.Helper()
	for index := range preview.Providers {
		if preview.Providers[index].SourceKey == sourceKey {
			return &preview.Providers[index]
		}
	}
	t.Fatalf("未找到 CC Switch Provider: %s", sourceKey)
	return nil
}
