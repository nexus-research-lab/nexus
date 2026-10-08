package sandbox

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

func newSandboxReceiptRepository(t *testing.T) *Repository {
	t.Helper()
	path := filepath.Join(t.TempDir(), "sandbox.db")
	db, err := sql.Open("sqlite", path+"?_pragma=foreign_keys(0)&_pragma=busy_timeout(5000)&_txlock=immediate")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	if err = goose.SetDialect("sqlite3"); err != nil {
		t.Fatal(err)
	}
	if err = goose.Up(db, "../../../db/migrations/sqlite"); err != nil {
		t.Fatal(err)
	}
	return NewRepository(config.Config{DatabaseDriver: "sqlite"}, db)
}

func testSandboxReceipt() protocol.SandboxPolicyReceiptSnapshot {
	now := time.Date(2026, 9, 24, 1, 2, 3, 0, time.UTC)
	return protocol.SandboxPolicyReceiptSnapshot{
		Version: 1, OwnerUserID: "owner-1", SessionKey: "session-1", SessionID: "bridge-1",
		RuntimeKind: "nxs", Generation: 4, RoundID: "round-4", PolicyDigest: "sha256:policy",
		RequiredCapabilities:     []string{"sandbox", "file_tools"},
		AcknowledgedCapabilities: []string{"sandbox", "file_tools"},
		CapabilityEvidence:       "bridge_negotiated", IsolationEvidence: "not_attested",
		ResourcePolicyJSON: `{"scope":"session"}`, LeaseID: "lease-4",
		Phase: protocol.SandboxPolicyReceiptConfirmed, ConfirmedAt: now, UpdatedAt: now,
	}
}

func TestRepositoryPersistsLatestReceiptAndPhaseAcrossReopen(t *testing.T) {
	repository := newSandboxReceiptRepository(t)
	receipt := testSandboxReceipt()
	if err := repository.Save(t.Context(), receipt); err != nil {
		t.Fatal(err)
	}
	if err := repository.Save(t.Context(), receipt); err != nil {
		t.Fatalf("idempotent save: %v", err)
	}
	got, found, err := repository.Get(t.Context(), receipt.OwnerUserID, receipt.SessionKey, receipt.Generation)
	if err != nil || !found {
		t.Fatalf("get receipt found=%t err=%v", found, err)
	}
	if got.PolicyDigest != receipt.PolicyDigest || got.RoundID != receipt.RoundID || len(got.RequiredCapabilities) != 2 {
		t.Fatalf("receipt = %#v", got)
	}
	// A duplicate confirmed-generation callback cannot rewrite the policy facts
	// or move the original confirmation timestamp, even if its payload differs.
	mutatedConfirmed := receipt
	mutatedConfirmed.PolicyDigest = "sha256:late-confirmed-policy"
	mutatedConfirmed.LeaseID = "late-confirmed-lease"
	mutatedConfirmed.ConfirmedAt = receipt.ConfirmedAt.Add(time.Hour)
	mutatedConfirmed.UpdatedAt = receipt.UpdatedAt.Add(time.Hour)
	if err := repository.Save(t.Context(), mutatedConfirmed); err != nil {
		t.Fatalf("conflicting confirmed retry: %v", err)
	}
	got, found, err = repository.Get(t.Context(), receipt.OwnerUserID, receipt.SessionKey, receipt.Generation)
	if err != nil || !found || got.PolicyDigest != receipt.PolicyDigest || got.LeaseID != receipt.LeaseID || !got.ConfirmedAt.Equal(receipt.ConfirmedAt) {
		t.Fatalf("conflicting confirmed retry changed immutable payload: found=%t receipt=%#v err=%v", found, got, err)
	}
	// An out-of-order duplicate observation may not move the durable observation
	// time backwards, even while the row is still confirmed.
	olderConfirmed := receipt
	olderConfirmed.UpdatedAt = mutatedConfirmed.UpdatedAt.Add(-2 * time.Hour)
	if err := repository.Save(t.Context(), olderConfirmed); err != nil {
		t.Fatalf("older confirmed retry: %v", err)
	}
	got, found, err = repository.Get(t.Context(), receipt.OwnerUserID, receipt.SessionKey, receipt.Generation)
	if err != nil || !found || !got.UpdatedAt.Equal(mutatedConfirmed.UpdatedAt) {
		t.Fatalf("older confirmed retry regressed observation time: found=%t receipt=%#v err=%v", found, got, err)
	}
	if err := repository.UpdatePhase(t.Context(), receipt.OwnerUserID, receipt.SessionKey, receipt.Generation, protocol.SandboxPolicyReceiptUnknown, "close ACK lost"); err != nil {
		t.Fatal(err)
	}
	got, found, err = repository.Latest(t.Context(), receipt.OwnerUserID, receipt.SessionKey)
	if err != nil || !found {
		t.Fatalf("latest receipt found=%t err=%v", found, err)
	}
	if got.Phase != protocol.SandboxPolicyReceiptUnknown || got.UnknownReason != "close ACK lost" {
		t.Fatalf("unknown phase = %#v", got)
	}
	if err := repository.Save(t.Context(), receipt); err != nil {
		t.Fatalf("retry original receipt: %v", err)
	}
	got, found, err = repository.Latest(t.Context(), receipt.OwnerUserID, receipt.SessionKey)
	if err != nil || !found || got.Phase != protocol.SandboxPolicyReceiptUnknown || got.UnknownReason != "close ACK lost" {
		t.Fatalf("retry reopened lifecycle receipt: found=%t receipt=%#v err=%v", found, got, err)
	}
	mutated := receipt
	mutated.PolicyDigest = "sha256:late-mutated-policy"
	mutated.LeaseID = "late-mutated-lease"
	if err := repository.Save(t.Context(), mutated); err != nil {
		t.Fatalf("late terminal save: %v", err)
	}
	got, found, err = repository.Latest(t.Context(), receipt.OwnerUserID, receipt.SessionKey)
	if err != nil || !found || got.PolicyDigest != receipt.PolicyDigest || got.LeaseID != receipt.LeaseID {
		t.Fatalf("late terminal save changed immutable payload: found=%t receipt=%#v err=%v", found, got, err)
	}
	// A late callback from the old close path must not reopen an unknown
	// generation as retiring or confirmed.
	if err := repository.UpdatePhase(t.Context(), receipt.OwnerUserID, receipt.SessionKey, receipt.Generation, protocol.SandboxPolicyReceiptRetiring, "late callback"); err != nil {
		t.Fatalf("late retiring callback: %v", err)
	}
	got, found, err = repository.Latest(t.Context(), receipt.OwnerUserID, receipt.SessionKey)
	if err != nil || !found || got.Phase != protocol.SandboxPolicyReceiptUnknown || got.UnknownReason != "close ACK lost" {
		t.Fatalf("late callback reopened receipt: found=%t receipt=%#v err=%v", found, got, err)
	}
	if _, _, err := repository.Latest(t.Context(), "other-owner", receipt.SessionKey); err != nil {
		t.Fatal(err)
	}
}

