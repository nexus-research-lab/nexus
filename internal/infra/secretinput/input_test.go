// INPUT: configuration templates containing secret placeholders, direct values, and malformed slots.
// OUTPUT: proof that only human-materialized values cross the write boundary and model-visible projections stay redacted.
// POS: regression tests for the conversational configuration secret boundary.
package secretinput

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestJSONPreparationAndMaterialization(t *testing.T) {
	template := json.RawMessage(`{
		"provider":"custom",
		"auth_token":{"$secret":"provider.auth_token"},
		"nested":{"client_secret":{"$secret":"provider.client_secret"}}
	}`)

	prepared, slots, err := PrepareJSON(template)
	if err != nil {
		t.Fatalf("PrepareJSON() error = %v", err)
	}
	if len(slots) != 2 ||
		slots[0] != (Slot{ID: "provider.auth_token", Path: "auth_token"}) ||
		slots[1] != (Slot{ID: "provider.client_secret", Path: "nested.client_secret"}) {
		t.Fatalf("slots = %#v", slots)
	}
	if strings.Contains(string(prepared), "provider.auth_token") ||
		!strings.Contains(string(prepared), validationSecret) {
		t.Fatalf("prepared payload did not replace slots: %s", prepared)
	}

	materialized, err := MaterializeJSON(template, map[string]string{
		"provider.auth_token":    "token-value",
		"provider.client_secret": "client-value",
	})
	if err != nil {
		t.Fatalf("MaterializeJSON() error = %v", err)
	}
	var result map[string]any
	if err = json.Unmarshal(materialized, &result); err != nil {
		t.Fatalf("decode materialized JSON: %v", err)
	}
	if result["auth_token"] != "token-value" {
		t.Fatalf("auth_token = %#v", result["auth_token"])
	}
	nested := result["nested"].(map[string]any)
	if nested["client_secret"] != "client-value" {
		t.Fatalf("client_secret = %#v", nested["client_secret"])
	}
}

func TestPrepareJSONRejectsModelVisibleSecrets(t *testing.T) {
	tests := map[string]json.RawMessage{
		"direct secret":          json.RawMessage(`{"auth_token":"model-saw-this"}`),
		"non-secret placeholder": json.RawMessage(`{"display_name":{"$secret":"display.name.slot"}}`),
		"short slot":             json.RawMessage(`{"password":{"$secret":"short"}}`),
		"reused slot": json.RawMessage(`{
			"password":{"$secret":"shared.secret.slot"},
			"client_secret":{"$secret":"shared.secret.slot"}
		}`),
		"credential URL": json.RawMessage(`{"base_url":"https://user:pass@example.test/v1"}`),
		"query secret":   json.RawMessage(`{"base_url":"https://example.test/v1?api_key=value"}`),
	}
	for name, input := range tests {
		t.Run(name, func(t *testing.T) {
			if _, _, err := PrepareJSON(input); err == nil {
				t.Fatalf("PrepareJSON(%s) unexpectedly succeeded", input)
			}
		})
	}
}

func TestMaterializeJSONRequiresExactSlots(t *testing.T) {
	template := json.RawMessage(`{"password":{"$secret":"password.slot"}}`)
	tests := map[string]map[string]string{
		"missing": nil,
		"empty":   {"password.slot": " "},
		"extra":   {"password.slot": "value", "unexpected.slot": "value"},
	}
	for name, values := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := MaterializeJSON(template, values); err == nil {
				t.Fatal("MaterializeJSON() unexpectedly succeeded")
			}
		})
	}
}

