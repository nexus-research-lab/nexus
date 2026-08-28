package launcher

import (
	"context"
	"reflect"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	sessionsvc "github.com/nexus-research-lab/nexus/internal/service/session"
)

type fakeLauncherSessionReader struct {
	pages     map[string]*protocol.MessagePage
	requested []string
	limits    []int
}

func (f *fakeLauncherSessionReader) ListDirectorySessions(context.Context) ([]protocol.Session, error) {
	return nil, nil
}

func (f *fakeLauncherSessionReader) GetSessionMessagesPage(
	_ context.Context,
	sessionKey string,
	request sessionsvc.MessagePageRequest,
) (*protocol.MessagePage, error) {
	f.requested = append(f.requested, sessionKey)
	f.limits = append(f.limits, request.Limit)
	if page, ok := f.pages[sessionKey]; ok {
		return page, nil
	}
	return &protocol.MessagePage{}, nil
}

func TestAttachLatestReplyPreviewsIncludesDMConversations(t *testing.T) {
	sharedRoomKey := protocol.BuildRoomSharedSessionKey("conversation-room")
	reader := &fakeLauncherSessionReader{
		pages: map[string]*protocol.MessagePage{
			"agent-session-dm": {
				Items: []protocol.Message{
					{
						"role": "assistant",
						"content": []any{
							map[string]any{"type": "text", "text": "DM latest reply"},
						},
					},
				},
			},
			sharedRoomKey: {
				Items: []protocol.Message{
					{
						"role": "assistant",
						"content": []any{
							map[string]any{"type": "text", "text": "Room latest reply"},
						},
					},
				},
			},
		},
	}
	service := &Service{session: reader}
	items := []BootstrapConversation{
		{
			SessionKey: "agent-session-dm",
			AgentID:    "agent-a",
			RoomType:   protocol.RoomTypeDM,
			Title:      "DM",
		},
		{
			SessionKey:     "room-slot-a",
			RoomID:         "room-1",
			ConversationID: "conversation-room",
			RoomType:       "room",
			Title:          "Room",
		},
		{
			SessionKey:     "room-slot-b",
			RoomID:         "room-1",
			ConversationID: "conversation-room",
			RoomType:       "room",
			Title:          "Room duplicate",
		},
		{
			SessionKey:  "external-dm",
			AgentID:     "agent-b",
			RoomType:    protocol.RoomTypeDM,
			ChannelType: protocol.SessionChannelDiscord,
			Title:       "External DM",
		},
	}

	service.attachLatestReplyPreviews(context.Background(), items)

	if got := items[0].LastReplyPreview; got != "DM latest reply" {
		t.Fatalf("DM preview = %q, want %q", got, "DM latest reply")
	}
	if got := items[1].LastReplyPreview; got != "Room latest reply" {
		t.Fatalf("Room preview = %q, want %q", got, "Room latest reply")
	}
	if got := items[2].LastReplyPreview; got != "" {
		t.Fatalf("duplicate Room preview = %q, want empty", got)
	}
	if got := items[3].LastReplyPreview; got != "" {
		t.Fatalf("external conversation preview = %q, want empty", got)
	}

	wantRequests := []string{"agent-session-dm", sharedRoomKey}
	if !reflect.DeepEqual(reader.requested, wantRequests) {
		t.Fatalf("requested session keys = %#v, want %#v", reader.requested, wantRequests)
	}
	for _, limit := range reader.limits {
		if limit != 2 {
			t.Fatalf("message page limit = %d, want 2", limit)
		}
	}
}
