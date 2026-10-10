package authctx

import (
	"context"
	"testing"
)

func TestQueuedHumanPrincipalBindingRequiresOpaqueHostAuthIdentity(t *testing.T) {
	password := QueuedHumanPrincipalBinding{
		UserID: "owner-a", AuthMethod: AuthMethodPassword, SessionID: "sess-a",
	}
	ctx := WithQueuedHumanPrincipalBinding(context.Background(), password)
	got, ok := QueuedHumanPrincipalBindingFromContext(ctx)
	if !ok || got != password {
		t.Fatalf("password queue binding = (%+v, %v)", got, ok)
	}

	for _, invalid := range []QueuedHumanPrincipalBinding{
		{UserID: "owner-a", AuthMethod: AuthMethodPassword},
		{UserID: "owner-a", AuthMethod: AuthMethodLocal},
		{UserID: SystemUserID, AuthMethod: AuthMethodLocal, SessionID: "forged-token"},
		{UserID: "owner-a", AuthMethod: "mcp_runtime", SessionID: "sess-a"},
	} {
		invalidCtx := WithQueuedHumanPrincipalBinding(context.Background(), invalid)
		if _, exists := QueuedHumanPrincipalBindingFromContext(invalidCtx); exists {
			t.Fatalf("invalid queue binding was accepted: %+v", invalid)
		}
	}
}

func TestDirectHumanPrincipalBindingSupportsLocalWebControlPlane(t *testing.T) {
	localWeb := WithState(context.Background(), State{AuthRequired: false})
	binding, ok := DirectHumanPrincipalBindingFromContext(localWeb, SystemUserID)
	if !ok || binding != (QueuedHumanPrincipalBinding{
		UserID: SystemUserID, AuthMethod: AuthMethodLocal,
	}) {
		t.Fatalf("local Web queue binding = (%+v, %v)", binding, ok)
	}

	sessionID := "session-a"
	password := WithPrincipal(
		WithState(context.Background(), State{AuthRequired: true}),
		&Principal{
			UserID: "owner-a", AuthMethod: AuthMethodPassword, SessionID: &sessionID,
		},
	)
	binding, ok = DirectHumanPrincipalBindingFromContext(password, "owner-a")
	if !ok || binding != (QueuedHumanPrincipalBinding{
		UserID: "owner-a", AuthMethod: AuthMethodPassword, SessionID: sessionID,
	}) {
		t.Fatalf("password queue binding = (%+v, %v)", binding, ok)
	}

	for _, invalid := range []struct {
		name  string
		ctx   context.Context
		owner string
	}{
		{name: "missing auth state", ctx: context.Background(), owner: SystemUserID},
		{name: "wrong owner", ctx: password, owner: "owner-b"},
		{name: "missing password session", ctx: WithPrincipal(
			WithState(context.Background(), State{AuthRequired: true}),
			&Principal{UserID: "owner-a", AuthMethod: AuthMethodPassword},
		), owner: "owner-a"},
	} {
		t.Run(invalid.name, func(t *testing.T) {
			if got, accepted := DirectHumanPrincipalBindingFromContext(invalid.ctx, invalid.owner); accepted {
				t.Fatalf("invalid direct human binding accepted: %+v", got)
			}
		})
	}
}
