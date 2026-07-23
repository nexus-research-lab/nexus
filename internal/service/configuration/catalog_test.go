package configuration

import (
	"errors"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	preferencessvc "github.com/nexus-research-lab/nexus/internal/service/preferences"
	providersvc "github.com/nexus-research-lab/nexus/internal/service/provider"
)

func TestCatalogRoutesSpecializedDomains(t *testing.T) {
	definition, err := definitionFor(DomainAutomation)
	if err != nil {
		t.Fatal(err)
	}
	if definition.ManagedBy != "nexus_automation" {
		t.Fatalf("automation managed_by = %q", definition.ManagedBy)
	}
	if _, err = operationFor(definition, "create"); err == nil || !strings.Contains(err.Error(), "nexus_automation") {
		t.Fatalf("delegated write should point to specialized tool: %v", err)
	}
}

func TestCatalogPublishesOperationInputContracts(t *testing.T) {
	definition, err := definitionFor(DomainConnectors)
	if err != nil {
		t.Fatal(err)
	}
	operation, err := operationFor(definition, "save_oauth_client")
	if err != nil {
		t.Fatal(err)
	}
	if operation.TargetDescription == "" || operation.InputShape == nil {
		t.Fatalf("operation contract is incomplete: %+v", operation)
	}
	if len(operation.RequiredInputFields) != 2 {
		t.Fatalf("required input fields = %v", operation.RequiredInputFields)
	}
}

func TestPlanRejectsNonMainAgentBeforeReadingState(t *testing.T) {
	service := &Service{}
	_, err := service.PlanChange(t.Context(), Actor{
		OwnerUserID: "owner", AgentID: "worker", IsMainAgent: false,
	}, ChangeRequest{Domain: DomainPreferences, Operation: "update", Input: []byte(`{}`)})
	if !errors.Is(err, ErrMainAgentRequired) {
		t.Fatalf("PlanChange error = %v, want ErrMainAgentRequired", err)
	}
}

func TestValidateChangeRequestCoversSensitiveAndDestructiveOperations(t *testing.T) {
	cases := []ChangeRequest{
		{Domain: DomainProviders, Operation: "create", Input: []byte(`{"provider":"custom","auth_token":"secret"}`)},
		{Domain: DomainChannels, Operation: "upsert", Target: "feishu", Input: []byte(`{"agent_id":"nexus","credentials":{"app_secret":"secret"}}`)},
		{Domain: DomainConnectors, Operation: "save_oauth_client", Target: "feishu-docx", Input: []byte(`{"client_id":"id","client_secret":"secret"}`)},
		{Domain: DomainSkills, Operation: "install", Target: "planner", Input: []byte(`{"agent_id":"worker"}`)},
		{Domain: DomainHost, Operation: "update_runtime_settings", Input: []byte(`{"workspace_path":"/tmp/nexus"}`)},
	}
	for _, request := range cases {
		if err := validateChangeRequest(request); err != nil {
			t.Fatalf("%s.%s validation failed: %v", request.Domain, request.Operation, err)
		}
	}
}

func TestValidateChangeRequestRejectsUnknownFields(t *testing.T) {
	err := validateChangeRequest(ChangeRequest{
		Domain: DomainPreferences, Operation: "update",
		Input: []byte(`{"agent_sdk_diagnostics_enabledd":true}`),
	})
	if err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("unknown input field should fail planning: %v", err)
	}
}

func TestPreferencesRuntimeEffectDistinguishesLiveAndFutureSettings(t *testing.T) {
	operation := OperationDefinition{RuntimeEffect: "immediate"}
	webSearch := runtimeEffectForRequest(ChangeRequest{
		Domain: DomainPreferences, Operation: "update",
		Input: []byte(`{"web_search":{"enabled":true}}`),
	}, operation)
	if webSearch != "immediate" {
		t.Fatalf("web search effect = %q", webSearch)
	}
	defaults := runtimeEffectForRequest(ChangeRequest{
		Domain: DomainPreferences, Operation: "update",
		Input: []byte(`{"agent_sdk_diagnostics_enabled":true}`),
	}, operation)
	if defaults != "next_session_or_new_agent" {
		t.Fatalf("default setting effect = %q", defaults)
	}
}

func TestProviderUpdatePatchPreservesUnspecifiedFields(t *testing.T) {
	displayName := "Renamed"
	input := providerUpdateRequest{DisplayName: &displayName}
	result := input.serviceInput(providersvc.Record{
		ProviderKind: "llm", PresetKey: "custom", APIFormat: "responses",
		DisplayName: "Old", BaseURL: "https://example.com/v1", ModelsPath: "/models", Enabled: true,
	})
	if result.DisplayName != displayName || !result.Enabled || result.APIFormat != "responses" {
		t.Fatalf("provider patch reset unspecified fields: %+v", result)
	}
	if result.AuthToken != nil {
		t.Fatal("provider patch must preserve the stored token when auth_token is omitted")
	}
}

func TestMergePatchesNestedPreferenceAndAgentOptions(t *testing.T) {
	previous := preferencessvc.DefaultPreferences()
	previous.DefaultAgentOptions.Provider = "openai"
	previous.DefaultAgentOptions.Model = "gpt"
	parsed := preferencessvc.UpdateRequest{}
	merged, err := mergedPreferencesUpdate(
		previous,
		parsed,
		[]byte(`{"default_agent_options":{"permission_mode":"plan"}}`),
	)
	if err != nil {
		t.Fatal(err)
	}
	if merged.DefaultAgentOptions == nil ||
		merged.DefaultAgentOptions.Provider != "openai" ||
		merged.DefaultAgentOptions.Model != "gpt" ||
		merged.DefaultAgentOptions.PermissionMode != "plan" {
		t.Fatalf("nested preference patch reset defaults: %+v", merged.DefaultAgentOptions)
	}

	payload, err := mergeJSONObject(protocol.Options{
		Provider: "openai", Model: "gpt", AllowedTools: []string{"Read"},
	}, []byte(`{"allowed_tools":["Read","Write"]}`))
	if err != nil {
		t.Fatal(err)
	}
	var options protocol.Options
	if err = strictDecodeJSON(payload, &options); err != nil {
		t.Fatal(err)
	}
	if options.Provider != "openai" || options.Model != "gpt" || len(options.AllowedTools) != 2 {
		t.Fatalf("Agent options patch reset model selection: %+v", options)
	}
}

func TestValidateRuntimeSettingsRejectsRelativeWorkspace(t *testing.T) {
	err := validateRuntimeSettings(config.RuntimeSettings{WorkspacePath: "relative/workspace"})
	if err == nil {
		t.Fatal("relative workspace_path must fail planning")
	}
}
