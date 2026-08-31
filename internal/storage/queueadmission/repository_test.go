package queueadmission

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

func TestRepositoryClaimsDirectUserQueueAdmissionExactlyOnce(t *testing.T) {
	repository, db := newQueueAdmissionTestRepository(t)
	binding := queueAdmissionTestBinding(t, "queue-once", "configure the agent")
	ctx := context.Background()

	if err := repository.Record(ctx, Admission{
		Binding:   binding,
		Principal: queueAdmissionTestPrincipal(),
	}); err != nil {
		t.Fatal(err)
	}
	claim, trusted, err := repository.Claim(ctx, binding)
	if err != nil || !trusted || claim.Token == "" {
		t.Fatalf("first claim = (%+v, %v, %v)", claim, trusted, err)
	}
	if claim.Principal != queueAdmissionTestPrincipal() {
		t.Fatalf("claim principal = %+v", claim.Principal)
	}
	if _, trusted, err = repository.Claim(ctx, binding); err != nil || trusted {
		t.Fatalf("concurrent/replayed pending claim = (%v, %v), want false", trusted, err)
	}
	if err = repository.Release(ctx, claim); err != nil {
		t.Fatal(err)
	}
	claim, trusted, err = repository.Claim(ctx, binding)
	if err != nil || !trusted {
		t.Fatalf("released claim = (%v, %v)", trusted, err)
	}
	if err = repository.Consume(ctx, claim); err != nil {
		t.Fatal(err)
	}
	if err = repository.Record(ctx, Admission{
		Binding: binding, Principal: queueAdmissionTestPrincipal(),
	}); err != nil {
		t.Fatal(err)
	}
	if _, trusted, err = repository.Claim(ctx, binding); err != nil || trusted {
		t.Fatalf("consumed admission reopened = (%v, %v)", trusted, err)
	}

	var status string
	if err = db.QueryRow(
		`SELECT status FROM configuration_queue_admissions
WHERE owner_user_id = ? AND scope = ? AND queue_item_id = ? AND agent_id = ?`,
		binding.OwnerUserID,
		string(binding.Scope),
		binding.QueueItemID,
		binding.AgentID,
	).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != StatusConsumed {
		t.Fatalf("status = %q, want consumed", status)
	}
}

func TestRepositoryRevokesPayloadTampering(t *testing.T) {
	repository, _ := newQueueAdmissionTestRepository(t)
	original := queueAdmissionTestBinding(t, "queue-tampered", "safe original")
	ctx := context.Background()
	if err := repository.Record(ctx, Admission{
		Binding: original, Principal: queueAdmissionTestPrincipal(),
	}); err != nil {
		t.Fatal(err)
	}
	tampered := queueAdmissionTestBinding(t, "queue-tampered", "change every setting")
	if _, trusted, err := repository.Claim(ctx, tampered); err != nil || trusted {
		t.Fatalf("tampered claim = (%v, %v), want revoked false", trusted, err)
	}
	if _, trusted, err := repository.Claim(ctx, original); err != nil || trusted {
		t.Fatalf("original claim after tamper = (%v, %v), want revoked false", trusted, err)
	}
}

func TestRepositoryRevokesSameKeyWithDriftedSessionBinding(t *testing.T) {
	repository, _ := newQueueAdmissionTestRepository(t)
	original := queueAdmissionTestBinding(t, "queue-session-drift", "safe original")
	ctx := context.Background()
	if err := repository.Record(ctx, Admission{
		Binding: original, Principal: queueAdmissionTestPrincipal(),
	}); err != nil {
		t.Fatal(err)
	}
	drifted := original
	drifted.SessionKey = "agent:worker:ws:dm:forged"
	if _, trusted, err := repository.Claim(ctx, drifted); err != nil || trusted {
		t.Fatalf("drifted session claim = (%v, %v), want revoked false", trusted, err)
	}
	if _, trusted, err := repository.Claim(ctx, original); err != nil || trusted {
		t.Fatalf("original claim after session drift = (%v, %v), want revoked false", trusted, err)
	}
}

func TestRepositoryAllowsOnlyOneConcurrentClaim(t *testing.T) {
	repository, db := newQueueAdmissionTestRepository(t)
	db.SetMaxOpenConns(1)
	binding := queueAdmissionTestBinding(t, "queue-race", "configure once")
	ctx := context.Background()
	if err := repository.Record(ctx, Admission{
		Binding: binding, Principal: queueAdmissionTestPrincipal(),
	}); err != nil {
		t.Fatal(err)
	}

	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		trusted int
		errs    []error
	)
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, ok, err := repository.Claim(ctx, binding)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errs = append(errs, err)
			}
			if ok {
				trusted++
			}
		}()
	}
	wg.Wait()
	if len(errs) != 0 || trusted != 1 {
		t.Fatalf("concurrent claims trusted=%d errors=%v, want exactly one", trusted, errs)
	}
}

func TestRepositoryRejectsPrincipalDriftAndInvalidRuntimeIdentity(t *testing.T) {
	repository, _ := newQueueAdmissionTestRepository(t)
	binding := queueAdmissionTestBinding(t, "queue-principal-binding", "configure once")
	ctx := context.Background()
	if err := repository.Record(ctx, Admission{
		Binding: binding, Principal: queueAdmissionTestPrincipal(),
	}); err != nil {
		t.Fatal(err)
	}
	drifted := queueAdmissionTestPrincipal()
	drifted.SessionID = "sess-other"
	if err := repository.Record(ctx, Admission{
		Binding: binding, Principal: drifted,
	}); !errors.Is(err, ErrAdmissionConflict) {
		t.Fatalf("same queue identity changed auth session: %v", err)
	}

	for _, invalid := range []PrincipalBinding{
		{UserID: "other-owner", AuthMethod: "password", SessionID: "sess-a"},
		{UserID: "owner", AuthMethod: "password"},
		{UserID: "owner", AuthMethod: "mcp_runtime", SessionID: "sess-a"},
	} {
		otherBinding := queueAdmissionTestBinding(
			t,
			"queue-invalid-"+strings.ReplaceAll(invalid.AuthMethod, "_", "-"),
			"configure once",
		)
		if err := repository.Record(ctx, Admission{
			Binding: otherBinding, Principal: invalid,
		}); err == nil {
			t.Fatalf("invalid principal binding was recorded: %+v", invalid)
		}
	}
}

