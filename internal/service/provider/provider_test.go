// provider_test.go 覆盖 Provider 夹具、配置版本、HTTP、列表与可见性变更测试。
package provider

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/handler/handlertest"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	providerstore "github.com/nexus-research-lab/nexus/internal/storage/provider"
)

func newTestService(t *testing.T) (*Service, *sql.DB) {
	t.Helper()

	cfg := handlertest.NewConfig(t)
	handlertest.MigrateSQLite(t, cfg.DatabaseURL)
	db := handlertest.OpenSQLite(t, cfg.DatabaseURL)
	t.Cleanup(func() { _ = db.Close() })
	return NewServiceWithDB(cfg, db), db
}

func insertProviderUsageAgent(
	t *testing.T,
	db *sql.DB,
	agentID string,
	slug string,
	name string,
	displayName string,
	isMain bool,
	provider string,
	status string,
) {
	t.Helper()
	insertProviderUsageAgentForOwner(t, db, authctx.SystemUserID, agentID, slug, name, displayName, isMain, provider, status)
}

func insertProviderUsageAgentForOwner(
	t *testing.T,
	db *sql.DB,
	ownerUserID string,
	agentID string,
	slug string,
	name string,
	displayName string,
	isMain bool,
	provider string,
	status string,
) {
	t.Helper()
	_, err := db.Exec(`
INSERT INTO agents (
    id, slug, name, description, definition, status, workspace_path, owner_user_id, is_main
) VALUES (?, ?, ?, '', '', ?, ?, ?, ?)`,
		agentID,
		slug,
		name,
		status,
		"/tmp/"+slug,
		ownerUserID,
		isMain,
	)
	if err != nil {
		t.Fatalf("插入 agent 失败: %v", err)
	}
	_, err = db.Exec(`
INSERT INTO profiles (
    id, agent_id, display_name, headline, profile_markdown
) VALUES (?, ?, ?, '', '')`,
		"profile-"+agentID,
		agentID,
		displayName,
	)
	if err != nil {
		t.Fatalf("插入 profile 失败: %v", err)
	}
	_, err = db.Exec(`
INSERT INTO runtimes (
    id, agent_id, provider, permission_mode, allowed_tools_json, disallowed_tools_json,
    mcp_servers_json, setting_sources_json, runtime_version
) VALUES (?, ?, ?, '', '[]', '[]', '{}', '[]', 1)`,
		"runtime-"+agentID,
		agentID,
		provider,
	)
	if err != nil {
		t.Fatalf("插入 runtime 失败: %v", err)
	}
}

func providerTestContext(userID string, role string) context.Context {
	return authctx.WithPrincipal(context.Background(), &authctx.Principal{
		UserID:     userID,
		Username:   userID,
		Role:       role,
		AuthMethod: authctx.AuthMethodPassword,
	})
}

func stringPointer(value string) *string {
	return &value
}

type runtimeSelection struct {
	provider string
	model    string
	version  int64
}

func runtimeSelectionsByAgent(t *testing.T, db *sql.DB, agentIDs ...string) map[string]runtimeSelection {
	t.Helper()
	result := map[string]runtimeSelection{}
	for _, agentID := range agentIDs {
		row := db.QueryRow(
			`SELECT COALESCE(provider, ''), COALESCE(model, ''), runtime_version
			 FROM runtimes WHERE agent_id = ? LIMIT 1`,
			agentID,
		)
		var item runtimeSelection
		if err := row.Scan(&item.provider, &item.model, &item.version); err != nil {
			t.Fatalf("读取 runtime provider/model 失败: %v", err)
		}
		result[agentID] = item
	}
	return result
}

