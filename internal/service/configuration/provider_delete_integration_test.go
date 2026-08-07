// INPUT: 对话配置强制删除正在使用的 Provider 与可用的替代默认模型。
// OUTPUT: 显式 Agent 绑定保持不变、下一轮动态回退及受影响 runtime 计数证明。
// POS: configuration Provider 删除到 Agent 下一轮动态解析的跨域集成测试。
package configuration_test

import (
	"encoding/json"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	configurationsvc "github.com/nexus-research-lab/nexus/internal/service/configuration"
	providersvc "github.com/nexus-research-lab/nexus/internal/service/provider"
)

func TestProviderForceDeletePreservesExplicitAgentBinding(t *testing.T) {
	fixture := newScopedConfigurationFixture(t)
	worker := fixture.createAgent(t, "Provider Reassignment Worker")
	fallback, err := fixture.services.Provider.Create(fixture.ownerCtx, providersvc.CreateInput{
		Provider:    "configuration-fallback",
		PresetKey:   "custom",
		APIFormat:   providersvc.APIFormatAnthropicMessages,
		DisplayName: "Configuration Fallback",
		AuthToken:   "fallback-token",
		BaseURL:     "https://fallback.example.com",
		ModelsPath:  "/models",
		Enabled:     true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = fixture.services.Provider.UpdateModelAtVersion(
		fixture.ownerCtx,
		fallback.Provider,
		"fallback-model",
		providersvc.UpdateModelInput{Enabled: true, IsDefault: true},
		fallback.ConfigurationVersion,
	); err != nil {
		t.Fatal(err)
	}
	target, err := fixture.services.Provider.Create(fixture.ownerCtx, providersvc.CreateInput{
		Provider:    "configuration-delete-target",
		PresetKey:   "custom",
		APIFormat:   providersvc.APIFormatAnthropicMessages,
		DisplayName: "Configuration Delete Target",
		AuthToken:   "target-token",
		BaseURL:     "https://target.example.com",
		ModelsPath:  "/models",
		Enabled:     true,
	})
	if err != nil {
		t.Fatal(err)
	}
	options := worker.Options
	options.Provider = target.Provider
	options.Model = "target-model"
	worker, err = fixture.services.Core.Agent.UpdateAgent(
		fixture.ownerCtx,
		worker.AgentID,
		protocol.UpdateRequest{Options: &options},
	)
	if err != nil {
		t.Fatal(err)
	}

	actor := configurationsvc.Actor{
		OwnerUserID: fixture.main.OwnerUserID,
		AgentID:     fixture.main.AgentID,
		IsMainAgent: true,
		SessionKey:  "agent:" + fixture.main.AgentID + ":ws:dm:provider-delete",
		ContextKind: configurationsvc.ContextKindAgent,
		ContextID:   fixture.main.AgentID,
	}
	bindConfigurationTestRound(t, fixture.services, &actor)
	input := json.RawMessage(`{"force":true}`)
	plan, err := fixture.services.Configuration.PlanChange(
		fixture.ownerCtx,
		actor,
		configurationsvc.ChangeRequest{
			Domain: configurationsvc.DomainProviders, Operation: "delete",
			Target: target.Provider, Input: input,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if plan.StateVersion != target.ConfigurationVersion {
		t.Fatalf("delete plan version = %d, want %d", plan.StateVersion, target.ConfigurationVersion)
	}
	request := configurationsvc.ChangeRequest{
		RequestID: "provider-force-delete-notifier-01",
		Domain:    configurationsvc.DomainProviders, Operation: "delete",
		Target: target.Provider, Input: input,
		ExpectedRevision: plan.CurrentRevision, PlanDigest: plan.PlanDigest,
	}
	approveConfigurationTestChange(t, fixture.services, fixture.ownerCtx, actor, request, plan)
	applied, err := fixture.services.Configuration.ApplyChange(fixture.ownerCtx, actor, request)
	if err != nil {
		t.Fatal(err)
	}
	if !applied.Applied {
		t.Fatalf("force delete was not applied: %+v", applied)
	}
	updated, err := fixture.services.Core.Agent.GetAgent(fixture.ownerCtx, worker.AgentID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Options.Provider != target.Provider ||
		updated.Options.Model != "target-model" ||
		updated.RuntimeVersion != worker.RuntimeVersion {
		t.Fatalf("force delete rewrote explicit Agent binding: options=%+v runtime_version=%d, previous=%d",
			updated.Options, updated.RuntimeVersion, worker.RuntimeVersion)
	}
}