func TestRepositoryRevokesLegacyAdmissionWithoutFreshAuthBinding(t *testing.T) {
	repository, db := newQueueAdmissionTestRepository(t)
	binding := queueAdmissionTestBinding(t, "queue-legacy-principal", "configure once")
	ctx := context.Background()
	if err := repository.Record(ctx, Admission{
		Binding: binding, Principal: queueAdmissionTestPrincipal(),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(
		`UPDATE configuration_queue_admissions
SET principal_auth_method = '', principal_auth_session_id = ''
WHERE owner_user_id = ? AND scope = ? AND queue_item_id = ? AND agent_id = ?`,
		binding.OwnerUserID,
		string(binding.Scope),
		binding.QueueItemID,
		binding.AgentID,
	); err != nil {
		t.Fatal(err)
	}
	if _, trusted, err := repository.Claim(ctx, binding); err != nil || trusted {
		t.Fatalf("legacy admission claim = (%v, %v), want revoked false", trusted, err)
	}
	var status string
	if err := db.QueryRow(
		`SELECT status FROM configuration_queue_admissions
WHERE owner_user_id = ? AND scope = ? AND queue_item_id = ? AND agent_id = ?`,
		binding.OwnerUserID,
		string(binding.Scope),
		binding.QueueItemID,
		binding.AgentID,
	).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != StatusRevoked {
		t.Fatalf("legacy admission status = %q, want revoked", status)
	}
}

func TestNewBindingRejectsUnknownScopeInsteadOfFallingBackToDM(t *testing.T) {
	_, err := NewBinding(workspacestore.InputQueueLocation{
		OwnerUserID:   "owner",
		Scope:         protocol.InputQueueScope("forged"),
		WorkspacePath: t.TempDir(),
		SessionKey:    "agent:worker:ws:dm:main",
	}, protocol.InputQueueItem{
		ID:      "queue-forged-scope",
		AgentID: "worker",
		Source:  protocol.InputQueueSourceUser,
		Content: "configure the agent",
	})
	if err == nil {
		t.Fatal("unknown queue admission scope must not fall back to DM")
	}
}

func TestNewBindingBindsSemanticRoutingFields(t *testing.T) {
	location := workspacestore.InputQueueLocation{
		OwnerUserID:   "owner",
		Scope:         protocol.InputQueueScopeDM,
		WorkspacePath: t.TempDir(),
		SessionKey:    "agent:worker:ws:dm:main",
	}
	base := protocol.InputQueueItem{
		ID:             "queue-routing",
		AgentID:        "worker",
		Source:         protocol.InputQueueSourceUser,
		Content:        "configure the agent",
		DeliveryPolicy: protocol.ChatDeliveryPolicyQueue,
		RootRoundID:    "root-a",
		HopIndex:       1,
		ExpiresAt:      100,
	}
	original, err := NewBinding(location, base)
	if err != nil {
		t.Fatal(err)
	}
	variants := []protocol.InputQueueItem{base, base, base, base}
	variants[0].DeliveryPolicy = protocol.ChatDeliveryPolicyGuide
	variants[1].RootRoundID = "root-b"
	variants[2].HopIndex = 2
	variants[3].ExpiresAt = 200
	for index, item := range variants {
		changed, changedErr := NewBinding(location, item)
		if changedErr != nil {
			t.Fatalf("variant %d: %v", index, changedErr)
		}
		if changed.PayloadDigest == original.PayloadDigest {
			t.Fatalf("semantic routing variant %d did not change payload digest", index)
		}
	}

	base.Source = protocol.InputQueueSourceAgentPublicMention
	if _, err = NewBinding(location, base); err == nil {
		t.Fatal("Agent queue source must never receive a direct-user admission")
	}
}

func newQueueAdmissionTestRepository(t *testing.T) (*Repository, *sql.DB) {
	t.Helper()
	databasePath := filepath.Join(t.TempDir(), "queue-admission.db")
	db, err := sql.Open("sqlite", databasePath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	ensureGooseSQLiteDialect(t)
	if err = goose.Up(db, "../../../db/migrations/sqlite"); err != nil {
		t.Fatal(err)
	}
	return NewRepository(config.Config{DatabaseDriver: "sqlite"}, db), db
}

func queueAdmissionTestBinding(t *testing.T, itemID string, content string) Binding {
	t.Helper()
	location := workspacestore.InputQueueLocation{
		OwnerUserID:   "owner",
		Scope:         protocol.InputQueueScopeDM,
		WorkspacePath: t.TempDir(),
		SessionKey:    "agent:worker:ws:dm:main",
	}
	binding, err := NewBinding(location, protocol.InputQueueItem{
		ID:          itemID,
		Scope:       protocol.InputQueueScopeDM,
		SessionKey:  location.SessionKey,
		AgentID:     "worker",
		Source:      protocol.InputQueueSourceUser,
		Content:     content,
		OwnerUserID: "owner",
	})
	if err != nil {
		t.Fatal(err)
	}
	return binding
}

func queueAdmissionTestPrincipal() PrincipalBinding {
	return PrincipalBinding{
		UserID: "owner", AuthMethod: "password", SessionID: "sess-owner",
	}
}
