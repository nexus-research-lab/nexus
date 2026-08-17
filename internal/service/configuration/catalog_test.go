package configuration

import (
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"

	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
	"github.com/nexus-research-lab/nexus/internal/infra/secretinput"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/runtime/clientopts"
	preferencessvc "github.com/nexus-research-lab/nexus/internal/service/preferences"
	providersvc "github.com/nexus-research-lab/nexus/internal/service/provider"
)

func TestCatalogRoutesSpecializedDomains(t *testing.T) {
	definition, err := definitionFor(DomainAutomation)
	if err != nil {
		t.Fatal(err)
	}
	if definition.ManagedBy != "nexus automation" {
		t.Fatalf("automation managed_by = %q", definition.ManagedBy)
	}
	if _, err = operationFor(definition, "create"); err == nil || !strings.Contains(err.Error(), "nexus automation") {
		t.Fatalf("delegated write should point to specialized CLI: %v", err)
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

func TestSkillCatalogRequiresScopedSourceIdentityAndDisableConfirmation(t *testing.T) {
	definition, err := definitionFor(DomainSkills)
	if err != nil {
		t.Fatal(err)
	}
	for _, operationName := range []string{"install", "uninstall", "install_self", "uninstall_self"} {
		operation, operationErr := operationFor(definition, operationName)
		if operationErr != nil {
			t.Fatal(operationErr)
		}
		if !slices.Contains(operation.RequiredInputFields, "target_scope") ||
			!slices.Contains(operation.RequiredInputFields, "source_identity") {
			t.Fatalf("%s 缺少明确来源字段: %+v", operationName, operation)
		}
	}
	for _, operationName := range []string{"uninstall", "uninstall_self"} {
		operation, operationErr := operationFor(definition, operationName)
		if operationErr != nil {
			t.Fatal(operationErr)
		}
		if !operation.RequiresConfirmation {
			t.Fatalf("%s 必须要求显式确认: %+v", operationName, operation)
		}
	}
}

func TestSkillCatalogPublishesOwnerPrivateSourceContracts(t *testing.T) {
	definition, err := definitionFor(DomainSkills)
	if err != nil {
		t.Fatal(err)
	}
	for _, operationName := range []string{
		"create_private_source",
		"update_source",
		"import_private",
		"delete_private_source",
	} {
		operation, operationErr := operationFor(definition, operationName)
		if operationErr != nil {
			t.Fatal(operationErr)
		}
		if !operation.RequiresConfirmation {
			t.Fatalf("%s 必须要求显式确认: %+v", operationName, operation)
		}
	}
	for _, operationName := range []string{"create_private_source", "update_source"} {
		operation, operationErr := operationFor(definition, operationName)
		if operationErr != nil {
			t.Fatal(operationErr)
		}
		shape, ok := operation.InputShape.(map[string]any)
		if !ok {
			t.Fatalf("%s input shape = %#v", operationName, operation.InputShape)
		}
		tokenShape, ok := shape["token"].(map[string]string)
		if !ok || strings.TrimSpace(tokenShape["$secret"]) == "" {
			t.Fatalf("%s token 必须是原生 secret slot: %#v", operationName, shape["token"])
		}
	}
	selfAccess := accessFor(&resolvedActor{Authority: AuthorityAgentSelf}, definition)
	for _, operationName := range []string{
		"create_private_source",
		"update_source",
		"import_private",
		"delete_private_source",
	} {
		if slices.Contains(selfAccess.AllowedOperations, operationName) {
			t.Fatalf("agent_self 不得获得 owner 私有来源操作 %s", operationName)
		}
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
		{
			Domain: DomainSkills, Operation: "install", Target: "planner",
			Input: []byte(`{"agent_id":"worker","target_scope":"global_library","source_identity":"skill-source:test"}`),
		},
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

func TestValidateAgentChangeRejectsSkillSelectionBypass(t *testing.T) {
	cases := []ChangeRequest{
		{
			Domain: DomainAgents, Operation: "create",
			Input: []byte(`{"name":"worker","options":{"skill_ids":["external:demo"]}}`),
		},
		{
			Domain: DomainAgents, Operation: "update", Target: "worker",
			Input: []byte(`{"options":{"disabled_skill_ids":["demo"]}}`),
		},
		{
			Domain: DomainPreferences, Operation: "update",
			Input: []byte(`{"default_agent_options":{"skill_ids":["external:demo"]}}`),
		},
		{
			Domain: DomainAgents, Operation: "update", Target: "worker",
			Input: []byte(`{"options":{"skill_ids":null}}`),
		},
	}
	for _, request := range cases {
		err := validateChangeRequest(request)
		if err == nil || !strings.Contains(err.Error(), "Skills 操作") {
			t.Fatalf("%s.%s skill bypass error = %v", request.Domain, request.Operation, err)
		}
	}
}

func TestValidateSkillSelectionRejectsAmbiguousSource(t *testing.T) {
	for _, input := range []string{
		`{"agent_id":"worker","source_identity":"skill-source:test"}`,
		`{"agent_id":"worker","target_scope":"wrong","source_identity":"skill-source:test"}`,
		`{"agent_id":"worker","target_scope":"global_library"}`,
	} {
		err := validateChangeRequest(ChangeRequest{
			Domain: DomainSkills, Operation: "install", Target: "planner", Input: []byte(input),
		})
		if err == nil {
			t.Fatalf("ambiguous Skill input should fail: %s", input)
		}
	}
}

func TestValidateChangeRequestRejectsEmptyMutationPatches(t *testing.T) {
	cases := []ChangeRequest{
		{Domain: DomainPreferences, Operation: "update", Input: []byte(`{}`)},
		{Domain: DomainProviders, Operation: "update", Target: "custom", Input: []byte(`{}`)},
		{Domain: DomainAgents, Operation: "update", Target: "worker", Input: []byte(`{}`)},
		{Domain: DomainAgents, Operation: "update_self_profile", Input: []byte(`{}`)},
		{Domain: DomainAgents, Operation: "update_self_runtime", Input: []byte(`{}`)},
		{Domain: DomainChannels, Operation: "update_pairing", Target: "pairing", Input: []byte(`{}`)},
		{Domain: DomainRooms, Operation: "update_profile", Input: []byte(`{}`)},
		{Domain: DomainRooms, Operation: "set_collaboration_policy", Input: []byte(`{}`)},
	}
	for _, request := range cases {
		if err := validateChangeRequest(request); err == nil || !strings.Contains(err.Error(), "至少要提供一个") {
			t.Fatalf("%s.%s empty patch error = %v", request.Domain, request.Operation, err)
		}
	}
}

func TestValidateSelfRuntimeRejectsLimitRemoval(t *testing.T) {
	for _, input := range []string{
		`{"max_turns":0}`,
		`{"max_thinking_tokens":0}`,
		`{"max_turns":-1}`,
	} {
		err := validateChangeRequest(ChangeRequest{
			Domain: DomainAgents, Operation: "update_self_runtime", Input: []byte(input),
		})
		if err == nil || !strings.Contains(err.Error(), "必须大于 0") {
			t.Fatalf("self runtime limit removal input=%s error=%v", input, err)
		}
	}
}

func TestValidateChangeRequestRejectsInvalidManagedMCPConfiguration(t *testing.T) {
	cases := []ChangeRequest{
		{
			Domain: DomainAgents, Operation: "create",
			Input: []byte(`{"name":"worker","options":{"mcp_servers":{"nexus_goal":{"command":"forged"}}}}`),
		},
		{
			Domain: DomainAgents, Operation: "update", Target: "worker",
			Input: []byte(`{"options":{"mcp_servers":{"with space":{"command":"custom"}}}}`),
		},
		{
			Domain: DomainPreferences, Operation: "update",
			Input: []byte(`{"default_agent_options":{"mcp_servers":{"amap_maps":{"command":"custom"}}}}`),
		},
	}
	for _, request := range cases {
		if err := validateChangeRequest(request); err == nil || !strings.Contains(err.Error(), "MCP server") {
			t.Fatalf("%s.%s invalid MCP error = %v", request.Domain, request.Operation, err)
		}
	}
}

func TestSecretTemplateMaterializesIntoTypedMCPConfiguration(t *testing.T) {
	template := json.RawMessage(`{
		"name": "worker",
		"options": {
			"mcp_servers": {
				"local_tools": {
					"type": "stdio",
					"command": "npx",
					"args": ["-y", "@example/local-mcp"],
					"env": {"LOCAL_TOKEN": {"$secret": "mcp.local.token"}}
				},
				"remote_tools": {
					"type": "http",
					"url": "https://mcp.example.test/rpc",
					"headers": {"Authorization": {"$secret": "mcp.remote.authorization"}},
					"oauth": {
						"clientId": "public-client",
						"callbackPort": 43123,
						"xaa": true
					}
				}
			}
		}
	}`)
	prepared, slots, err := secretinput.PrepareJSON(template)
	if err != nil {
		t.Fatalf("PrepareJSON() error = %v", err)
	}
	if len(slots) != 2 {
		t.Fatalf("secret slots = %#v, want env and header leaves", slots)
	}
	preparedRequest := ChangeRequest{
		Domain: DomainAgents, Operation: "create", Input: prepared,
	}
	if err = validateChangeRequest(preparedRequest); err != nil {
		t.Fatalf("prepared MCP configuration failed validation: %v", err)
	}

	materialized, err := secretinput.MaterializeJSON(template, map[string]string{
		"mcp.local.token":          "local-secret",
		"mcp.remote.authorization": "Bearer remote-secret",
	})
	if err != nil {
		t.Fatalf("MaterializeJSON() error = %v", err)
	}
	materializedRequest := preparedRequest
	materializedRequest.Input = materialized
	if err = validateChangeRequest(materializedRequest); err != nil {
		t.Fatalf("materialized MCP configuration failed validation: %v", err)
	}
	var createRequest protocol.CreateRequest
	if err = strictDecodeJSON(materialized, &createRequest); err != nil {
		t.Fatalf("decode materialized Agent: %v", err)
	}
	merged, err := clientopts.MergeAgentMCPServers(nil, createRequest.Options.MCPServers)
	if err != nil {
		t.Fatalf("parse materialized MCP configuration: %v", err)
	}
	local, ok := merged["local_tools"].(sdkmcp.StdioServerConfig)
	if !ok || local.Command != "npx" ||
		len(local.Args) != 2 || local.Args[1] != "@example/local-mcp" ||
		local.Env["LOCAL_TOKEN"] != "local-secret" {
		t.Fatalf("typed stdio MCP configuration = %#v", merged["local_tools"])
	}
	remote, ok := merged["remote_tools"].(sdkmcp.HTTPServerConfig)
	if !ok || remote.URL != "https://mcp.example.test/rpc" ||
		remote.Headers["Authorization"] != "Bearer remote-secret" {
		t.Fatalf("typed HTTP MCP configuration = %#v", merged["remote_tools"])
	}
	if remote.OAuth == nil || remote.OAuth.ClientID != "public-client" ||
		remote.OAuth.CallbackPort != 43123 ||
		remote.OAuth.XAA == nil || !*remote.OAuth.XAA {
		t.Fatalf("typed MCP OAuth configuration = %#v", remote.OAuth)
	}
}

func TestSecretTemplatePreservesMixedProviderOptions(t *testing.T) {
	template := json.RawMessage(`{
		"model_id": "image-model",
		"input": {
			"enabled": true,
			"is_default": false,
			"capabilities_override": {"vision": true},
			"context_window": 128000,
			"provider_options": {
				"temperature": 0.25,
				"stream": true,
				"stop": ["END"],
				"thinking": {"type": "enabled", "budget_tokens": 4096},
				"api_key": {"$secret": "provider.option.api_key"}
			}
		}
	}`)
	prepared, slots, err := secretinput.PrepareJSON(template)
	if err != nil {
		t.Fatalf("PrepareJSON() error = %v", err)
	}
	if len(slots) != 1 || slots[0].Path != "input.provider_options.api_key" {
		t.Fatalf("provider option slots = %#v", slots)
	}
	preparedRequest := ChangeRequest{
		Domain: DomainProviders, Operation: "update_model", Target: "custom", Input: prepared,
	}
	if err = validateChangeRequest(preparedRequest); err != nil {
		t.Fatalf("prepared provider options failed validation: %v", err)
	}
	materialized, err := secretinput.MaterializeJSON(template, map[string]string{
		"provider.option.api_key": "provider-secret",
	})
	if err != nil {
		t.Fatalf("MaterializeJSON() error = %v", err)
	}
	materializedRequest := preparedRequest
	materializedRequest.Input = materialized
	if err = validateChangeRequest(materializedRequest); err != nil {
		t.Fatalf("materialized provider options failed validation: %v", err)
	}
	var mutation providerModelMutation
	if err = strictDecodeJSON(materialized, &mutation); err != nil {
		t.Fatalf("decode materialized provider options: %v", err)
	}
	options := mutation.Input.ProviderOptions
	if options["temperature"] != 0.25 || options["stream"] != true {
		t.Fatalf("provider option scalar types changed: %#v", options)
	}
	stop, ok := options["stop"].([]any)
	if !ok || len(stop) != 1 || stop[0] != "END" {
		t.Fatalf("provider option array changed: %#v", options["stop"])
	}
	thinking, ok := options["thinking"].(map[string]any)
	if !ok || thinking["type"] != "enabled" ||
		thinking["budget_tokens"] != float64(4096) {
		t.Fatalf("provider option object/number changed: %#v", options["thinking"])
	}
	if options["api_key"] != "provider-secret" {
		t.Fatalf("provider option secret was not materialized: %#v", options["api_key"])
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

func TestReloadStatusExplainsDeferredAndMixedEffects(t *testing.T) {
	tests := []struct {
		effect               string
		state                string
		currentRoundAffected bool
	}{
		{effect: "immediate", state: "applied", currentRoundAffected: true},
		{effect: "mixed", state: "scheduled", currentRoundAffected: true},
		{effect: "restart_required", state: "restart_required"},
		{effect: "next_round", state: "scheduled"},
		{effect: "next_session", state: "scheduled"},
		{effect: "next_ingress", state: "scheduled"},
		{effect: "next_session_or_new_agent", state: "scheduled"},
		{effect: "ui_immediate_runtime_next_round", state: "scheduled"},
		{effect: "ui_immediate", state: "applied"},
		{effect: "ui_immediate_runtime_next_input", state: "scheduled"},
		{effect: "authority_immediate_routing_next_input", state: "applied", currentRoundAffected: true},
		{effect: "security_immediate_runtime_next_round", state: "applied", currentRoundAffected: true},
		{effect: "mixed: WebSearch immediate; defaults later", state: "scheduled", currentRoundAffected: true},
	}
	for _, test := range tests {
		status := reloadStatusFor(ChangeRequest{}, test.effect)
		if status.State != test.state ||
			status.CurrentRoundAffected != test.currentRoundAffected ||
			strings.TrimSpace(status.Message) == "" {
			t.Fatalf("effect %q reload status = %+v", test.effect, status)
		}
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
