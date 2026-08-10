package configuration_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	configurationsvc "github.com/nexus-research-lab/nexus/internal/service/configuration"
	providersvc "github.com/nexus-research-lab/nexus/internal/service/provider"
)

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