func TestProviderFetchModelsAdvancesVersionExactlyOnce(t *testing.T) {
	var requestCount int
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requestCount++
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"data":[{"id":"remote-model","name":"Remote Model"}]}`))
	}))
	t.Cleanup(server.Close)

	ctx := context.Background()
	service, _ := newTestService(t)
	created := createConfigurationVersionTestProvider(t, service, ctx, "provider-fetch", server.URL)
	result, err := service.FetchModelsAtVersion(
		ctx,
		created.Provider,
		created.ConfigurationVersion,
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.Count != 1 || len(result.Models) != 1 || result.Models[0].ModelID != "remote-model" {
		t.Fatalf("fetch result = %+v", result)
	}
	if result.Models[0].ContextWindow == nil || *result.Models[0].ContextWindow != defaultModelContextWindow ||
		result.Models[0].MaxOutputTokens == nil || *result.Models[0].MaxOutputTokens != defaultModelMaxOutputTokens {
		t.Fatalf("远端模型缺少卡片信息时未应用默认值: %+v", result.Models[0])
	}
	current, err := service.Get(ctx, created.Provider)
	if err != nil {
		t.Fatal(err)
	}
	if current.ConfigurationVersion != created.ConfigurationVersion+1 {
		t.Fatalf("fetch configuration_version = %d", current.ConfigurationVersion)
	}
	if _, err = service.FetchModelsAtVersion(
		ctx,
		created.Provider,
		created.ConfigurationVersion,
	); !errors.Is(err, ErrConfigurationVersionConflict) {
		t.Fatalf("stale fetch error = %v", err)
	}
	if requestCount != 2 {
		t.Fatalf("remote request count = %d, want 2", requestCount)
	}
}

func TestForceDeleteProviderRollsBackRuntimeReassignmentOnDeleteFailure(t *testing.T) {
	ctx := context.Background()
	service, db := newTestService(t)
	fallback := createConfigurationVersionTestProvider(t, service, ctx, "provider-rollback-fallback", "https://fallback.example.com")
	if _, err := service.UpdateModelAtVersion(
		ctx,
		fallback.Provider,
		"fallback-model",
		UpdateModelInput{Enabled: true, IsDefault: true},
		fallback.ConfigurationVersion,
	); err != nil {
		t.Fatal(err)
	}
	target := createConfigurationVersionTestProvider(t, service, ctx, "provider-rollback-target", "https://target.example.com")
	insertProviderUsageAgent(
		t,
		db,
		"agent-provider-rollback",
		"provider-rollback",
		"Provider Rollback",
		"",
		false,
		target.Provider,
		"active",
	)
	if _, err := db.Exec(`
CREATE TRIGGER reject_provider_delete
BEFORE DELETE ON provider
BEGIN
    SELECT RAISE(ABORT, 'forced provider delete failure');
