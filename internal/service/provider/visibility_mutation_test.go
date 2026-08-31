package provider

import (
	"context"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	providerstore "github.com/nexus-research-lab/nexus/internal/storage/provider"
)

func TestProviderListIncludesUsageAgents(t *testing.T) {
	ctx := context.Background()
	service, db := newTestService(t)
	record, err := service.Create(ctx, CreateInput{
		Provider:    "blocked",
		PresetKey:   presetCustom,
		APIFormat:   APIFormatAnthropicMessages,
		AuthToken:   "blocked-key",
		BaseURL:     "https://api.example.com",
		ModelsPath:  "/models",
		DisplayName: "Blocked",
	})
	if err != nil {
		t.Fatalf("创建 provider 失败: %v", err)
	}
	insertProviderUsageAgent(t, db, "agent-main", "main", "main", "主助手", true, record.Provider, "active")
	insertProviderUsageAgent(t, db, "agent-worker", "worker", "worker", "", false, record.Provider, "active")
	insertProviderUsageAgent(t, db, "agent-archived", "archived", "archived", "归档助手", false, record.Provider, "archived")

	records, err := service.List(ctx)
	if err != nil {
		t.Fatalf("读取 provider 列表失败: %v", err)
	}
	var target *Record
	for index := range records {
		if records[index].Provider == record.Provider {
			target = &records[index]
			break
		}
	}
	if target == nil {
		t.Fatalf("未找到 provider: %+v", records)
	}
	if target.UsageCount != 2 {
		t.Fatalf("usage_count 应只统计 active Agent: %+v", target)
	}
	if len(target.UsedByAgents) != 2 {
		t.Fatalf("used_by_agents 数量不正确: %+v", target.UsedByAgents)
	}
	if target.UsedByAgents[0].AgentID != "agent-main" || target.UsedByAgents[0].DisplayName != "主助手" || !target.UsedByAgents[0].IsMain {
		t.Fatalf("主 Agent 摘要不正确: %+v", target.UsedByAgents[0])
	}
	if target.UsedByAgents[1].AgentID != "agent-worker" || target.UsedByAgents[1].DisplayName != "worker" {
		t.Fatalf("普通 Agent 摘要不正确: %+v", target.UsedByAgents[1])
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

func TestProviderDisablePreservesExplicitBindingsForAutomaticRestore(t *testing.T) {
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
	setTestDefaultAgentSelection(service, DefaultAgentSelection{
		Provider:    fallback.Provider,
		Model:       "fallback-model",
		RuntimeKind: "claude",
	})
	record, err := service.Create(ctx, CreateInput{
		Provider:    "used-private",
		PresetKey:   presetCustom,
		APIFormat:   APIFormatAnthropicMessages,
		AuthToken:   "private-token",
		BaseURL:     "https://private.example.com",
		ModelsPath:  "/models",
		Enabled:     true,
		DisplayName: "Used Private",
	})
	if err != nil {
		t.Fatalf("创建私有 provider 失败: %v", err)
	}
	insertProviderUsageAgentForOwner(t, db, "owner-user", "agent-main", "main", "Nexus", "", true, record.Provider, "active")
	insertProviderUsageAgentForOwner(t, db, "owner-user", "agent-used-private", "used-private", "Used Private Agent", "", false, record.Provider, "active")
	if _, err = db.Exec(`UPDATE runtimes SET model = ? WHERE agent_id IN (?, ?)`, "target-model", "agent-main", "agent-used-private"); err != nil {
		t.Fatalf("写入显式模型绑定失败: %v", err)
	}

	updated, err := service.Update(ctx, record.Provider, UpdateInput{
		PresetKey:    record.PresetKey,
		APIFormat:    record.APIFormat,
		DisplayName:  record.DisplayName,
		BaseURL:      record.BaseURL,
		ModelsPath:   record.ModelsPath,
		ProviderKind: record.ProviderKind,
		AuthToken:    stringPointer(""),
		Enabled:      false,
	})
	if err != nil {
		t.Fatalf("关闭正在使用的 provider 应成功: %v", err)
	}
	if updated.Enabled {
		t.Fatalf("provider 应已关闭: %+v", updated)
	}
	entity, err := service.repository.GetVisibleByProvider(ctx, "owner-user", record.Provider)
	if err != nil {
		t.Fatalf("读取 provider 失败: %v", err)
	}
	if entity == nil || entity.AuthToken != "" {
		t.Fatalf("关闭 provider 应清空 token: %+v", entity)
	}
	runtimes := runtimeSelectionsByAgent(t, db, "agent-main", "agent-used-private")
	if runtimes["agent-main"].provider != record.Provider || runtimes["agent-main"].model != "target-model" ||
		runtimes["agent-used-private"].provider != record.Provider || runtimes["agent-used-private"].model != "target-model" {
		t.Fatalf("失效 provider 的显式绑定应保留，以便恢复后自动切回: %+v", runtimes)
	}
}

func TestProviderDisableRejectsCurrentDefaultModel(t *testing.T) {
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

	if _, err = service.Update(ctx, record.Provider, UpdateInput{
		ProviderKind: record.ProviderKind,
		PresetKey:    record.PresetKey,
		APIFormat:    record.APIFormat,
		DisplayName:  record.DisplayName,
		AuthToken:    stringPointer(""),
		BaseURL:      record.BaseURL,
		ModelsPath:   record.ModelsPath,
		Enabled:      false,
	}); err == nil || !strings.Contains(err.Error(), "默认模型仍使用") {
		t.Fatalf("删除当前默认模型凭据应被拒绝: %v", err)
	}
	entity, err := service.repository.GetVisibleByProvider(ctx, "owner-user", record.Provider)
	if err != nil {
		t.Fatalf("读取 provider 失败: %v", err)
	}
	if entity == nil || !entity.Enabled || entity.AuthToken != "current-token" {
		t.Fatalf("被拒绝后 provider 不应变更: %+v", entity)
	}
	runtimes := runtimeSelectionsByAgent(t, db, "agent-current")
	if runtimes["agent-current"].provider != record.Provider {
		t.Fatalf("被拒绝后显式绑定不应变更: %+v", runtimes)
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

func TestForceDeleteProviderPreservesExplicitBindingsForAutomaticRestore(t *testing.T) {
	ctx := context.Background()
	service, db := newTestService(t)
	fallback, err := service.Create(ctx, CreateInput{
		Provider:    "fallback-provider",
		PresetKey:   presetCustom,
		APIFormat:   APIFormatAnthropicMessages,
		AuthToken:   "fallback-key",
		BaseURL:     "https://api.fallback.example.com",
		ModelsPath:  "/models",
		Enabled:     true,
		DisplayName: "Fallback",
	})
	if err != nil {
		t.Fatalf("创建 fallback provider 失败: %v", err)
	}
	if _, err = service.UpdateModel(ctx, fallback.Provider, "fallback-model", UpdateModelInput{
		Enabled:   true,
		IsDefault: true,
	}); err != nil {
		t.Fatalf("设置 fallback 默认模型失败: %v", err)
	}
	setTestDefaultAgentSelection(service, DefaultAgentSelection{
		Provider:    fallback.Provider,
		Model:       "fallback-model",
		RuntimeKind: "claude",
	})
	target, err := service.Create(ctx, CreateInput{
		Provider:    "delete-target",
		PresetKey:   presetCustom,
		APIFormat:   APIFormatAnthropicMessages,
		AuthToken:   "target-key",
		BaseURL:     "https://api.target.example.com",
		ModelsPath:  "/models",
		Enabled:     true,
		DisplayName: "Target",
	})
	if err != nil {
		t.Fatalf("创建待删除 provider 失败: %v", err)
	}
	if _, err = service.UpdateModel(ctx, target.Provider, "target-model", UpdateModelInput{
		Enabled:   true,
		IsDefault: true,
	}); err != nil {
		t.Fatalf("设置待删除模型失败: %v", err)
	}
	insertProviderUsageAgent(t, db, "agent-force-a", "force-a", "Force A", "", true, target.Provider, "active")
	insertProviderUsageAgent(t, db, "agent-force-b", "force-b", "Force B", "", false, target.Provider, "active")
	insertProviderUsageAgent(t, db, "agent-force-archived", "force-archived", "Force Archived", "", false, target.Provider, "archived")
	if _, err = db.Exec(`UPDATE runtimes SET model = ? WHERE agent_id IN (?, ?, ?)`, "target-model", "agent-force-a", "agent-force-b", "agent-force-archived"); err != nil {
		t.Fatalf("写入待删除模型绑定失败: %v", err)
	}
	if _, err = service.Delete(ctx, target.Provider, DeleteInput{}); err == nil {
		t.Fatalf("普通删除应被正在使用的 provider 阻止")
	}
	result, err := service.Delete(ctx, target.Provider, DeleteInput{Force: true})
	if err != nil {
		t.Fatalf("强制删除 provider 失败: %v", err)
	}
	if !result.FallbackToDefault || result.AffectedRuntimeCount != 3 {
		t.Fatalf("强制删除结果不正确: %+v", result)
	}
	if _, err = service.Get(ctx, target.Provider); err == nil {
		t.Fatalf("待删除 provider 应已移除")
	}
	runtimes := runtimeSelectionsByAgent(t, db, "agent-force-a", "agent-force-b", "agent-force-archived")
	if runtimes["agent-force-a"].provider != target.Provider ||
		runtimes["agent-force-a"].model != "target-model" ||
		runtimes["agent-force-b"].provider != target.Provider ||
		runtimes["agent-force-b"].model != "target-model" ||
		runtimes["agent-force-archived"].provider != target.Provider ||
		runtimes["agent-force-archived"].model != "target-model" {
		t.Fatalf("runtime provider/model 应保留原选择，以便重新配置后自动恢复: %+v", runtimes)
	}
	options, err := service.ListOptions(ctx)
	if err != nil {
		t.Fatalf("读取 provider options 失败: %v", err)
	}
	if optionByProvider(options.Items, fallback.Provider) == nil {
		t.Fatalf("回退 Provider 不应随目标 Provider 一起删除: %+v", options)
	}
}
