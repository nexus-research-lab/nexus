// configuration_test.go 覆盖配置服务目录、租约、审计、人工审批、偏好变更、修订与脱敏单元测试。
package configuration

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/secretinput"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	"github.com/nexus-research-lab/nexus/internal/runtime/clientopts"
	preferencessvc "github.com/nexus-research-lab/nexus/internal/service/preferences"
)

func TestRuntimeCapabilityBindsStableSessionToActiveRound(t *testing.T) {
	manager := runtimectx.NewManager()
	service := &Service{
		runtime:                    manager,
		runtimeCapabilities:        make(map[string]*runtimeCapabilityRecord),
		runtimeCapabilityBySession: make(map[string]string),
		runtimeCapabilityNow:       func() time.Time { return time.Now().UTC() },
	}
	actor := Actor{
		OwnerUserID: "owner-a", AgentID: "agent-a",
		LeaseSessionKey: "session-a", LeaseRoundID: "round-a",
		RoundLeaseRequired: true,
	}
	token, err := service.IssueRuntimeCapability(actor)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.ResolveRuntimeCapability(token); err == nil ||
		!strings.Contains(err.Error(), "尚未开始") {
		t.Fatalf("inactive capability error = %v", err)
	}
	if err = manager.StartRound(context.Background(), actor.LeaseSessionKey, actor.LeaseRoundID, nil); err != nil {
		t.Fatal(err)
	}
	resolved, err := service.ResolveRuntimeCapability(token)
	if err != nil || resolved.AgentID != actor.AgentID {
		t.Fatalf("resolved actor = %+v, err=%v", resolved, err)
	}

	next := actor
	next.LeaseRoundID = "round-b"
	nextToken, err := service.IssueRuntimeCapability(next)
	if err != nil {
		t.Fatal(err)
	}
	if nextToken != token {
		t.Fatalf("same runtime session rotated token: %q != %q", nextToken, token)
	}
	if err = manager.StartRound(context.Background(), next.LeaseSessionKey, next.LeaseRoundID, nil); err != nil {
		t.Fatal(err)
	}
	if _, err = service.ResolveRuntimeCapability(token); err == nil ||
		!strings.Contains(err.Error(), "并发 round") {
		t.Fatalf("concurrent rounds must fail closed, err=%v", err)
	}
	manager.MarkRoundFinished(actor.LeaseSessionKey, actor.LeaseRoundID)
	resolved, err = service.ResolveRuntimeCapability(token)
	if err != nil || resolved.LeaseRoundID != next.LeaseRoundID {
		t.Fatalf("successor actor = %+v, err=%v", resolved, err)
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

func TestUpdatePreferencesRestoresOnlyWrittenWebSearchVersion(t *testing.T) {
	preferences := preferencessvc.NewService(config.Config{
		WorkspacePath: filepath.Join(t.TempDir(), "workspace"),
	})
	service := &Service{prefs: preferences}
	const ownerID = "owner-configuration-rollback"

	initial, err := preferences.Get(context.Background(), ownerID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.updatePreferences(
		context.Background(),
		Actor{OwnerUserID: ownerID},
		preferencessvc.UpdateRequest{
			WebSearch: &preferencessvc.WebSearchSettings{DefaultCount: 9},
		},
		json.RawMessage(`{"web_search":{"default_count":9}}`),
		initial.Version,
	)
	if err == nil {
		t.Fatal("缺少 runtime 热同步依赖时应失败并回滚")
	}
	stored, readErr := preferences.Get(context.Background(), ownerID)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if stored.WebSearch.DefaultCount != initial.WebSearch.DefaultCount {
		t.Fatalf("WebSearch 热同步失败后未恢复旧值: %+v", stored.WebSearch)
	}
	if stored.Version != initial.Version+2 {
		t.Fatalf("写入和条件回滚应各推进一次 version: got=%d want=%d", stored.Version, initial.Version+2)
	}
}

func TestRedactInputSecretsFromExecutionError(t *testing.T) {
	input := json.RawMessage(`{"credentials":{"token":"secret-value"},"name":"safe"}`)
	err := redactInputSecrets(errors.New("remote rejected secret-value for safe"), input)
	if strings.Contains(err.Error(), "secret-value") {
		t.Fatalf("error leaked input secret: %v", err)
	}
	if !strings.Contains(err.Error(), "safe") {
		t.Fatalf("non-secret context should remain: %v", err)
	}
}
