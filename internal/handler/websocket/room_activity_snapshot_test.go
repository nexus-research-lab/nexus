package websocket

import (
	"context"
	"testing"

	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
)

func TestActiveChatActivitySourcesAggregatesExactConversationSessions(t *testing.T) {
	permission := permissionctx.NewContext()
	runtime := runtimectx.NewManager()
	handler := &Handler{permission: permission, runtime: runtime}

	bindRunningRound := func(runtimeSessionKey string, dispatchSessionKey string, conversationID string, roundID string) {
		t.Helper()
		permission.BindSessionRoute(runtimeSessionKey, permissionctx.RouteContext{
			DispatchSessionKey: dispatchSessionKey,
			RoomID:             "room-1",
			ConversationID:     conversationID,
		})
		if err := runtime.StartRound(context.Background(), runtimeSessionKey, roundID, func() {}); err != nil {
			t.Fatalf("StartRound(%q): %v", runtimeSessionKey, err)
		}
	}

	bindRunningRound("agent:cindy:ws:dm:conversation-a", "agent:cindy:ws:dm:conversation-a", "conversation-a", "round-a")
	bindRunningRound("agent:cindy:ws:dm:conversation-b", "agent:cindy:ws:dm:conversation-b", "conversation-b", "round-b")
	permission.BindSessionRoute("agent:idle:ws:dm:conversation-idle", permissionctx.RouteContext{
		DispatchSessionKey: "agent:idle:ws:dm:conversation-idle",
		RoomID:             "room-1",
		ConversationID:     "conversation-idle",
	})

	sources := handler.activeChatActivitySources("room-1")
	if len(sources) != 2 {
		t.Fatalf("active source count = %d, want 2: %+v", len(sources), sources)
	}
	if sources[0].SessionKey != "agent:cindy:ws:dm:conversation-a" ||
		sources[0].ConversationID != "conversation-a" ||
		len(sources[0].RunningRoundIDs) != 1 || sources[0].RunningRoundIDs[0] != "round-a" {
		t.Fatalf("first source = %+v", sources[0])
	}
	if sources[1].SessionKey != "agent:cindy:ws:dm:conversation-b" ||
		sources[1].ConversationID != "conversation-b" ||
		len(sources[1].RunningRoundIDs) != 1 || sources[1].RunningRoundIDs[0] != "round-b" {
		t.Fatalf("second source = %+v", sources[1])
	}
}
