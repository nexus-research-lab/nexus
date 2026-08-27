// INPUT: broker issuance before a fresh/forked provider SDK Session identity is published.
// OUTPUT: the already-issued runtime command context observes each host update.
// POS: regression boundary for MCP-equivalent invocation-time Session identity.
package runtime

import (
	"context"
	"testing"
)

func TestRuntimeCommandContextReadsCurrentSDKSessionIdentity(t *testing.T) {
	identity := NewSDKSessionIdentityState("")
	commandContext := RuntimeCommandContext{SDKSessionIdentity: identity}
	if got := commandContext.CurrentSDKSessionID(); got != "" {
		t.Fatalf("initial SDK session identity = %q", got)
	}
	identity.Set(" sdk-session-fresh ")
	if got := commandContext.CurrentSDKSessionID(); got != "sdk-session-fresh" {
		t.Fatalf("updated SDK session identity = %q", got)
	}
	identity.Set("sdk-session-forked")
	if got := commandContext.CurrentSDKSessionID(); got != "sdk-session-forked" {
		t.Fatalf("forked SDK session identity = %q", got)
	}
}

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
