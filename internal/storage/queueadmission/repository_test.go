package queueadmission

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

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
