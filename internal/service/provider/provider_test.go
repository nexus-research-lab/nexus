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

func setTestDefaultAgentSelection(service *Service, selection DefaultAgentSelection) {
	service.SetDefaultAgentSelectionResolver(func(context.Context, string) (DefaultAgentSelection, error) {
		return selection, nil
	})
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

func TestProviderModelAndDefaultWritesShareAggregateVersion(t *testing.T) {
	ctx := context.Background()
	service, _ := newTestService(t)
	first := createConfigurationVersionTestProvider(t, service, ctx, "provider-first", "https://first.example.com")
	second := createConfigurationVersionTestProvider(t, service, ctx, "provider-second", "https://second.example.com")

	if _, err := service.UpdateModelAtVersion(
		ctx,
		first.Provider,
		"first-model",
		UpdateModelInput{Enabled: true, IsDefault: true},
		first.ConfigurationVersion,
	); err != nil {
		t.Fatal(err)
	}
	firstAfterDefault, err := service.Get(ctx, first.Provider)
	if err != nil {
		t.Fatal(err)
	}
	secondUnchanged, err := service.Get(ctx, second.Provider)
	if err != nil {
		t.Fatal(err)
	}
	if firstAfterDefault.ConfigurationVersion != 2 || secondUnchanged.ConfigurationVersion != 1 {
		t.Fatalf("versions after first default: first=%d second=%d",
			firstAfterDefault.ConfigurationVersion, secondUnchanged.ConfigurationVersion)
	}

	if _, err = service.UpdateModelAtVersion(
		ctx,
		second.Provider,
		"second-model",
		UpdateModelInput{Enabled: true},
		secondUnchanged.ConfigurationVersion,
	); err != nil {
		t.Fatal(err)
	}
	secondWithModel, err := service.Get(ctx, second.Provider)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.SetDefaultModelAtVersion(
		ctx,
		second.Provider,
		"second-model",
		secondWithModel.ConfigurationVersion,
	); err != nil {
		t.Fatal(err)
	}
	firstAfterSwitch, err := service.Get(ctx, first.Provider)
	if err != nil {
		t.Fatal(err)
	}
	secondAfterSwitch, err := service.Get(ctx, second.Provider)
	if err != nil {
		t.Fatal(err)
	}
	if firstAfterSwitch.ConfigurationVersion != 3 || secondAfterSwitch.ConfigurationVersion != 3 {
		t.Fatalf("versions after default switch: first=%d second=%d",
			firstAfterSwitch.ConfigurationVersion, secondAfterSwitch.ConfigurationVersion)
	}
	if len(firstAfterSwitch.Models) != 1 || firstAfterSwitch.Models[0].IsDefault ||
		len(secondAfterSwitch.Models) != 1 || !secondAfterSwitch.Models[0].IsDefault {
		t.Fatalf("default switch failed: first=%+v second=%+v",
			firstAfterSwitch.Models, secondAfterSwitch.Models)
	}

	if _, err = service.UpdateModelAtVersion(
		ctx,
		second.Provider,
		"second-model",
		UpdateModelInput{Enabled: true},
		secondWithModel.ConfigurationVersion,
	); !errors.Is(err, ErrConfigurationVersionConflict) {
		t.Fatalf("stale model update error = %v", err)
	}

	_, err = service.repository.WithProviderMutation(
		ctx,
		first.ID,
		firstAfterSwitch.ConfigurationVersion,
		func(mutation *providerstore.Mutation) error {
			return mutation.UpdateModel(ctx, providerstore.ModelEntity{
				ID: "missing-model-row", ProviderID: first.ID, ModelID: "missing-model",
			})
		},
	)
	if !errors.Is(err, ErrModelNotFound) {
		t.Fatalf("missing model error = %v, want ErrModelNotFound", err)
	}
	firstAfterRollback, err := service.Get(ctx, first.Provider)
	if err != nil {
		t.Fatal(err)
	}
	if firstAfterRollback.ConfigurationVersion != firstAfterSwitch.ConfigurationVersion {
		t.Fatalf("failed model write advanced version: before=%d after=%d",
			firstAfterSwitch.ConfigurationVersion, firstAfterRollback.ConfigurationVersion)
	}
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

func TestProviderTestStateAndModelAdvanceVersionExactlyOnce(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write([]byte(`{"status":"completed","output":[{"type":"message","content":[{"type":"output_text","text":"pong"}]}]}`))
	}))
	t.Cleanup(server.Close)

	ctx := context.Background()
	service, _ := newTestService(t)
	created := createConfigurationVersionTestProvider(t, service, ctx, "provider-test", server.URL)
	result, err := service.TestModelAtVersion(
		ctx,
		created.Provider,
		"tested-model",
		created.ConfigurationVersion,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Success || result.Status != TestStatusSuccess {
		t.Fatalf("test result = %+v", result)
	}
	current, err := service.Get(ctx, created.Provider)
	if err != nil {
		t.Fatal(err)
	}
	if current.ConfigurationVersion != created.ConfigurationVersion+1 ||
		current.LastTestStatus != TestStatusSuccess ||
		len(current.Models) != 1 || !current.Models[0].Enabled {
		t.Fatalf("provider after test = %+v", current)
	}
	if _, err = service.TestModelAtVersion(
		ctx,
		created.Provider,
		"tested-model",
		created.ConfigurationVersion,
	); !errors.Is(err, ErrConfigurationVersionConflict) {
		t.Fatalf("stale test error = %v", err)
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

func TestProviderVisibilityScopesProvidersByOwner(t *testing.T) {
	service, _ := newTestService(t)
	adminCtx := providerTestContext("admin-user", authctx.RoleAdmin)
	ownerACtx := providerTestContext("owner-a", authctx.RoleMember)
	ownerBCtx := providerTestContext("owner-b", authctx.RoleMember)

	publicProvider, err := service.CreatePublic(adminCtx, CreateInput{
		Provider:    "shared",
		PresetKey:   presetCustom,
		APIFormat:   APIFormatAnthropicMessages,
		AuthToken:   "public-key",
		BaseURL:     "https://public.example.com",
		ModelsPath:  "/models",
		Enabled:     true,
		DisplayName: "Public Shared",
	})
	if err != nil {
		t.Fatalf("创建公共 provider 失败: %v", err)
	}
	if publicProvider.Visibility != providerstore.VisibilityPublic || publicProvider.OwnerUserID != "" {
		t.Fatalf("公共 provider scope 不正确: %+v", publicProvider)
	}
	if _, err = service.UpdateModel(adminCtx, publicProvider.Provider, "public-model", UpdateModelInput{Enabled: true, IsDefault: true}); err != nil {
		t.Fatalf("设置公共模型失败: %v", err)
	}

	privateProvider, err := service.Create(ownerBCtx, CreateInput{
		Provider:    "shared",
		PresetKey:   presetCustom,
		APIFormat:   APIFormatAnthropicMessages,
		AuthToken:   "private-key",
		BaseURL:     "https://private.example.com",
		ModelsPath:  "/models",
		Enabled:     true,
		DisplayName: "Private Shared",
	})
	if err != nil {
		t.Fatalf("创建私有 provider 失败: %v", err)
	}
	if privateProvider.Visibility != providerstore.VisibilityPrivate || privateProvider.OwnerUserID != "owner-b" {
		t.Fatalf("私有 provider scope 不正确: %+v", privateProvider)
	}
	if _, err = service.UpdateModel(ownerBCtx, privateProvider.Provider, "private-model", UpdateModelInput{Enabled: true, IsDefault: true}); err != nil {
		t.Fatalf("设置私有模型失败: %v", err)
	}

	ownerAConfig, err := service.ResolveLLMConfig(ownerACtx, "shared", "public-model")
	if err != nil {
		t.Fatalf("owner A 应能使用公共 provider: %v", err)
	}
	if ownerAConfig.AuthToken != "public-key" || ownerAConfig.BaseURL != "https://public.example.com" {
		t.Fatalf("owner A provider 解析不正确: %+v", ownerAConfig)
	}
	ownerBConfig, err := service.ResolveLLMConfig(ownerBCtx, "shared", "private-model")
	if err != nil {
		t.Fatalf("owner B 应优先使用私有 provider: %v", err)
	}
	if ownerBConfig.AuthToken != "private-key" || ownerBConfig.BaseURL != "https://private.example.com" {
		t.Fatalf("owner B provider 解析不正确: %+v", ownerBConfig)
	}

	ownerARecords, err := service.List(ownerACtx)
	if err != nil {
		t.Fatalf("读取 owner A provider 列表失败: %v", err)
	}
	if len(ownerARecords) != 1 || ownerARecords[0].Visibility != providerstore.VisibilityPublic {
		t.Fatalf("owner A 应只看到公共 provider: %+v", ownerARecords)
	}
	ownerBRecords, err := service.List(ownerBCtx)
	if err != nil {
		t.Fatalf("读取 owner B provider 列表失败: %v", err)
	}
	if len(ownerBRecords) != 1 || ownerBRecords[0].Visibility != providerstore.VisibilityPrivate ||
		ownerBRecords[0].DisplayName != "Private Shared" {
		t.Fatalf("owner B 应看到私有 provider 覆盖公共同名项: %+v", ownerBRecords)
	}
}

func TestProviderPublicAdminMethodsUsePublicScope(t *testing.T) {
	service, _ := newTestService(t)
	adminCtx := providerTestContext("admin-user", authctx.RoleAdmin)
	memberCtx := providerTestContext("member-user", authctx.RoleMember)

	publicProvider, err := service.CreatePublic(adminCtx, CreateInput{
		Provider:    "shared-admin",
		PresetKey:   presetCustom,
		APIFormat:   APIFormatAnthropicMessages,
		AuthToken:   "public-key",
		BaseURL:     "https://public.example.com",
		ModelsPath:  "/models",
		Enabled:     true,
		DisplayName: "Public Shared",
	})
	if err != nil {
		t.Fatalf("创建公共 provider 失败: %v", err)
	}
	privateProvider, err := service.Create(adminCtx, CreateInput{
		Provider:    publicProvider.Provider,
		Visibility:  providerstore.VisibilityPrivate,
		PresetKey:   presetCustom,
		APIFormat:   APIFormatAnthropicMessages,
		AuthToken:   "private-key",
		BaseURL:     "https://private.example.com",
		ModelsPath:  "/models",
		Enabled:     true,
		DisplayName: "Private Shared",
	})
	if err != nil {
		t.Fatalf("创建同名私有 provider 失败: %v", err)
	}

	publicRecords, err := service.ListPublic(adminCtx)
	if err != nil {
		t.Fatalf("读取公共 provider 列表失败: %v", err)
	}
	if len(publicRecords) != 1 || publicRecords[0].ID != publicProvider.ID {
		t.Fatalf("公共 provider 列表不应混入同名私有项: %+v", publicRecords)
	}
	updated, err := service.UpdatePublic(adminCtx, publicProvider.Provider, UpdateInput{
		PresetKey:    presetCustom,
		APIFormat:    APIFormatAnthropicMessages,
		DisplayName:  "Public Updated",
		AuthToken:    stringPointer("public-new-key"),
		BaseURL:      "https://public-updated.example.com",
		ModelsPath:   "/models",
		ProviderKind: ProviderKindLLM,
		Enabled:      true,
	})
	if err != nil {
		t.Fatalf("更新公共 provider 失败: %v", err)
	}
	if updated.ID != publicProvider.ID || updated.DisplayName != "Public Updated" {
		t.Fatalf("公共 provider 更新没有命中公共作用域: %+v", updated)
	}
	visible, err := service.Get(adminCtx, publicProvider.Provider)
	if err != nil {
		t.Fatalf("读取可见 provider 失败: %v", err)
	}
	if visible.ID != privateProvider.ID || visible.DisplayName != "Private Shared" {
		t.Fatalf("普通可见读取仍应优先同名私有 provider: %+v", visible)
	}
	if _, err = service.ListPublic(memberCtx); err == nil || !strings.Contains(err.Error(), "只有管理员") {
		t.Fatalf("普通成员不应能读取订阅 provider 管理列表: %v", err)
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

func TestProviderPublicCreateRequiresAdmin(t *testing.T) {
	service, _ := newTestService(t)
	adminCtx := providerTestContext("admin-user", authctx.RoleAdmin)
	memberCtx := providerTestContext("member-user", authctx.RoleMember)

	if _, err := service.Create(memberCtx, CreateInput{
		Provider:   "member-public",
		Visibility: providerstore.VisibilityPublic,
		AuthToken:  "member-key",
		BaseURL:    "https://member.example.com",
	}); err == nil || !strings.Contains(err.Error(), "普通设置只能创建私有 Provider") {
		t.Fatalf("普通设置入口不应能创建公共 provider: %v", err)
	}
	if _, err := service.CreatePublic(memberCtx, CreateInput{
		Provider:  "member-public",
		AuthToken: "member-key",
		BaseURL:   "https://member.example.com",
	}); err == nil || !strings.Contains(err.Error(), "只有管理员") {
		t.Fatalf("普通成员不应通过运营入口创建公共 provider: %v", err)
	}
	adminPrivateProvider, err := service.Create(adminCtx, CreateInput{
		Provider:  "admin-private",
		AuthToken: "admin-key",
		BaseURL:   "https://admin.example.com",
	})
	if err != nil {
		t.Fatalf("管理员普通设置入口应能创建私有 provider: %v", err)
	}
	if adminPrivateProvider.Visibility != providerstore.VisibilityPrivate || adminPrivateProvider.OwnerUserID != "admin-user" {
		t.Fatalf("管理员普通设置入口默认应创建私有 provider: %+v", adminPrivateProvider)
	}

	privateProvider, err := service.Create(memberCtx, CreateInput{
		Provider:  "member-private",
		AuthToken: "member-key",
		BaseURL:   "https://member.example.com",
	})
	if err != nil {
		t.Fatalf("普通成员应能创建私有 provider: %v", err)
	}
	if privateProvider.Visibility != providerstore.VisibilityPrivate || privateProvider.OwnerUserID != "member-user" {
		t.Fatalf("普通成员默认应创建私有 provider: %+v", privateProvider)
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

func TestProviderDisableAndClearLastKeyAllowEmptyDefault(t *testing.T) {
	service, db := newTestService(t)
	ctx := providerTestContext("owner-user", authctx.RoleMember)
	record, err := service.Create(ctx, CreateInput{
		Provider:    "current-default",
		PresetKey:   presetCustom,
		APIFormat:   APIFormatAnthropicMessages,
		AuthToken:   "current-token",
		BaseURL:     "https://current.example.com",
		ModelsPath:  "/models",
		Enabled:     true,
		DisplayName: "Current Default",
	})
	if err != nil {
		t.Fatalf("创建 provider 失败: %v", err)
	}
	if _, err = service.UpdateModel(ctx, record.Provider, "current-model", UpdateModelInput{Enabled: true, IsDefault: true}); err != nil {
		t.Fatalf("设置默认模型失败: %v", err)
	}
	setTestDefaultAgentSelection(service, DefaultAgentSelection{
		Provider:    record.Provider,
		Model:       "current-model",
		RuntimeKind: "claude",
	})
	insertProviderUsageAgentForOwner(t, db, "owner-user", "agent-current", "current", "Current", "", false, record.Provider, "active")

	for _, tc := range []struct {
		name    string
		key     *string
		enabled bool
		wantKey string
	}{
		{"disable", nil, false, "current-token"},
		{"enable", nil, true, "current-token"},
		{"replace", stringPointer("new-token"), true, "new-token"},
		{"clear", stringPointer(""), false, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := service.Update(ctx, record.Provider, UpdateInput{
				ProviderKind: record.ProviderKind, PresetKey: record.PresetKey, APIFormat: record.APIFormat,
				DisplayName: record.DisplayName, AuthToken: tc.key, BaseURL: record.BaseURL,
				ModelsPath: record.ModelsPath, Enabled: tc.enabled,
			})
			if err != nil {
				t.Fatalf("update failed: %v", err)
			}
			entity, err := service.repository.GetVisibleByProvider(ctx, "owner-user", record.Provider)
			if err != nil || entity == nil || entity.Enabled != tc.enabled || entity.AuthToken != tc.wantKey {
				t.Fatal("credential or enabled state was not preserved correctly")
			}
			options, err := service.ListOptions(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if tc.enabled {
				if options.DefaultSelection == nil || options.DefaultSelection.Model != "current-model" {
					t.Fatal("saved default was lost")
				}
			} else {
				if options.DefaultSelection != nil || len(options.Items) != 0 || len(options.BackgroundItems) != 0 || len(options.ImageItems) != 0 || len(options.VisionItems) != 0 {
					t.Fatal("unavailable provider still selectable")
				}
				if _, err := service.ResolveRuntimeConfig(ctx, "", ""); err == nil {
					t.Fatal("runtime invented a default")
				}
			}
			models, err := service.repository.ListModelsByProviderID(ctx, record.ID)
			if err != nil || len(models) != 1 || !models[0].Enabled || !models[0].IsDefault {
				t.Fatal("model configuration was lost")
			}
			runtimes := runtimeSelectionsByAgent(t, db, "agent-current")
			if runtimes["agent-current"].provider != record.Provider {
				t.Fatal("explicit agent binding was lost")
			}
		})
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
