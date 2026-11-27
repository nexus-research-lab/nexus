// configuration_test.go 覆盖配置服务目录、租约、审计、人工审批、偏好变更、修订与脱敏单元测试。
package configuration

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/infra/secretinput"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	"github.com/nexus-research-lab/nexus/internal/runtime/clientopts"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	preferencessvc "github.com/nexus-research-lab/nexus/internal/service/preferences"
	"github.com/nexus-research-lab/nexus/internal/storage"
	"github.com/pressly/goose/v3"
)

func TestRequireActiveRoundUsesDMAndRoomRuntimeLease(t *testing.T) {
	tests := []struct {
		name            string
		sessionKey      string
		roundID         string
		leaseSessionKey string
		leaseRoundID    string
	}{
		{
			name: "dm",
			sessionKey: protocol.BuildAgentSessionKey(
				"agent-a", protocol.SessionChannelWebSocketSegment, protocol.RoomTypeDM, "main", "",
			),
			roundID: "dm-round-a",
			leaseSessionKey: protocol.BuildAgentSessionKey(
				"agent-a", protocol.SessionChannelWebSocketSegment, protocol.RoomTypeDM, "main", "",
			),
			leaseRoundID: "dm-round-a",
		},
		{
			name:       "room agent slot",
			sessionKey: protocol.BuildRoomSharedSessionKey("conversation-a"),
			roundID:    "root-round-a",
			leaseSessionKey: protocol.BuildRoomAgentSessionKey(
				"conversation-a", "agent-a", protocol.RoomTypeGroup,
			),
			leaseRoundID: "agent-round-a",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			manager := runtimectx.NewManager()
			if err := manager.StartRound(t.Context(), test.leaseSessionKey, test.leaseRoundID, nil); err != nil {
				t.Fatalf("start runtime lease: %v", err)
			}
			service := &Service{runtime: manager}
			actor := Actor{
				SessionKey: test.sessionKey, RoundID: test.roundID,
				LeaseSessionKey: test.leaseSessionKey, LeaseRoundID: test.leaseRoundID,
				RoundLeaseRequired: true,
			}
			if err := service.requireActiveRound(actor); err != nil {
				t.Fatalf("active lease rejected: %v", err)
			}

			stale := actor
			stale.LeaseRoundID += "-stale"
			if err := service.requireActiveRound(stale); err == nil ||
				!strings.Contains(err.Error(), "已结束") {
				t.Fatalf("stale lease error = %v", err)
			}

			manager.MarkRoundFinished(test.leaseSessionKey, test.leaseRoundID)
			if err := service.requireActiveRound(actor); err == nil ||
				!strings.Contains(err.Error(), "已结束") {
				t.Fatalf("finished lease error = %v", err)
			}
		})
	}
}

func TestValidateAgentRuntimeIdentityRejectsTransferAcrossDMs(t *testing.T) {
	dmSession := protocol.BuildAgentSessionKey(
		"agent-a", protocol.SessionChannelWebSocketSegment, protocol.RoomTypeDM, "dm-a", "",
	)
	actor := Actor{
		AgentID: "agent-a", SessionKey: dmSession, RoundID: "round-a",
		LeaseSessionKey: dmSession, LeaseRoundID: "round-a",
		RoundLeaseRequired: true,
	}
	if err := validateAgentRuntimeIdentity(actor); err != nil {
		t.Fatalf("valid DM identity rejected: %v", err)
	}

	swappedSession := actor
	swappedSession.LeaseSessionKey = protocol.BuildAgentSessionKey(
		"agent-a", protocol.SessionChannelWebSocketSegment, protocol.RoomTypeDM, "dm-b", "",
	)
	if err := validateAgentRuntimeIdentity(swappedSession); err == nil ||
		!strings.Contains(err.Error(), "不匹配") {
		t.Fatalf("swapped DM lease error = %v", err)
	}

	external := actor
	external.SessionKey = protocol.BuildAgentSessionKey(
		"agent-a", "telegram", protocol.RoomTypeDM, "chat-a", "",
	)
	external.LeaseSessionKey = external.SessionKey
	if err := validateAgentRuntimeIdentity(external); err == nil ||
		!strings.Contains(err.Error(), "WebSocket") {
		t.Fatalf("external channel lease error = %v", err)
	}
}

