package runtime

import (
	"context"
	"testing"
)

func TestRuntimeRoundLeaseContextRequiresCompleteInternalLease(t *testing.T) {
	if _, ok := RuntimeRoundLeaseFromContext(context.Background()); ok {
		t.Fatal("empty context must not contain an MCP round lease")
	}
	if _, ok := RuntimeRoundLeaseFromContext(
		WithRuntimeRoundLease(context.Background(), "session", ""),
	); ok {
		t.Fatal("partial lease must not be recorded")
	}

	ctx := WithRuntimeRoundLease(context.Background(), " session ", " round ")
	lease, ok := RuntimeRoundLeaseFromContext(ctx)
	if !ok {
		t.Fatal("complete lease missing")
	}
	if lease.SessionKey != "session" || lease.RoundID != "round" {
		t.Fatalf("lease was not normalized: %+v", lease)
	}
}