func TestPrepareJSONPreservesExtensibleConfigurationTypesAndRequiresSecretLeaves(t *testing.T) {
	template := json.RawMessage(`{
		"mcp_servers": {
			"local": {
				"type": "stdio",
				"command": "npx",
				"args": ["-y", "@example/mcp"],
				"env": {
					"PATH": {"$secret": "mcp.env.path"}
				}
			},
			"remote": {
				"type": "http",
				"url": "https://mcp.example.test/rpc",
				"headers": {
					"X-API-Version": {"$secret": "mcp.header.version"}
				},
				"oauth": {
					"clientId": "public-client-id",
					"callbackPort": 43123,
					"xaa": true
				}
			}
		},
		"provider_options": {
			"temperature": 0.25,
			"stream": true,
			"thinking": {"type": "enabled", "budget_tokens": 4096},
			"api_key": {"$secret": "provider.option.api_key"}
		}
	}`)

	prepared, slots, err := PrepareJSON(template)
	if err != nil {
		t.Fatalf("PrepareJSON() error = %v", err)
	}
	if len(slots) != 3 {
		t.Fatalf("slots = %#v, want three sensitive leaves", slots)
	}
	var decoded map[string]any
	if err = json.Unmarshal(prepared, &decoded); err != nil {
		t.Fatalf("decode prepared JSON: %v", err)
	}
	mcpServers := decoded["mcp_servers"].(map[string]any)
	local := mcpServers["local"].(map[string]any)
	if local["type"] != "stdio" || local["command"] != "npx" {
		t.Fatalf("stdio structural fields changed type/value: %#v", local)
	}
	args := local["args"].([]any)
	if len(args) != 2 || args[0] != "-y" || args[1] != "@example/mcp" {
		t.Fatalf("stdio args changed type/value: %#v", args)
	}
	remote := mcpServers["remote"].(map[string]any)
	oauth := remote["oauth"].(map[string]any)
	if oauth["clientId"] != "public-client-id" ||
		oauth["callbackPort"] != float64(43123) ||
		oauth["xaa"] != true {
		t.Fatalf("OAuth structural fields changed type/value: %#v", oauth)
	}
	providerOptions := decoded["provider_options"].(map[string]any)
	if providerOptions["temperature"] != 0.25 || providerOptions["stream"] != true {
		t.Fatalf("provider options lost mixed JSON types: %#v", providerOptions)
	}
	thinking := providerOptions["thinking"].(map[string]any)
	if thinking["type"] != "enabled" || thinking["budget_tokens"] != float64(4096) {
		t.Fatalf("nested provider options changed: %#v", thinking)
	}

	materialized, err := MaterializeJSON(template, map[string]string{
		"mcp.env.path":            "/safe/bin",
		"mcp.header.version":      "2026-07",
		"provider.option.api_key": "provider-secret",
	})
	if err != nil {
		t.Fatalf("MaterializeJSON() error = %v", err)
	}
	if !strings.Contains(string(materialized), `"callbackPort":43123`) ||
		!strings.Contains(string(materialized), `"xaa":true`) ||
		!strings.Contains(string(materialized), `"temperature":0.25`) {
		t.Fatalf("materialized payload stringified structural values: %s", materialized)
	}

	for name, direct := range map[string]json.RawMessage{
		"mcp env":          json.RawMessage(`{"mcp_servers":{"local":{"command":"npx","env":{"PATH":"/seen/by/model"}}}}`),
		"mcp header":       json.RawMessage(`{"mcp_servers":{"remote":{"type":"http","url":"https://mcp.example.test","headers":{"X-Version":"seen"}}}}`),
		"provider api key": json.RawMessage(`{"provider_options":{"temperature":0.2,"api_key":"seen"}}`),
	} {
		t.Run(name, func(t *testing.T) {
			if _, _, prepareErr := PrepareJSON(direct); prepareErr == nil {
				t.Fatalf("PrepareJSON(%s) accepted a direct sensitive leaf", direct)
			}
		})
	}
}

func TestConfigurationToolProjectionKeepsSlotsAndRedactsDirectValues(t *testing.T) {
	input := map[string]any{
		"domain": "providers",
		"input": map[string]any{
			"auth_token":   map[string]any{"$secret": "provider.auth_token"},
			"password":     "must-not-leak",
			"display_name": "Example",
		},
	}
	redacted := RedactConfigurationToolInput(
		"mcp__nexus_config__apply_nexus_configuration_change",
		input,
	)
	payload, err := json.Marshal(redacted)
	if err != nil {
		t.Fatalf("marshal redacted input: %v", err)
	}
	if strings.Contains(string(payload), "must-not-leak") {
		t.Fatalf("redacted payload leaked direct secret: %s", payload)
	}
	if !strings.Contains(string(payload), "provider.auth_token") {
		t.Fatalf("redacted payload lost opaque slot metadata: %s", payload)
	}
	structural := RedactConfigurationToolInput(
		"mcp__nexus_config__plan_nexus_configuration_change",
		map[string]any{
			"input": map[string]any{
				"mcp_servers": map[string]any{
					"remote": map[string]any{
						"type": "http",
						"url":  "https://mcp.example.test",
						"oauth": map[string]any{
							"callbackPort": 43123,
							"xaa":          true,
						},
						"headers": map[string]any{"Authorization": "must-not-leak"},
					},
				},
			},
		},
	)
	structuralPayload, err := json.Marshal(structural)
	if err != nil {
		t.Fatalf("marshal structural projection: %v", err)
	}
	if strings.Contains(string(structuralPayload), "must-not-leak") ||
		!strings.Contains(string(structuralPayload), `"callbackPort":43123`) ||
		!strings.Contains(string(structuralPayload), `"xaa":true`) {
		t.Fatalf("tool projection did not preserve structure/redact leaves: %s", structuralPayload)
	}
	safeInput := map[string]any{
		"domain": "providers",
		"input": map[string]any{
			"auth_token":   map[string]any{"$secret": "provider.auth_token"},
			"display_name": "Example",
		},
	}
	if len(SlotsFromToolInput(
		"mcp__nexus_config__apply_nexus_configuration_change",
		safeInput,
	)) != 1 {
		t.Fatal("configuration slot was not recovered from safe tool input")
	}
}
