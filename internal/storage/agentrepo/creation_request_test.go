package agentrepo

import (
	"context"
	"testing"
)

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
