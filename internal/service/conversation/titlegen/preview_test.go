package titlegen

import (
	"context"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestFillEmptyPreviewFromGoalUpdatesDefaultSessionTitle(t *testing.T) {
	t.Parallel()

	sessionStore := &fakeSessionService{
		sessions: map[string]*protocol.Session{
			"agent:a:ws:dm:conv_1": {
				SessionKey: "agent:a:ws:dm:conv_1",
				Title:      "New Chat",
			},
		},
	}
	events := &fakeEventBroadcaster{}
	service := NewService(nil, sessionStore, nil, events)

	if err := service.FillEmptyPreviewFromGoal(context.Background(), "agent:a:ws:dm:conv_1", "Ship Goal mode"); err != nil {
		t.Fatalf("FillEmptyPreviewFromGoal() error = %v", err)
	}

	if got := sessionStore.sessions["agent:a:ws:dm:conv_1"].Title; got != "Ship Goal mode" {
		t.Fatalf("session title = %q, want goal objective", got)
	}
	if len(events.events) != 1 || events.events[0].EventType != protocol.EventTypeSessionResyncRequired {
		t.Fatalf("events = %#v, want session_resync_required", events.events)
	}
}

func TestFillEmptyPreviewFromGoalUpdatesDefaultRoomConversationTitle(t *testing.T) {
	t.Parallel()

	roomStore := &fakeRoomService{
		contexts: map[string]*protocol.ConversationContextAggregate{
			"conv_1": {
				Room: protocol.RoomRecord{
					ID:   "room_1",
					Name: "协作房间",
				},
				Conversation: protocol.ConversationRecord{
					ID:    "conv_1",
					Title: "协作房间",
				},
			},
		},
	}
	events := &fakeEventBroadcaster{}
	service := NewService(nil, nil, roomStore, events)

	if err := service.FillEmptyPreviewFromGoal(context.Background(), "room:group:conv_1", "完成 Room Goal"); err != nil {
		t.Fatalf("FillEmptyPreviewFromGoal() error = %v", err)
	}

	if got := roomStore.contexts["conv_1"].Conversation.Title; got != "完成 Room Goal" {
		t.Fatalf("conversation title = %q, want goal objective", got)
	}
	if len(events.events) != 1 || events.events[0].Data["room_id"] != "room_1" || events.events[0].Data["conversation_id"] != "conv_1" {
		t.Fatalf("events = %#v, want room conversation resync", events.events)
	}
}

func TestFillEmptyPreviewFromGoalSkipsNonDefaultTitles(t *testing.T) {
	t.Parallel()

	sessionStore := &fakeSessionService{
		sessions: map[string]*protocol.Session{
			"agent:a:ws:dm:conv_1": {
				SessionKey: "agent:a:ws:dm:conv_1",
				Title:      "已有标题",
			},
		},
	}
	service := NewService(nil, sessionStore, nil, &fakeEventBroadcaster{})

	if err := service.FillEmptyPreviewFromGoal(context.Background(), "agent:a:ws:dm:conv_1", "Ship Goal mode"); err != nil {
		t.Fatalf("FillEmptyPreviewFromGoal() error = %v", err)
	}

	if got := sessionStore.sessions["agent:a:ws:dm:conv_1"].Title; got != "已有标题" {
		t.Fatalf("session title = %q, want unchanged non-default title", got)
	}
}