func TestValidateRoomRuntimeIdentityRejectsTransferAcrossAgentSlots(t *testing.T) {
	contextValue := &protocol.ConversationContextAggregate{
		Room: protocol.RoomRecord{ID: "room-a", RoomType: protocol.RoomTypeGroup},
		Conversation: protocol.ConversationRecord{
			ID: "conversation-a", RoomID: "room-a",
		},
	}
	actor := Actor{
		AgentID: "agent-a", ConversationID: "conversation-a",
		SessionKey: protocol.BuildRoomSharedSessionKey("conversation-a"),
		LeaseSessionKey: protocol.BuildRoomAgentSessionKey(
			"conversation-a", "agent-a", protocol.RoomTypeGroup,
		),
		RoundLeaseRequired: true,
	}
	if err := validateRoomRuntimeIdentityValue(actor, "room-a", contextValue); err != nil {
		t.Fatalf("valid Room identity rejected: %v", err)
	}

	swappedAgent := actor
	swappedAgent.LeaseSessionKey = protocol.BuildRoomAgentSessionKey(
		"conversation-a", "agent-b", protocol.RoomTypeGroup,
	)
	if err := validateRoomRuntimeIdentityValue(swappedAgent, "room-a", contextValue); err == nil ||
		!strings.Contains(err.Error(), "Agent slot") {
		t.Fatalf("swapped Room Agent lease error = %v", err)
	}

	swappedConversation := actor
	swappedConversation.LeaseSessionKey = protocol.BuildRoomAgentSessionKey(
		"conversation-b", "agent-a", protocol.RoomTypeGroup,
	)
	if err := validateRoomRuntimeIdentityValue(swappedConversation, "room-a", contextValue); err == nil ||
		!strings.Contains(err.Error(), "Agent slot") {
		t.Fatalf("swapped Room conversation lease error = %v", err)
	}
}

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

func TestBeginAuditAllowsOnlyOneExecutorForRequestID(t *testing.T) {
	cfg := config.Config{
		DatabaseDriver: "sqlite",
		DatabaseURL:    filepath.Join(t.TempDir(), "nexus.db"),
	}
	db, err := storage.OpenDB(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err = goose.SetDialect("sqlite3"); err != nil {
		t.Fatal(err)
	}
	if err = goose.Up(db, "../../../db/migrations/sqlite"); err != nil {
		t.Fatal(err)
	}
	service := NewService(cfg, db, nil, nil, nil, nil, nil, nil, nil)
	actor := Actor{
		OwnerUserID: "owner", AgentID: "nexus", IsMainAgent: true,
		SessionKey: "room:business", RoundID: "root-round",
		LeaseSessionKey: "room:business:nexus", LeaseRoundID: "agent-round",
	}
	resolved := &resolvedActor{
		Actor: actor, Authority: AuthorityOwnerMain,
		Context: ScopeRef{Kind: ScopeKindOwner, ID: actor.OwnerUserID},
	}
	request := ChangeRequest{
		RequestID: "request-concurrent-1", Domain: DomainPreferences, Operation: "update",
		Input: []byte(`{"web_search_api_key":"must-not-leak"}`),
	}
	plan := ChangePlan{
		Domain: DomainPreferences, Operation: "update", CurrentRevision: "sha256:before",
		Scope: ScopeRef{Kind: ScopeKindOwner, ID: actor.OwnerUserID}, PlanDigest: "sha256:intent",
	}

	start := make(chan struct{})
	created := make(chan bool, 2)
	errs := make(chan error, 2)
	var wait sync.WaitGroup
	for range 2 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			_, won, beginErr := service.beginAudit(t.Context(), resolved, request, plan, nil)
			created <- won
			errs <- beginErr
		}()
	}
	close(start)
	wait.Wait()
	close(created)
	close(errs)
	winners := 0
	for won := range created {
		if won {
			winners++
		}
	}
	for beginErr := range errs {
		if beginErr != nil {
			t.Fatalf("beginAudit error: %v", beginErr)
		}
	}
	if winners != 1 {
		t.Fatalf("audit executors = %d, want 1", winners)
	}
	record, err := service.auditByID(t.Context(), actor.OwnerUserID, request.RequestID)
	if err != nil {
		t.Fatal(err)
	}
	if record == nil || record.Status != "applying" {
		t.Fatalf("audit record = %+v", record)
	}
	if record.SessionKey != actor.SessionKey || record.RoundID != actor.RoundID ||
		record.LeaseSessionKey != actor.LeaseSessionKey ||
		record.LeaseRoundID != actor.LeaseRoundID {
		t.Fatalf("audit did not bind business and runtime lease identity: %+v", record)
	}
	if string(record.Request) == "" || containsText(string(record.Request), "must-not-leak") {
		t.Fatalf("audit request leaked secret: %s", record.Request)
	}
}

