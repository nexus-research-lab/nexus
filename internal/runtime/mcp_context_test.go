package runtime

import (
	"context"
	"testing"
)

func TestMCPRoundLeaseContextRequiresCompleteInternalLease(t *testing.T) {
	if _, ok := MCPRoundLeaseFromContext(context.Background()); ok {
		t.Fatal("empty context must not contain an MCP round lease")
	}
	if _, ok := MCPRoundLeaseFromContext(
		WithMCPRoundLease(context.Background(), "session", ""),
	); ok {
		t.Fatal("partial lease must not be recorded")
	}

	ctx := WithMCPRoundLease(context.Background(), " session ", " round ")
	lease, ok := MCPRoundLeaseFromContext(ctx)
	if !ok {
		t.Fatal("complete lease missing")
	}
	if lease.SessionKey != "session" || lease.RoundID != "round" {
		t.Fatalf("lease was not normalized: %+v", lease)
	}
}
