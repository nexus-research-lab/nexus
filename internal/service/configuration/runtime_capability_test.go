package configuration

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/runtime"
)

func TestRuntimeCapabilityBindsStableSessionToActiveRound(t *testing.T) {
	manager := runtime.NewManager()
	service := &Service{
		runtime:                    manager,
		runtimeCapabilities:        make(map[string]*runtimeCapabilityRecord),
		runtimeCapabilityBySession: make(map[string]string),
		runtimeCapabilityNow:       func() time.Time { return time.Now().UTC() },
	}
	actor := Actor{
		OwnerUserID: "owner-a", AgentID: "agent-a",
		LeaseSessionKey: "session-a", LeaseRoundID: "round-a",
		RoundLeaseRequired: true,
	}
	token, err := service.IssueRuntimeCapability(actor)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.ResolveRuntimeCapability(token); err == nil ||
		!strings.Contains(err.Error(), "尚未开始") {
		t.Fatalf("inactive capability error = %v", err)
	}
	if err = manager.StartRound(context.Background(), actor.LeaseSessionKey, actor.LeaseRoundID, nil); err != nil {
		t.Fatal(err)
	}
	resolved, err := service.ResolveRuntimeCapability(token)
	if err != nil || resolved.AgentID != actor.AgentID {
		t.Fatalf("resolved actor = %+v, err=%v", resolved, err)
	}

	next := actor
	next.LeaseRoundID = "round-b"
	nextToken, err := service.IssueRuntimeCapability(next)
	if err != nil {
		t.Fatal(err)
	}
	if nextToken != token {
		t.Fatalf("same runtime session rotated token: %q != %q", nextToken, token)
	}
	if err = manager.StartRound(context.Background(), next.LeaseSessionKey, next.LeaseRoundID, nil); err != nil {
		t.Fatal(err)
	}
	if _, err = service.ResolveRuntimeCapability(token); err == nil ||
		!strings.Contains(err.Error(), "并发 round") {
		t.Fatalf("concurrent rounds must fail closed, err=%v", err)
	}
	manager.MarkRoundFinished(actor.LeaseSessionKey, actor.LeaseRoundID)
	resolved, err = service.ResolveRuntimeCapability(token)
	if err != nil || resolved.LeaseRoundID != next.LeaseRoundID {
		t.Fatalf("successor actor = %+v, err=%v", resolved, err)
	}
}
