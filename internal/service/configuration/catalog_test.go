package configuration

import (
	"encoding/json"
	"strings"
	"testing"

	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
	"github.com/nexus-research-lab/nexus/internal/infra/secretinput"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/runtime/clientopts"
)

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
			Input: []byte(`{"name":"worker","options":{"mcp_servers":{"nexus_shadow":{"command":"forged"}}}}`),
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

func TestClassifyChangeRiskRequiresHumanApprovalForSensitiveInput(t *testing.T) {
	risk, requires := classifyChangeRisk(OperationDefinition{}, ChangeRequest{
		Domain: DomainPreferences, Operation: "update",
		Input: []byte(`{"web_search_api_key":"secret"}`),
	})
	if risk != "sensitive" || !requires {
		t.Fatalf("risk=%q requires=%v, want sensitive/true", risk, requires)
	}
}

func TestClassifyChangeRiskRequiresHumanApprovalForRuntimePreferences(t *testing.T) {
	for _, input := range []string{
		`{"agent_runtime_kind":"nxs"}`,
		`{"runtime_settings":{"nxs":{"setting_sources":["user"]}}}`,
		`{"web_search":{"enabled":true}}`,
		`{"default_agent_options":{"model":"expensive"}}`,
		`{"default_image_model_selection":{"provider":"custom","model":"image"}}`,
	} {
		risk, requires := classifyChangeRisk(OperationDefinition{}, ChangeRequest{
			Domain: DomainPreferences, Operation: "update", Input: []byte(input),
		})
		if risk != "high_risk" || !requires {
			t.Fatalf("input=%s risk=%q requires=%v, want high_risk/true", input, risk, requires)
		}
	}
}

func TestClassifyChangeRiskDistinguishesHighRiskFromDestructive(t *testing.T) {
	risk, requires := classifyChangeRisk(OperationDefinition{
		RequiresConfirmation: true,
	}, ChangeRequest{Domain: DomainChannels, Operation: "upsert"})
	if risk != "high_risk" || !requires {
		t.Fatalf("upsert risk=%q requires=%v, want high_risk/true", risk, requires)
	}

	risk, requires = classifyChangeRisk(OperationDefinition{
		RequiresConfirmation: true,
	}, ChangeRequest{Domain: DomainChannels, Operation: "delete_config"})
	if risk != "destructive" || !requires {
		t.Fatalf("delete risk=%q requires=%v, want destructive/true", risk, requires)
	}
}

func TestCatalogRequiresHumanApprovalForExternalAndExecutableChanges(t *testing.T) {
	for _, reference := range []struct {
		domain    string
		operation string
	}{
		{DomainProviders, "create"},
		{DomainAgents, "create"},
		{DomainChannels, "upsert"},
		{DomainConnectors, "connect"},
		{DomainSkills, "import_git"},
		{DomainSkills, "import_url"},
		{DomainSkills, "import_skills_sh"},
		{DomainSkills, "update_source"},
		{DomainSkills, "install"},
		{DomainSkills, "install_self"},
		{DomainSkills, "uninstall"},
		{DomainSkills, "uninstall_self"},
		{DomainSkills, "update_single"},
		{DomainAgents, "update_self_runtime"},
		{DomainRooms, "update_profile"},
	} {
		definition, err := definitionFor(reference.domain)
		if err != nil {
			t.Fatal(err)
		}
		operation, err := operationFor(definition, reference.operation)
		if err != nil {
			t.Fatal(err)
		}
		if !operation.RequiresConfirmation {
			t.Fatalf("%s.%s must require human approval", reference.domain, reference.operation)
		}
	}
}