END`); err != nil {
		t.Fatal(err)
	}

	if _, err := service.DeleteAtVersion(
		ctx,
		target.Provider,
		DeleteInput{Force: true},
		target.ConfigurationVersion,
	); err == nil {
		t.Fatal("force delete should fail through trigger")
	}
	if _, err := service.Get(ctx, target.Provider); err != nil {
		t.Fatalf("target provider was not rolled back: %v", err)
	}
	runtime := runtimeSelectionsByAgent(t, db, "agent-provider-rollback")["agent-provider-rollback"]
	if runtime.provider != target.Provider || runtime.version != 1 {
		t.Fatalf("runtime reassignment was not rolled back: %+v", runtime)
	}
}

func createConfigurationVersionTestProvider(
	t *testing.T,
	service *Service,
	ctx context.Context,
	provider string,
	baseURL string,
) *Record {
	t.Helper()
	record, err := service.Create(ctx, CreateInput{
		Provider:    provider,
		PresetKey:   presetCustom,
		APIFormat:   APIFormatResponses,
		DisplayName: provider,
		AuthToken:   "provider-test-token",
		BaseURL:     baseURL,
		ModelsPath:  "/models",
		Enabled:     true,
	})
	if err != nil {
		t.Fatal(err)
	}
	return record
}

func TestProviderTestRedactsSensitiveErrors(t *testing.T) {
	ctx := context.Background()
	service, _ := newTestService(t)
	var mu sync.Mutex
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		mu.Lock()
		requestCount++
		mu.Unlock()
		if request.URL.Path == "/models" {
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write([]byte(`{"data":[{"id":"model-1"}]}`))
			return
		}
		writer.WriteHeader(http.StatusUnauthorized)
		_, _ = writer.Write([]byte(`{"error":"Bearer secret-token Authorization x-api-key"}`))
	}))
	defer server.Close()

	record, err := service.Create(ctx, CreateInput{
		Provider:   "redact",
		PresetKey:  presetCustom,
		APIFormat:  APIFormatChatCompletions,
		AuthToken:  "secret-token",
		BaseURL:    server.URL,
		ModelsPath: "/models",
		Enabled:    true,
	})
	if err != nil {
		t.Fatalf("创建 provider 失败: %v", err)
	}
	result, err := service.TestProvider(ctx, record.Provider)
	if err != nil {
		t.Fatalf("TestProvider 不应返回 transport 错误: %v", err)
	}
	if result.Success {
		t.Fatalf("测试应失败: %+v", result)
	}
	for _, leaked := range []string{"secret-token", "Authorization", "x-api-key"} {
		if strings.Contains(result.Error, leaked) {
			t.Fatalf("错误信息泄漏敏感内容 %q: %s", leaked, result.Error)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if requestCount != 6 {
		t.Fatalf("Provider 测试应读取目录后独立验证五类可用协议: got=%d", requestCount)
	}
}

func TestBatchedModelReadsMatchPerProviderReads(t *testing.T) {
	ctx := context.Background()
	service, _ := newTestService(t)
	providerIDs := make([]string, 0, 2)
	for index, name := range []string{"image-a", "image-b"} {
		record, err := service.Create(ctx, CreateInput{
			ProviderKind: ProviderKindImageGeneration,
			Provider:     name,
			PresetKey:    presetCustom,
			APIFormat:    APIFormatModelScopeImageGeneration,
			AuthToken:    "image-key",
			BaseURL:      "https://api-inference.modelscope.cn/v1",
			Enabled:      true,
			DisplayName:  name,
		})
		if err != nil {
			t.Fatalf("创建 provider 失败: %v", err)
		}
		providerIDs = append(providerIDs, record.ID)
		now := service.now()
		models := make([]providerstore.ModelEntity, 0, 3)
		for _, modelID := range []string{"z-model", "a-model", "m-model"} {
			models = append(models, providerstore.ModelEntity{
				ID:                       service.idFactory("provider_model"),
				ProviderID:               record.ID,
				ModelID:                  fmt.Sprintf("%s-%d", modelID, index),
				DisplayName:              modelID,
				Category:                 "image",
				Enabled:                  modelID != "m-model",
				CapabilitiesAutoJSON:     "{}",
				CapabilitiesOverrideJSON: "{}",
				ProviderOptionsJSON:      "{}",
				LastSeenAt:               now,
				CreatedAt:                now,
				UpdatedAt:                now,
			})
		}
		if err := service.repository.UpsertModels(ctx, models); err != nil {
			t.Fatalf("写入模型失败: %v", err)
		}
	}

	batched, err := service.repository.ListModelsByProviderIDs(ctx, append(providerIDs, providerIDs[0], " "))
	if err != nil {
		t.Fatalf("批量读取失败: %v", err)
	}
	if len(batched) != len(providerIDs) {
		t.Fatalf("批量结果应按 Provider 去重分组: %d", len(batched))
	}
	for _, providerID := range providerIDs {
		single, err := service.repository.ListModelsByProviderID(ctx, providerID)
		if err != nil {
			t.Fatalf("单个读取失败: %v", err)
		}
		// 组内顺序必须与单个读取一致：enabled 优先，再按名称。
		if len(single) != 3 || !reflect.DeepEqual(batched[providerID], single) {
			t.Fatalf("批量与单个读取不一致:\n%+v\n%+v", batched[providerID], single)
		}
	}

	options, err := service.ListOptionsForRuntime(ctx, "nxs")
	if err != nil {
		t.Fatalf("读取 options 失败: %v", err)
	}
	if len(options.ImageItems) != 2 || len(options.ImageItems[0].Models) != 2 || len(options.ImageItems[1].Models) != 2 {
		t.Fatalf("图片选项应只含各自启用的模型: %+v", options.ImageItems)
	}
}

func TestSubscriptionDefaultModelBecomesNewUserFallback(t *testing.T) {
	service, _ := newTestService(t)
	adminCtx := providerTestContext("admin-user", authctx.RoleAdmin)
	newUserCtx := providerTestContext("new-user", authctx.RoleMember)

	first, err := service.CreatePublic(adminCtx, CreateInput{
		Provider:    "subscription-first",
		PresetKey:   presetCustom,
		APIFormat:   APIFormatAnthropicMessages,
		AuthToken:   "first-key",
		BaseURL:     "https://first.example.com",
		Enabled:     true,
		DisplayName: "First",
	})
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.CreatePublic(adminCtx, CreateInput{
		Provider:    "subscription-second",
		PresetKey:   presetCustom,
		APIFormat:   APIFormatAnthropicMessages,
		AuthToken:   "second-key",
		BaseURL:     "https://second.example.com",
		Enabled:     true,
		DisplayName: "Second",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.UpdatePublicModel(adminCtx, first.Provider, "first-model", UpdateModelInput{Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if _, err = service.UpdatePublicModel(adminCtx, second.Provider, "second-model", UpdateModelInput{Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if _, err = service.SetPublicDefaultModel(adminCtx, first.Provider, "first-model"); err != nil {
		t.Fatal(err)
	}
	if _, err = service.SetPublicDefaultModel(adminCtx, second.Provider, "second-model"); err != nil {
		t.Fatal(err)
	}

	config, err := service.ResolveRuntimeConfigForRuntime(newUserCtx, "", "", "nxs")
	if err != nil {
		t.Fatal(err)
	}
	if config.Provider != second.Provider || config.Model != "second-model" {
		t.Fatalf("new user fallback = %s/%s, want %s/second-model", config.Provider, config.Model, second.Provider)
	}
	records, err := service.ListPublic(adminCtx)
	if err != nil {
		t.Fatal(err)
	}
	for _, record := range records {
		for _, model := range record.Models {
			wantDefault := record.Provider == second.Provider && model.ModelID == "second-model"
			if model.IsDefault != wantDefault {
				t.Fatalf("unexpected subscription default: provider=%s model=%s default=%v", record.Provider, model.ModelID, model.IsDefault)
			}
		}
	}
}

func TestProviderPublicMutationRequiresAdminAndDeleteProtectsGlobalUsage(t *testing.T) {
	service, db := newTestService(t)
	adminCtx := providerTestContext("admin-user", authctx.RoleAdmin)
	memberCtx := providerTestContext("member-user", authctx.RoleMember)
	record, err := service.CreatePublic(adminCtx, CreateInput{
		Provider:    "public-guard",
		PresetKey:   presetCustom,
		APIFormat:   APIFormatAnthropicMessages,
		AuthToken:   "public-key",
		BaseURL:     "https://public.example.com",
		ModelsPath:  "/models",
		Enabled:     true,
		DisplayName: "Public Guard",
	})
	if err != nil {
		t.Fatalf("创建公共 provider 失败: %v", err)
	}
	if _, err = service.UpdateModel(adminCtx, record.Provider, "public-model", UpdateModelInput{Enabled: true, IsDefault: true}); err != nil {
		t.Fatalf("设置公共模型失败: %v", err)
	}
	if _, err = service.Update(memberCtx, record.Provider, UpdateInput{
		DisplayName: "Member Edit",
		AuthToken:   stringPointer("member-key"),
		BaseURL:     "https://member.example.com",
		Enabled:     true,
	}); err == nil || !strings.Contains(err.Error(), "只有管理员") {
		t.Fatalf("普通成员不应能维护公共 provider: %v", err)
	}

	insertProviderUsageAgentForOwner(t, db, "owner-a", "agent-public-a", "public-a", "Public A", "", false, record.Provider, "active")
	insertProviderUsageAgentForOwner(t, db, "owner-b", "agent-public-b", "public-b", "Public B", "", false, record.Provider, "active")
	if _, err = service.Delete(adminCtx, record.Provider, DeleteInput{}); err == nil || !strings.Contains(err.Error(), "2 个 Agent") {
		t.Fatalf("公共 provider 删除应按全局使用保护: %v", err)
	}
}

func TestReconcileDefaultAgentBindingsOnlyClearsMainBinding(t *testing.T) {
	service, db := newTestService(t)
	ctx := providerTestContext("owner-user", authctx.RoleMember)
	fallback, err := service.Create(ctx, CreateInput{
		Provider:    "fallback-provider",
		PresetKey:   presetCustom,
		APIFormat:   APIFormatAnthropicMessages,
		AuthToken:   "fallback-token",
		BaseURL:     "https://fallback.example.com",
		ModelsPath:  "/models",
		Enabled:     true,
		DisplayName: "Fallback",
	})
	if err != nil {
		t.Fatalf("创建 fallback provider 失败: %v", err)
	}
	if _, err = service.UpdateModel(ctx, fallback.Provider, "fallback-model", UpdateModelInput{Enabled: true, IsDefault: true}); err != nil {
		t.Fatalf("设置 fallback 默认模型失败: %v", err)
	}
	stale, err := service.Create(ctx, CreateInput{
		Provider:    "stale-provider",
		PresetKey:   presetCustom,
		APIFormat:   APIFormatAnthropicMessages,
		AuthToken:   "stale-token",
		BaseURL:     "https://stale.example.com",
		ModelsPath:  "/models",
		Enabled:     false,
		DisplayName: "Stale",
	})
	if err != nil {
		t.Fatalf("创建失效 provider 失败: %v", err)
	}
	insertProviderUsageAgentForOwner(t, db, "owner-user", "agent-main", "main", "Nexus", "", true, fallback.Provider, "active")
	insertProviderUsageAgentForOwner(t, db, "owner-user", "agent-stale", "stale", "Stale", "", false, stale.Provider, "active")
	insertProviderUsageAgentForOwner(t, db, "owner-user", "agent-valid", "valid", "Valid", "", false, fallback.Provider, "active")
	if _, err = db.Exec(`
		UPDATE runtimes
		SET model = CASE agent_id
			WHEN 'agent-main' THEN 'legacy-main-model'
			WHEN 'agent-stale' THEN 'stale-model'
			WHEN 'agent-valid' THEN 'fallback-model'
		END
		WHERE agent_id IN ('agent-main', 'agent-stale', 'agent-valid')`); err != nil {
		t.Fatalf("写入显式模型绑定失败: %v", err)
	}

	repaired, err := service.ReconcileDefaultAgentBindings(ctx, DefaultAgentSelection{
		Provider:    fallback.Provider,
		Model:       "fallback-model",
		RuntimeKind: "claude",
	})
	if err != nil {
		t.Fatalf("修复历史绑定失败: %v", err)
	}
	if repaired != 1 {
		t.Fatalf("只应清理 Nexus 主智能体的历史显式绑定: got=%d", repaired)
	}
	runtimes := runtimeSelectionsByAgent(t, db, "agent-main", "agent-stale", "agent-valid")
	if runtimes["agent-main"].provider != "" || runtimes["agent-main"].model != "" {
		t.Fatalf("主智能体应改为跟随默认模型: %+v", runtimes)
	}
	if runtimes["agent-stale"].provider != stale.Provider || runtimes["agent-stale"].model != "stale-model" {
		t.Fatalf("普通 Agent 的失效绑定应保留，以便 Provider 恢复后自动切回: %+v", runtimes)
	}
	if runtimes["agent-valid"].provider != fallback.Provider || runtimes["agent-valid"].model != "fallback-model" {
		t.Fatalf("可用的显式绑定不应被改写: %+v", runtimes)
	}
}
