package configuration

import (
	"context"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/storage"
	"github.com/pressly/goose/v3"
)

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

func TestFinishAuditSurvivesCallerCancellation(t *testing.T) {
	service, actor, resolved := newAuditTestService(t)
	request := ChangeRequest{
		RequestID: "request-cancelled-1", Domain: DomainPreferences, Operation: "update",
		Input: []byte(`{"web_search_api_key":"must-not-leak"}`),
	}
	plan := ChangePlan{
		Domain: DomainPreferences, Operation: "update", CurrentRevision: "before",
		Scope: ScopeRef{Kind: ScopeKindOwner, ID: actor.OwnerUserID}, PlanDigest: "intent",
	}
	if _, created, err := service.beginAudit(t.Context(), resolved, request, plan, nil); err != nil || !created {
		t.Fatalf("beginAudit created=%v err=%v", created, err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := service.finishAudit(ctx, actor, request.RequestID, "success", map[string]any{
		"secret": "must-not-leak",
	}, "after", nil); err != nil {
		t.Fatalf("finishAudit should detach from caller cancellation: %v", err)
	}
	record, err := service.auditByID(t.Context(), actor.OwnerUserID, request.RequestID)
	if err != nil {
		t.Fatal(err)
	}
	if record == nil || record.Status != "success" || record.RevisionAfter != "after" {
		t.Fatalf("audit record = %+v", record)
	}
	if containsText(string(record.Result), "must-not-leak") {
		t.Fatalf("audit result leaked secret: %s", record.Result)
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
