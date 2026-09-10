// INPUT: 对话 Provider 配置、公共订阅与 Agent 显式绑定。
// OUTPUT: 公共 Provider 不可读写，强制删除保留 Agent 显式绑定。
// POS: configuration Provider 权限与删除边界集成测试。
package configuration_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
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

func TestConversationProviderControlCannotInspectOrManagePublicProviders(t *testing.T) {
	fixture := newScopedConfigurationFixture(t)
	public, err := fixture.services.Provider.CreatePublic(
		fixture.ownerCtx,
		providersvc.CreateInput{
			ProviderKind: providersvc.ProviderKindLLM,
			Provider:     "public-dialog-boundary",
			PresetKey:    "custom",
			APIFormat:    providersvc.APIFormatResponses,
			DisplayName:  "Public Dialog Boundary",
			AuthToken:    "public-provider-secret",
			BaseURL:      "https://public-provider.example.com/v1",
			ModelsPath:   "/models",
			Enabled:      true,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	private, err := fixture.services.Provider.Create(
		fixture.ownerCtx,
		providersvc.CreateInput{
			ProviderKind: providersvc.ProviderKindLLM,
			Provider:     "private-dialog-boundary",
			PresetKey:    "custom",
			APIFormat:    providersvc.APIFormatResponses,
			DisplayName:  "Private Dialog Boundary",
			AuthToken:    "private-provider-secret",
			BaseURL:      "https://private-provider.example.com/v1",
			ModelsPath:   "/models",
			Enabled:      true,
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	actor := configurationsvc.Actor{
		OwnerUserID:     fixture.main.OwnerUserID,
		AgentID:         fixture.main.AgentID,
		IsMainAgent:     true,
		SessionKey:      "agent:" + fixture.main.AgentID + ":ws:dm:provider-boundary",
		ContextKind:     configurationsvc.ContextKindAgent,
		ContextID:       fixture.main.AgentID,
		PrincipalRole:   authctx.RoleOwner,
		AuthMethod:      authctx.AuthMethodLocal,
		LocalSingleUser: true,
	}
	bindConfigurationTestRound(t, fixture.services, &actor)

	inspection, err := fixture.services.Configuration.Inspect(
		fixture.ownerCtx,
		actor,
		[]string{configurationsvc.DomainProviders},
		false,
	)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(inspection.Domains[configurationsvc.DomainProviders].Values)
	if err != nil {
		t.Fatal(err)
	}
	text := string(payload)
	if !strings.Contains(text, private.BaseURL) {
		t.Fatalf("owner private Provider missing from conversational inspection: %s", text)
	}
	for _, forbidden := range []string{
		public.BaseURL,
		public.AuthTokenMasked,
	} {
		if forbidden != "" && strings.Contains(text, forbidden) {
			t.Fatalf("public Provider configuration leaked through conversational inspection (%s): %s", forbidden, text)
		}
	}

	if _, err = fixture.services.Configuration.PlanChange(
		fixture.ownerCtx,
		actor,
		configurationsvc.ChangeRequest{
			Domain:    configurationsvc.DomainProviders,
			Operation: "update",
			Target:    public.Provider,
			Input:     json.RawMessage(`{"display_name":"Agent Controlled"}`),
		},
	); err == nil || !strings.Contains(err.Error(), "私有") {
		t.Fatalf("public Provider must stay human-admin-only, got: %v", err)
	}
	if _, err = fixture.services.Configuration.PlanChange(
		fixture.ownerCtx,
		actor,
		configurationsvc.ChangeRequest{
			Domain:    configurationsvc.DomainProviders,
			Operation: "create",
			Input: json.RawMessage(`{
				"provider":"forged-public",
				"visibility":"public",
				"auth_token":{"$secret":"provider.auth_token"}
			}`),
		},
	); err == nil || !strings.Contains(err.Error(), "公共订阅 Provider") {
		t.Fatalf("conversation must not create public Provider, got: %v", err)
	}
}
