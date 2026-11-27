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

	"github.com/nexus-research-lab/nexus/internal/config"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	preferencessvc "github.com/nexus-research-lab/nexus/internal/service/preferences"
	"github.com/nexus-research-lab/nexus/internal/storage"
	"github.com/pressly/goose/v3"
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