func TestReplayRecoversExpiredApplyingAudit(t *testing.T) {
	service, actor, resolved := newAuditTestService(t)
	request := ChangeRequest{
		RequestID: "request-expired-1", Domain: DomainPreferences, Operation: "update",
		Input: []byte(`{"chat_default_delivery_policy":"queue"}`),
	}
	plan := ChangePlan{
		Domain: DomainPreferences, Operation: "update", CurrentRevision: "before",
		Scope: ScopeRef{Kind: ScopeKindOwner, ID: actor.OwnerUserID}, PlanDigest: "intent",
	}
	if _, created, err := service.beginAudit(t.Context(), resolved, request, plan, nil); err != nil || !created {
		t.Fatalf("beginAudit created=%v err=%v", created, err)
	}
	if _, err := service.db.ExecContext(
		t.Context(),
		`UPDATE configuration_changes SET updated_at = datetime('now', '-10 minutes')
		 WHERE owner_user_id = ? AND request_id = ?`,
		actor.OwnerUserID,
		request.RequestID,
	); err != nil {
		t.Fatal(err)
	}
	record, err := service.auditByID(t.Context(), actor.OwnerUserID, request.RequestID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.replayOrRecover(t.Context(), actor, record); err == nil ||
		!strings.Contains(err.Error(), "reconcile") {
		t.Fatalf("expired applying replay error = %v", err)
	}
	record, err = service.auditByID(t.Context(), actor.OwnerUserID, request.RequestID)
	if err != nil {
		t.Fatal(err)
	}
	if record == nil || record.Status != "reconcile_required" {
		t.Fatalf("recovered audit record = %+v", record)
	}
}

func TestRecoverStaleApplyingChangesForAllOwnersIsBoundedAndScoped(t *testing.T) {
	service, _, _ := newAuditTestService(t)
	for _, ownerUserID := range []string{"owner-a", "owner-b", "owner-c"} {
		actor := Actor{OwnerUserID: ownerUserID, AgentID: "nexus", IsMainAgent: true}
		resolved := &resolvedActor{
			Actor: actor, Authority: AuthorityOwnerMain,
			Context: ScopeRef{Kind: ScopeKindOwner, ID: ownerUserID},
		}
		request := ChangeRequest{
			RequestID: "request-recovery-" + ownerUserID, Domain: DomainPreferences, Operation: "update",
			Input: []byte(`{"chat_default_delivery_policy":"queue"}`),
		}
		plan := ChangePlan{
			Domain: DomainPreferences, Operation: "update", CurrentRevision: "before",
			Scope: ScopeRef{Kind: ScopeKindOwner, ID: ownerUserID}, PlanDigest: "intent-" + ownerUserID,
		}
		if _, created, err := service.beginAudit(t.Context(), resolved, request, plan, nil); err != nil || !created {
			t.Fatalf("owner %s beginAudit created=%v err=%v", ownerUserID, created, err)
		}
		if _, err := service.db.ExecContext(
			t.Context(),
			`UPDATE configuration_changes SET updated_at = datetime('now', '-10 minutes')
			 WHERE owner_user_id = ? AND request_id = ?`,
			ownerUserID, request.RequestID,
		); err != nil {
			t.Fatal(err)
		}
	}

	recovered, err := service.RecoverStaleApplyingChangesForAllOwners(t.Context(), 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(recovered) != 2 {
		t.Fatalf("bounded recovery returned %d records, want 2: %+v", len(recovered), recovered)
	}
	for _, ownerUserID := range []string{"owner-a", "owner-b"} {
		var status string
		if err := service.db.QueryRowContext(
			t.Context(),
			`SELECT status FROM configuration_changes WHERE owner_user_id = ?`, ownerUserID,
		).Scan(&status); err != nil {
			t.Fatal(err)
		}
		if status != "reconcile_required" {
			t.Fatalf("owner %s status = %s, want reconcile_required", ownerUserID, status)
		}
	}
	var pending int
	if err := service.db.QueryRowContext(
		t.Context(),
		`SELECT COUNT(*) FROM configuration_changes WHERE owner_user_id = ? AND status = 'applying'`, "owner-c",
	).Scan(&pending); err != nil {
		t.Fatal(err)
	}
	if pending != 1 {
		t.Fatalf("owner-c pending applying count = %d, want 1", pending)
	}

	recovered, err = service.RecoverStaleApplyingChangesForAllOwners(t.Context(), 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(recovered) != 1 || recovered[0].OwnerUserID != "owner-c" {
		t.Fatalf("second recovery returned %+v, want owner-c only", recovered)
	}
}

func newAuditTestService(t *testing.T) (*Service, Actor, *resolvedActor) {
	t.Helper()
	cfg := config.Config{
		DatabaseDriver: "sqlite",
		DatabaseURL:    filepath.Join(t.TempDir(), "nexus.db"),
	}
	db, err := storage.OpenDB(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err = goose.SetDialect("sqlite3"); err != nil {
		t.Fatal(err)
	}
	if err = goose.Up(db, "../../../db/migrations/sqlite"); err != nil {
		t.Fatal(err)
	}
	service := NewService(cfg, db, nil, nil, nil, nil, nil, nil, nil)
	actor := Actor{OwnerUserID: "owner", AgentID: "nexus", IsMainAgent: true}
	return service, actor, &resolvedActor{
		Actor: actor, Authority: AuthorityOwnerMain,
		Context: ScopeRef{Kind: ScopeKindOwner, ID: actor.OwnerUserID},
	}
}

func containsText(value, needle string) bool {
	for index := 0; index+len(needle) <= len(value); index++ {
		if value[index:index+len(needle)] == needle {
			return true
		}
	}
	return false
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

func TestConsumeHumanApprovalIsBoundExpiringAndOneTime(t *testing.T) {
	now := time.Date(2026, time.July, 30, 12, 0, 0, 0, time.UTC)
	service := &Service{
		humanApprovals: make(map[string]humanApprovalRecord),
		approvalNow:    func() time.Time { return now },
	}
	actor := &resolvedActor{
		Actor: Actor{
			OwnerUserID: "owner-a", AgentID: "agent-a",
			SessionKey: "session-a", RoundID: "round-a",
			LeaseSessionKey: "runtime-session-a", LeaseRoundID: "runtime-round-a",
			ContextKind: ContextKindAgent, ContextID: "agent-a",
			PrincipalRole: "owner", AuthMethod: "password",
		},
		Authority: AuthorityAgentSelf,
		Context:   ScopeRef{Kind: ScopeKindAgent, ID: "agent-a"},
	}
	request := ChangeRequest{
		RequestID: "approval-once-01",
		Domain:    DomainAgents, Operation: "update_self_profile",
		PlanDigest: "hmac:plan", ExpectedRevision: "sha256:before",
	}
	plan := ChangePlan{
		Domain: DomainAgents, Operation: "update_self_profile",
		Scope:      ScopeRef{Kind: ScopeKindAgent, ID: "agent-a"},
		PlanDigest: "hmac:plan", CurrentRevision: "sha256:before",
	}
	key := humanApprovalKey(actor.OwnerUserID, request.RequestID)
	record := humanApprovalRecord{
		PermissionRequestID: "perm-approval-once-01",
		OwnerUserID:         actor.OwnerUserID, PrincipalRole: actor.PrincipalRole,
		PrincipalAuthMethod: actor.AuthMethod,
		AgentID:             actor.AgentID, SessionKey: actor.SessionKey, RoundID: actor.RoundID,
		LeaseSessionKey: actor.LeaseSessionKey, LeaseRoundID: actor.LeaseRoundID,
		ContextKind: actor.ContextKind, ContextID: actor.ContextID,
		Scope: plan.Scope, RequestID: request.RequestID,
		PlanDigest: plan.PlanDigest, ExpectedRevision: plan.CurrentRevision,
		Domain: plan.Domain, Operation: plan.Operation, Target: plan.Target,
		ApprovedAt: now.Add(-time.Second), ExpiresAt: now.Add(time.Minute),
	}

	expired := record
	expired.ExpiresAt = now
	service.humanApprovals[key] = expired
	if _, err := service.consumeHumanApproval(actor, request, plan); err == nil ||
		!strings.Contains(err.Error(), "尚未获得") {
		t.Fatalf("expired approval error = %v", err)
	}
	if _, exists := service.humanApprovals[key]; exists {
		t.Fatal("expired approval was not purged")
	}

	wrongSession := record
	wrongSession.SessionKey = "session-b"
	service.humanApprovals[key] = wrongSession
	if _, err := service.consumeHumanApproval(actor, request, plan); err == nil ||
		!strings.Contains(err.Error(), "不匹配") {
		t.Fatalf("cross-session approval error = %v", err)
	}
	if _, exists := service.humanApprovals[key]; !exists {
		t.Fatal("mismatched caller consumed another session's approval")
	}

	wrongLease := record
	wrongLease.LeaseRoundID = "runtime-round-b"
	service.humanApprovals[key] = wrongLease
	if _, err := service.consumeHumanApproval(actor, request, plan); err == nil ||
		!strings.Contains(err.Error(), "不匹配") {
		t.Fatalf("cross-lease approval error = %v", err)
	}
	if _, exists := service.humanApprovals[key]; !exists {
		t.Fatal("mismatched caller consumed another runtime lease's approval")
	}

	service.humanApprovals[key] = record
	consumed, err := service.consumeHumanApproval(actor, request, plan)
	if err != nil {
		t.Fatalf("consume valid approval: %v", err)
	}
	if consumed == nil || consumed.PermissionRequestID != record.PermissionRequestID {
		t.Fatalf("consumed approval = %+v", consumed)
	}
	if _, exists := service.humanApprovals[key]; exists {
		t.Fatal("valid approval was not consumed")
	}
	if _, err = service.consumeHumanApproval(actor, request, plan); err == nil {
		t.Fatal("one-time approval was replayed")
	}
}

func TestConfigurationApplyApprovalToolNameIsExactLeaf(t *testing.T) {
	for _, allowed := range []string{
		"apply_nexus_configuration_change",
		"mcp__nexus_config__apply_nexus_configuration_change",
		"nexus_config.apply_nexus_configuration_change",
		"nexus_config/apply_nexus_configuration_change",
	} {
		if !isConfigurationApplyApprovalTool(allowed) {
			t.Fatalf("expected configuration apply tool %q to be accepted", allowed)
		}
	}
	for _, rejected := range []string{
		"",
		"apply_nexus_configuration_change_extra",
		"forgedapply_nexus_configuration_change",
		"mcp__nexus_config__plan_nexus_configuration_change",
	} {
		if isConfigurationApplyApprovalTool(rejected) {
			t.Fatalf("forged configuration approval tool %q was accepted", rejected)
		}
	}
}

func TestActorFromHumanApprovalKeepsRoomBusinessAndLeaseIdentitySeparate(t *testing.T) {
	service := &Service{}
	ctx := authctx.WithPrincipal(context.Background(), &authctx.Principal{
		UserID: "owner-a", Role: authctx.RoleOwner, AuthMethod: authctx.AuthMethodPassword,
	})

	dmActor, err := service.actorFromHumanApproval(ctx, permissionctx.HumanToolApproval{
		RuntimeSessionKey:  "agent:agent-a:dm:main",
		DispatchSessionKey: "agent:agent-a:dm:main",
		Route: permissionctx.RouteContext{
			AgentID: "agent-a", RoundID: "dm-round-a",
		},
	})
	if err != nil {
		t.Fatalf("DM approval actor: %v", err)
	}
	if dmActor.SessionKey != dmActor.LeaseSessionKey ||
		dmActor.RoundID != dmActor.LeaseRoundID {
		t.Fatalf("DM business and lease identity must match: %+v", dmActor)
	}

	roomActor, err := service.actorFromHumanApproval(ctx, permissionctx.HumanToolApproval{
		RuntimeSessionKey:  "room:room-a:conversation-a:agent-a",
		DispatchSessionKey: "room:room-a:conversation-a",
		Route: permissionctx.RouteContext{
			AgentID: "agent-a", RoundID: "root-round-a", AgentRoundID: "agent-round-a",
			RoomID: "room-a", ConversationID: "conversation-a",
		},
	})
	if err != nil {
		t.Fatalf("Room approval actor: %v", err)
	}
	if roomActor.SessionKey != "room:room-a:conversation-a" ||
		roomActor.RoundID != "root-round-a" ||
		roomActor.LeaseSessionKey != "room:room-a:conversation-a:agent-a" ||
		roomActor.LeaseRoundID != "agent-round-a" {
		t.Fatalf("Room business and lease identity were conflated: %+v", roomActor)
	}

	_, err = service.actorFromHumanApproval(ctx, permissionctx.HumanToolApproval{
		RuntimeSessionKey:  "room:room-a:conversation-a:agent-a",
		DispatchSessionKey: "room:room-a:conversation-a",
		Route: permissionctx.RouteContext{
			AgentID: "agent-a", RoundID: "root-round-a", RoomID: "room-a",
		},
	})
	if err == nil || !strings.Contains(err.Error(), "lease") {
		t.Fatalf("Room approval without Agent slot round must fail closed: %v", err)
	}
}

func TestUpdatePreferencesMergesInsideLatestVersionBoundary(t *testing.T) {
	preferences := preferencessvc.NewService(config.Config{
		WorkspacePath: filepath.Join(t.TempDir(), "workspace"),
	})
	service := &Service{prefs: preferences}
	const ownerID = "owner-configuration-preferences"

	diagnosticsEnabled := true
	uiValue, err := preferences.Update(
		context.Background(),
		ownerID,
		preferencessvc.UpdateRequest{AgentSDKDiagnosticsEnabled: &diagnosticsEnabled},
	)
	if err != nil {
		t.Fatal(err)
	}
	rawInput := json.RawMessage(`{"chat_default_delivery_policy":"interrupt"}`)
	updated, err := service.updatePreferences(
		context.Background(),
		Actor{OwnerUserID: ownerID},
		preferencessvc.UpdateRequest{
			ChatDefaultDeliveryPolicy: policyPointer(protocol.ChatDeliveryPolicyInterrupt),
		},
		rawInput,
		uiValue.Version,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !updated.AgentSDKDiagnosticsEnabled ||
		updated.ChatDefaultDeliveryPolicy != protocol.ChatDeliveryPolicyInterrupt {
		t.Fatalf("对话 merge 覆盖了 UI 字段: %+v", updated)
	}
	if updated.Version != uiValue.Version+1 {
		t.Fatalf("configuration Preferences version = %d, want %d", updated.Version, uiValue.Version+1)
	}

	_, err = service.updatePreferences(
		context.Background(),
		Actor{OwnerUserID: ownerID},
		preferencessvc.UpdateRequest{
			ChatDefaultDeliveryPolicy: policyPointer(protocol.ChatDeliveryPolicyQueue),
		},
		json.RawMessage(`{"chat_default_delivery_policy":"queue"}`),
		uiValue.Version,
	)
	if !errors.Is(err, preferencessvc.ErrVersionConflict) {
		t.Fatalf("陈旧 configuration CAS error = %v", err)
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

func policyPointer(value protocol.ChatDeliveryPolicy) *protocol.ChatDeliveryPolicy {
	return &value
}

func TestConfigurationRevisionBindsScopeAndSecrets(t *testing.T) {
	service, _, _ := newAuditTestService(t)
	scope := ScopeRef{Kind: ScopeKindOwner, ID: "owner-one"}
	values := map[string]any{"api_key": "secret-one"}
	before, err := service.snapshotRevision(t.Context(), DomainProviders, scope, "provider", 1, values)
	if err != nil {
		t.Fatal(err)
	}
	for _, scenario := range []string{"scope", "domain", "target", "version", "secret"} {
		t.Run(scenario, func(t *testing.T) {
			domain, currentScope, target, version := DomainProviders, scope, "provider", int64(1)
			currentValues := map[string]any{"api_key": "secret-one"}
			switch scenario {
			case "scope":
				currentScope.ID = "owner-two"
			case "domain":
				domain = DomainPreferences
			case "target":
				target = "another-provider"
			case "version":
				version++
			case "secret":
				currentValues["api_key"] = "secret-two"
			}
			after, err := service.snapshotRevision(t.Context(), domain, currentScope, target, version, currentValues)
			if err != nil {
				t.Fatal(err)
			}
			if after == before || !strings.HasPrefix(after, durableRevisionPrefix) || strings.Contains(after, "secret") {
				t.Fatal("revision did not bind the changed identity/state without exposing the secret")
			}
		})
	}
}

func TestConfigurationRevisionLegacyReceiptIsIncomparable(t *testing.T) {
	current := DomainSnapshot{Revision: durableRevisionPrefix + strings.Repeat("a", 64)}
	record := AuditRecord{RevisionBefore: "hmac-sha256:legacy-process-key", Status: "reconcile_required"}
	if got := reconciliationEvidence(record, current); got.RevisionRelation != "incomparable" || got.DecisionSource != "human_confirmation_required" {
		t.Fatalf("legacy evidence = %+v", got)
	}
	record.RevisionAfter = current.Revision
	if got := reconciliationEvidence(record, current); got.RevisionRelation != "matches_recorded_after" {
		t.Fatalf("current-format after revision was not usable: %+v", got)
	}
	record.RevisionAfter = durableRevisionPrefix + strings.Repeat("b", 64)
	if got := reconciliationEvidence(record, current); got.RevisionRelation != "different" {
		t.Fatalf("current-format difference was hidden: %+v", got)
	}
}

func TestSanitizeValueRecognizesCamelCaseSecretKeys(t *testing.T) {
	cases := map[string]string{
		"DesktopSessionToken":            "desktop-session-secret",
		"DiscordBotToken":                "discord-bot-secret",
		"TelegramBotToken":               "telegram-bot-secret",
		"ConnectorCredentialsKey":        "connector-credentials-secret",
		"ConnectorCredentialsLegacyKeys": "connector-credentials-legacy-secret",
		"ConnectorGitHubClientSecret":    "github-client-secret",
		"SigningPrivateKey":              "signing-private-secret",
	}
	for key, secret := range cases {
		t.Run(key, func(t *testing.T) {
			payload, err := json.Marshal(sanitizeValue(map[string]any{key: secret}))
			if err != nil {
				t.Fatalf("marshal sanitized %s: %v", key, err)
			}
			text := string(payload)
			if strings.Contains(text, secret) {
				t.Fatalf("sanitized payload leaked CamelCase secret %s: %s", key, text)
			}
			if !strings.Contains(text, `"configured":true`) || !strings.Contains(text, `"redacted":true`) {
				t.Fatalf("sanitized payload lost secret presence for %s: %s", key, text)
			}
		})
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

func TestHostDomainSnapshotDoesNotExposeCredentials(t *testing.T) {
	t.Setenv("NEXUS_CONFIG_DIR", t.TempDir())
	secrets := []string{
		"desktop-session-secret",
		"discord-bot-secret",
		"telegram-bot-secret",
		"connector-credentials-secret",
		"github-client-secret",
		"google-client-secret",
		"linkedin-client-secret",
		"twitter-client-secret",
		"instagram-client-secret",
		"shopify-client-secret",
	}
	cfg := config.Config{
		ProjectName:                    "safe-project",
		DesktopSessionToken:            secrets[0],
		DiscordBotToken:                secrets[1],
		TelegramBotToken:               secrets[2],
		ConnectorCredentialsKey:        secrets[3],
		ConnectorGitHubClientID:        "github-client-id",
		ConnectorGitHubClientSecret:    secrets[4],
		ConnectorGoogleClientID:        "google-client-id",
		ConnectorGoogleClientSecret:    secrets[5],
		ConnectorLinkedInClientID:      "linkedin-client-id",
		ConnectorLinkedInClientSecret:  secrets[6],
		ConnectorTwitterClientID:       "twitter-client-id",
		ConnectorTwitterClientSecret:   secrets[7],
		ConnectorInstagramClientID:     "instagram-client-id",
		ConnectorInstagramClientSecret: secrets[8],
		ConnectorShopifyClientID:       "shopify-client-id",
		ConnectorShopifyClientSecret:   secrets[9],
	}
	service, _, _ := newAuditTestService(t)
	service.cfg = cfg
	actor := &resolvedActor{
		Actor: Actor{
			OwnerUserID:     authctx.SystemUserID,
			AgentID:         "agent-safe",
			IsMainAgent:     true,
			PrincipalRole:   authctx.RoleOwner,
			AuthMethod:      authctx.AuthMethodLocal,
			LocalSingleUser: true,
		},
		Authority: AuthorityOwnerMain,
		Context:   ScopeRef{Kind: ScopeKindOwner, ID: authctx.SystemUserID},
	}

	snapshot, err := service.domainSnapshot(t.Context(), actor, DomainHost, "", false)
	if err != nil {
		t.Fatalf("build host domain snapshot: %v", err)
	}
	payload, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("marshal host domain snapshot: %v", err)
	}
	text := string(payload)
	for _, secret := range secrets {
		if strings.Contains(text, secret) {
			t.Fatalf("host domain snapshot leaked %q: %s", secret, text)
		}
	}
	if !strings.Contains(text, "safe-project") {
		t.Fatalf("host domain snapshot lost non-sensitive configuration: %s", text)
	}
	if !strings.Contains(text, `"configured":true`) || !strings.Contains(text, `"redacted":true`) {
		t.Fatalf("host domain snapshot must retain credential presence status: %s", text)
	}
}