func TestRepositoryAllowsReaperFailureToDowngradeRetiredToUnknown(t *testing.T) {
	repository := newSandboxReceiptRepository(t)
	receipt := testSandboxReceipt()
	if err := repository.Save(t.Context(), receipt); err != nil {
		t.Fatal(err)
	}
	if err := repository.UpdatePhase(t.Context(), receipt.OwnerUserID, receipt.SessionKey, receipt.Generation, protocol.SandboxPolicyReceiptRetired, ""); err != nil {
		t.Fatal(err)
	}
	if err := repository.UpdatePhase(t.Context(), receipt.OwnerUserID, receipt.SessionKey, receipt.Generation, protocol.SandboxPolicyReceiptUnknown, "owner process reaper failed"); err != nil {
		t.Fatal(err)
	}
	got, found, err := repository.Latest(t.Context(), receipt.OwnerUserID, receipt.SessionKey)
	if err != nil || !found {
		t.Fatalf("latest receipt found=%t err=%v", found, err)
	}
	if got.Phase != protocol.SandboxPolicyReceiptUnknown || got.UnknownReason != "owner process reaper failed" {
		t.Fatalf("reaper failure phase = %#v", got)
	}
	// Once uncertainty is recorded, stale callbacks cannot make it look clean.
	if err := repository.UpdatePhase(t.Context(), receipt.OwnerUserID, receipt.SessionKey, receipt.Generation, protocol.SandboxPolicyReceiptRetired, "late callback"); err != nil {
		t.Fatal(err)
	}
	got, _, err = repository.Latest(t.Context(), receipt.OwnerUserID, receipt.SessionKey)
	if err != nil || got.Phase != protocol.SandboxPolicyReceiptUnknown || got.UnknownReason != "owner process reaper failed" {
		t.Fatalf("late retired callback changed unknown receipt = %#v err=%v", got, err)
	}
}

func TestRepositoryRejectsUnknownWithoutReasonAndMalformedRows(t *testing.T) {
	repository := newSandboxReceiptRepository(t)
	receipt := testSandboxReceipt()
	if err := repository.UpdatePhase(t.Context(), receipt.OwnerUserID, receipt.SessionKey, receipt.Generation, protocol.SandboxPolicyReceiptUnknown, ""); err == nil {
		t.Fatal("unknown phase without reason accepted")
	}
	if err := repository.Save(t.Context(), protocol.SandboxPolicyReceiptSnapshot{OwnerUserID: "owner", SessionKey: "session", Generation: 1, Version: 1, Phase: protocol.SandboxPolicyReceiptConfirmed}); err == nil {
		t.Fatal("missing digest accepted")
	}
	if err := repository.Save(t.Context(), receipt); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.db.Exec(`UPDATE sandbox_policy_receipts SET required_capabilities_json = '[' WHERE owner_user_id = 'owner-1'`); err != nil {
		t.Fatal(err)
	}
	if _, _, err := repository.Latest(t.Context(), receipt.OwnerUserID, receipt.SessionKey); err == nil {
		t.Fatal("malformed capability JSON was accepted")
	}
	if _, err := repository.db.Exec(`UPDATE sandbox_policy_receipts SET required_capabilities_json = '[]', phase = 'obsolete' WHERE owner_user_id = 'owner-1'`); err != nil {
		t.Fatal(err)
	}
	if _, _, err := repository.Latest(t.Context(), receipt.OwnerUserID, receipt.SessionKey); err == nil {
		t.Fatal("unknown persisted phase was accepted")
	}
}
