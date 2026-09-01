package agentrepo

import (
	"context"
	"errors"
	"testing"
)

func TestAgentCreationRequestClaimCommitReplayAndDeletionTombstone(t *testing.T) {
	db := newAgentRepositoryTestDB(t)
	repository := NewSQLRepository("sqlite", db)
	ctx := context.Background()
	claim := CreationRequestRecord{
		OwnerUserID:       "owner-1",
		CreationRequestID: "web-create:request-1",
		IntentDigest:      "digest-1",
		AgentID:           "agent-1",
		WorkspacePath:     "/tmp/agent-1",
		ClaimToken:        "claim-1",
		LeaseExpiresAtMS:  2_000,
	}

	reserved, claimed, err := repository.ClaimAgentCreation(ctx, claim, 1_000)
	if err != nil || !claimed || reserved.AgentID != claim.AgentID {
		t.Fatalf("first claim = %#v, claimed=%v, err=%v", reserved, claimed, err)
	}
	replayCandidate := claim
	replayCandidate.AgentID = "agent-2"
	replayCandidate.WorkspacePath = "/tmp/agent-2"
	replayCandidate.ClaimToken = "claim-2"
	replay, claimed, err := repository.ClaimAgentCreation(ctx, replayCandidate, 1_500)
	if err != nil || claimed || replay.AgentID != claim.AgentID || replay.ClaimToken != claim.ClaimToken {
		t.Fatalf("active replay = %#v, claimed=%v, err=%v", replay, claimed, err)
	}

	record := testCreateRecord()
	prepared, err := repository.MarkAgentCreationWorkspacePrepared(ctx, *reserved)
	if err != nil || !prepared {
		t.Fatalf("MarkAgentCreationWorkspacePrepared() prepared=%v err=%v", prepared, err)
	}
	reserved.Stage = CreationRequestWorkspacePrepared
	if err = repository.CommitAgentCreation(ctx, *reserved, record); err != nil {
		t.Fatalf("CommitAgentCreation() error = %v", err)
	}
	committed, err := repository.GetAgentCreationRequest(ctx, claim.OwnerUserID, claim.CreationRequestID)
	if err != nil || committed == nil || committed.Status != CreationRequestCommitted {
		t.Fatalf("committed receipt = %#v, err=%v", committed, err)
	}
	if err = repository.CommitAgentCreation(ctx, *reserved, record); !errors.Is(err, ErrCreationClaimLost) {
		t.Fatalf("stale claim commit error = %v, want ErrCreationClaimLost", err)
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = repository.markAgentCreationRequestsDeleted(ctx, tx, record.OwnerUserID, record.AgentID); err == nil {
		_, err = tx.ExecContext(ctx, `DELETE FROM agents WHERE id = ? AND owner_user_id = ?`, record.AgentID, record.OwnerUserID)
	}
	if err == nil {
		err = tx.Commit()
	} else {
		_ = tx.Rollback()
	}
	if err != nil {
		t.Fatalf("delete transaction error = %v", err)
	}
	deleted, err := repository.GetAgentCreationRequest(ctx, claim.OwnerUserID, claim.CreationRequestID)
	if err != nil || deleted == nil || deleted.Status != CreationRequestDeleted {
		t.Fatalf("deleted receipt = %#v, err=%v", deleted, err)
	}
}

func TestAgentCreationRequestExpiredLeaseKeepsReservedAgentIdentity(t *testing.T) {
	db := newAgentRepositoryTestDB(t)
	repository := NewSQLRepository("sqlite", db)
	ctx := context.Background()
	first := CreationRequestRecord{
		OwnerUserID:       "owner-1",
		CreationRequestID: "web-create:request-lease",
		IntentDigest:      "digest-lease",
		AgentID:           "agent-lease",
		WorkspacePath:     "/tmp/agent-lease",
		ClaimToken:        "claim-old",
		LeaseExpiresAtMS:  2_000,
	}
	if _, claimed, err := repository.ClaimAgentCreation(ctx, first, 1_000); err != nil || !claimed {
		t.Fatalf("first claim claimed=%v err=%v", claimed, err)
	}
	next := first
	next.AgentID = "must-not-replace-agent"
	next.WorkspacePath = "/tmp/must-not-replace-agent"
	next.ClaimToken = "claim-new"
	next.LeaseExpiresAtMS = 5_000
	claimedRecord, claimed, err := repository.ClaimAgentCreation(ctx, next, 2_001)
	if err != nil || !claimed {
		t.Fatalf("takeover claimed=%v err=%v", claimed, err)
	}
	if claimedRecord.AgentID != first.AgentID || claimedRecord.WorkspacePath != first.WorkspacePath ||
		claimedRecord.ClaimToken != next.ClaimToken {
		t.Fatalf("takeover replaced immutable reservation: %#v", claimedRecord)
	}
}

func TestAgentCreationReceiptAndAgentRecordCommitAtomically(t *testing.T) {
	db := newAgentRepositoryTestDB(t)
	repository := NewSQLRepository("sqlite", db)
	ctx := context.Background()
	claim := CreationRequestRecord{
		OwnerUserID:       "owner-1",
		CreationRequestID: "web-create:atomic",
		IntentDigest:      "digest-atomic",
		AgentID:           "agent-1",
		WorkspacePath:     "/tmp/agent-1",
		ClaimToken:        "claim-atomic",
		LeaseExpiresAtMS:  2_000,
	}
	reserved, claimed, err := repository.ClaimAgentCreation(ctx, claim, 1_000)
	if err != nil || !claimed {
		t.Fatalf("claim = %#v claimed=%v err=%v", reserved, claimed, err)
	}
	prepared, err := repository.MarkAgentCreationWorkspacePrepared(ctx, *reserved)
	if err != nil || !prepared {
		t.Fatalf("prepared=%v err=%v", prepared, err)
	}
	reserved.Stage = CreationRequestWorkspacePrepared
	if _, err = db.Exec(`
CREATE TRIGGER fail_runtime_create
BEFORE INSERT ON runtimes
BEGIN
  SELECT RAISE(FAIL, 'forced runtime insert failure');
END`); err != nil {
		t.Fatal(err)
	}
	if err = repository.CommitAgentCreation(ctx, *reserved, testCreateRecord()); err == nil {
		t.Fatal("CommitAgentCreation() unexpectedly succeeded")
	}
	current, err := repository.GetAgentCreationRequest(ctx, claim.OwnerUserID, claim.CreationRequestID)
	if err != nil {
		t.Fatal(err)
	}
	if current == nil || current.Status != CreationRequestPending ||
		current.Stage != CreationRequestWorkspacePrepared {
		t.Fatalf("receipt advanced without Agent transaction: %#v", current)
	}
	item, err := repository.GetAgent(ctx, claim.AgentID, claim.OwnerUserID)
	if err != nil || item != nil {
		t.Fatalf("Agent escaped failed transaction: item=%#v err=%v", item, err)
	}
	for table, column := range map[string]string{
		"agents":   "id",
		"profiles": "agent_id",
		"runtimes": "agent_id",
	} {
		var count int
		if err = db.QueryRow(
			"SELECT COUNT(*) FROM "+table+" WHERE "+column+" = ?",
			claim.AgentID,
		).Scan(&count); err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		if count != 0 {
			t.Fatalf("%s escaped failed transaction: count=%d", table, count)
		}
	}
}
