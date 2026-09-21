package relay

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	relaycontract "github.com/nexus-research-lab/nexus/internal/relay"
)

func TestClientWatchesCommittedStreamUpdates(t *testing.T) {
	const token = "relay-user-token"
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/ws/relay" || request.Header.Get("Authorization") != "Bearer "+token ||
			request.URL.Query().Get("stream_id") != "stream-1" ||
			request.URL.Query().Get("stream_epoch") != "epoch-1" {
			http.Error(writer, "invalid websocket request", http.StatusBadRequest)
			return
		}
		connection, err := websocket.Accept(writer, request, nil)
		if err != nil {
			t.Errorf("accept websocket: %v", err)
			return
		}
		defer connection.CloseNow()
		if err = wsjson.Write(request.Context(), connection, relaycontract.StreamUpdated{
			Type: "stream.updated", StreamID: "stream-1", StreamEpoch: "epoch-1", HighWaterSeq: 7,
		}); err != nil {
			t.Errorf("write websocket update: %v", err)
		}
	}))
	t.Cleanup(server.Close)
	client, err := NewClient(server.URL, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	stop := errors.New("stop after first update")
	err = client.Watch(context.Background(), token, "stream-1", "epoch-1", func(update relaycontract.StreamUpdated) error {
		if update.HighWaterSeq != 7 {
			t.Fatalf("high water = %d", update.HighWaterSeq)
		}
		return stop
	})
	if !errors.Is(err, stop) {
		t.Fatalf("Watch() error = %v", err)
	}
}

func TestWatchHeartbeatDetectsSilentPeerAndKeepsHealthyConnection(t *testing.T) {
	for _, responsive := range []bool{true, false} {
		t.Run(map[bool]string{true: "pong", false: "silent"}[responsive], func(t *testing.T) {
			release := make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				conn, err := websocket.Accept(w, r, nil)
				if err != nil {
					return
				}
				defer conn.CloseNow()
				if responsive {
					conn.CloseRead(r.Context())
				}
				<-release
			}))
			defer server.Close()
			defer close(release)
			ctx, cancel := context.WithCancelCause(t.Context())
			defer cancel(nil)
			conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http"), nil)
			if err != nil {
				t.Fatal(err)
			}
			defer conn.CloseNow()
			conn.CloseRead(ctx)
			go watchHeartbeat(ctx, conn, 10*time.Millisecond, 50*time.Millisecond, cancel)
			select {
			case <-ctx.Done():
				if responsive || !strings.Contains(context.Cause(ctx).Error(), "heartbeat failed") {
					t.Fatalf("unexpected failure: %v", context.Cause(ctx))
				}
			case <-time.After(250 * time.Millisecond):
				if !responsive {
					t.Fatal("silent peer was not detected")
				}
			}
		})
	}
}

func TestNodeWatchRefreshesOnTheSameConnection(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		defer conn.CloseNow()
		if err = wsjson.Write(r.Context(), conn, relaycontract.StreamUpdated{Type: "auth.refresh_required"}); err != nil {
			t.Error(err)
			return
		}
		var message struct {
			Type  string `json:"type"`
			Token string `json:"token"`
		}
		if err = wsjson.Read(r.Context(), conn, &message); err != nil || message.Type != "auth.refresh" || message.Token != "new-token" {
			t.Errorf("refresh: %+v %v", message, err)
			return
		}
		if err = wsjson.Write(r.Context(), conn, relaycontract.StreamUpdated{Type: "auth.refreshed"}); err != nil {
			t.Error(err)
			return
		}
		if err = wsjson.Write(r.Context(), conn, relaycontract.StreamUpdated{Type: "deliveries.updated"}); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	client, err := NewClient(server.URL, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	stop := errors.New("received delivery")
	err = client.WatchDeliveries(ctx, "old-token", func(context.Context) (string, error) { return "new-token", nil }, func() error { return stop })
	if !errors.Is(err, stop) {
		t.Fatal(err)
	}
}

func TestClientReturnsRelayErrorFromWebSocketUpgrade(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"code": "full_snapshot_required", "message": "snapshot required", "request_id": "req-wss",
		})
	}))
	t.Cleanup(server.Close)
	client, err := NewClient(server.URL, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	err = client.Watch(context.Background(), "token", "stream-1", "epoch-old", func(relaycontract.StreamUpdated) error {
		return nil
	})
	var remoteError *relaycontract.RemoteError
	if !errors.As(err, &remoteError) || remoteError.StatusCode != http.StatusConflict ||
		remoteError.Code != "full_snapshot_required" || remoteError.RequestID != "req-wss" {
		t.Fatalf("Watch() error = %#v", err)
	}
}

func TestClientMatchesRelayM1HTTPContract(t *testing.T) {
	const token = "relay-user-token"
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer "+token {
			http.Error(writer, "unauthorized", http.StatusUnauthorized)
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		switch request.Method + " " + request.URL.Path {
		case "GET /api/relay/v1/rooms":
			writeRelayTestData(t, writer, http.StatusOK, relaycontract.RoomList{Rooms: []relaycontract.RoomView{{
				Room: relaycontract.Room{ID: "room-1", TeamID: "team-1", Name: "研发群"},
				Conversation: relaycontract.Conversation{
					ID: "conversation-1", RoomID: "room-1", SyncStreamID: "stream-1", StreamEpoch: "epoch-1",
				},
				CurrentUserRole: "owner",
			}}})
		case "POST /api/relay/v1/rooms":
			if request.Header.Get("Idempotency-Key") != "create-room-1" {
				t.Errorf("Idempotency-Key = %q", request.Header.Get("Idempotency-Key"))
			}
			var input relaycontract.CreateRoomInput
			if err := json.NewDecoder(request.Body).Decode(&input); err != nil || input.Name != "研发群" {
				t.Errorf("create room input = %+v, %v", input, err)
			}
			writeRelayTestData(t, writer, http.StatusCreated, relaycontract.RoomView{
				Room: relaycontract.Room{ID: "room-1", TeamID: "team-1", Name: "研发群"},
				Conversation: relaycontract.Conversation{
					ID: "conversation-1", RoomID: "room-1", SyncStreamID: "stream-1", StreamEpoch: "epoch-1",
				},
				CurrentUserRole: "owner",
			})
		case "POST /api/relay/v1/conversations/conversation-1/messages":
			if request.Header.Get("Idempotency-Key") != "command-1" {
				t.Errorf("Idempotency-Key = %q", request.Header.Get("Idempotency-Key"))
			}
			var input relaycontract.CreateMessageInput
			if err := json.NewDecoder(request.Body).Decode(&input); err != nil {
				t.Errorf("decode message: %v", err)
			}
			want := relaycontract.CreateMessageInput{Content: relaycontract.MessageContent{
				Version: relaycontract.ContentVersionV1,
				Blocks:  []relaycontract.ContentBlock{{Type: relaycontract.BlockTypeMarkdown, Text: "hello"}},
			}}
			if !reflect.DeepEqual(input, want) {
				t.Errorf("message input = %#v", input)
			}
			writeRelayTestData(t, writer, http.StatusCreated, relaycontract.MessageCommit{
				Message: relaycontract.Message{
					ID: "message-1", ConversationID: "conversation-1", MessageSeq: 4,
					AuthorType: relaycontract.AuthorTypeUser, ClientMessageID: "command-1",
				},
				StreamID: "stream-1", StreamEpoch: "epoch-1", EventSeq: 5, HighWaterSeq: 5,
			})
		case "GET /api/relay/v1/conversations/conversation-1/snapshot":
			query := request.URL.Query()
			if query.Get("after_message_seq") != "2" || query.Get("limit") != "100" ||
				query.Get("through_message_seq") != "9" || query.Get("snapshot_seq") != "12" ||
				query.Get("stream_epoch") != "epoch-1" {
				t.Errorf("snapshot query = %q", request.URL.RawQuery)
			}
			writeRelayTestData(t, writer, http.StatusOK, relaycontract.Snapshot{
				ConversationID: "conversation-1", StreamID: "stream-1",
				StreamEpoch: "epoch-1", SnapshotSeq: 12, ThroughMessageSeq: 9, AfterMessageSeq: 2,
			})
		case "GET /api/relay/v1/sync-streams/stream-1/difference":
			query := request.URL.Query()
			if query.Get("after_seq") != "5" || query.Get("limit") != "20" ||
				query.Get("stream_epoch") != "epoch-1" {
				t.Errorf("difference query = %q", request.URL.RawQuery)
			}
			writeRelayTestData(t, writer, http.StatusOK, relaycontract.Difference{
				StreamID: "stream-1", StreamEpoch: "epoch-1", AfterSeq: 5, NextSeq: 7, HighWaterSeq: 7,
			})
		default:
			http.NotFound(writer, request)
		}
	}))
	t.Cleanup(server.Close)

	client, err := NewClient(server.URL, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	rooms, err := client.ListRooms(ctx, token)
	if err != nil || len(rooms.Rooms) != 1 || rooms.Rooms[0].CurrentUserRole != "owner" {
		t.Fatalf("ListRooms() = %+v, %v", rooms, err)
	}
	created, err := client.CreateRoom(ctx, token, "create-room-1", relaycontract.CreateRoomInput{Name: "研发群"})
	if err != nil || created.Room.ID != "room-1" || created.Conversation.StreamEpoch != "epoch-1" {
		t.Fatalf("CreateRoom() = %+v, %v", created, err)
	}
	commit, err := client.PostMessage(ctx, token, "conversation-1", "command-1", relaycontract.CreateMessageInput{
		Content: relaycontract.MessageContent{
			Version: relaycontract.ContentVersionV1,
			Blocks:  []relaycontract.ContentBlock{{Type: relaycontract.BlockTypeMarkdown, Text: "hello"}},
		},
	})
	if err != nil || commit.Message.ID != "message-1" ||
		commit.Message.AuthorType != relaycontract.AuthorTypeUser || commit.Message.ClientMessageID != "command-1" {
		t.Fatalf("PostMessage() = %+v, %v", commit, err)
	}
	through, snapshotSeq := int64(9), int64(12)
	snapshot, err := client.Snapshot(ctx, token, "conversation-1", relaycontract.SnapshotOptions{
		AfterMessageSeq: 2, Limit: 100,
		ThroughMessageSeq: &through, SnapshotSeq: &snapshotSeq, StreamEpoch: "epoch-1",
	})
	if err != nil || snapshot.SnapshotSeq != 12 {
		t.Fatalf("Snapshot() = %+v, %v", snapshot, err)
	}
	difference, err := client.Difference(ctx, token, "stream-1", relaycontract.DifferenceOptions{
		AfterSeq: 5, Limit: 20, StreamEpoch: "epoch-1",
	})
	if err != nil || difference.NextSeq != 7 {
		t.Fatalf("Difference() = %+v, %v", difference, err)
	}
}

func TestClientMatchesRoomMembershipHTTPContract(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
		call   func(context.Context, *Client) error
	}{
		{name: "get room", method: http.MethodGet, path: "/api/relay/v1/rooms/room-1", call: func(ctx context.Context, client *Client) error {
			_, err := client.GetRoom(ctx, "token", "room-1")
			return err
		}},
		{name: "list invitations", method: http.MethodGet, path: "/api/relay/v1/invitations", call: func(ctx context.Context, client *Client) error {
			_, err := client.ListInvitations(ctx, "token")
			return err
		}},
		{name: "invite", method: http.MethodPost, path: "/api/relay/v1/rooms/room-1/invitations", call: func(ctx context.Context, client *Client) error {
			_, err := client.InviteUser(ctx, "token", "room-1", "command-1", relaycontract.InviteRoomMemberInput{UserID: "user-2", ExpectedMembershipVersion: 1})
			return err
		}},
		{name: "accept", method: http.MethodPost, path: "/api/relay/v1/rooms/room-1/invitations/accept", call: func(ctx context.Context, client *Client) error {
			_, err := client.AcceptInvitation(ctx, "token", "room-1", "command-1", relaycontract.ResolveRoomInvitationInput{ExpectedMembershipVersion: 1})
			return err
		}},
		{name: "reject", method: http.MethodPost, path: "/api/relay/v1/rooms/room-1/invitations/reject", call: func(ctx context.Context, client *Client) error {
			_, err := client.RejectInvitation(ctx, "token", "room-1", "command-1", relaycontract.ResolveRoomInvitationInput{ExpectedMembershipVersion: 1})
			return err
		}},
		{name: "revoke", method: http.MethodDelete, path: "/api/relay/v1/rooms/room-1/invitations/user-2", call: func(ctx context.Context, client *Client) error {
			_, err := client.RevokeInvitation(ctx, "token", "room-1", "user-2", "command-1", relaycontract.ResolveRoomInvitationInput{ExpectedMembershipVersion: 1})
			return err
		}},
		{name: "update", method: http.MethodPatch, path: "/api/relay/v1/rooms/room-1/members/user-2", call: func(ctx context.Context, client *Client) error {
			_, err := client.UpdateMember(ctx, "token", "room-1", "user-2", "command-1", relaycontract.UpdateRoomMemberInput{Role: "admin", ExpectedMembershipVersion: 1})
			return err
		}},
		{name: "update room", method: http.MethodPatch, path: "/api/relay/v1/rooms/room-1", call: func(ctx context.Context, client *Client) error {
			_, err := client.UpdateRoom(ctx, "token", "room-1", "command-1", relaycontract.UpdateRoomInput{CoordinatorAgentID: new("agent-1"), ExpectedConfigurationVersion: 1})
			return err
		}},
		{name: "pause agent", method: http.MethodPatch, path: "/api/relay/v1/rooms/room-1/agents/agent-1", call: func(ctx context.Context, client *Client) error {
			_, err := client.UpdateAgent(ctx, "token", "room-1", "agent-1", "command-1", relaycontract.UpdateRoomAgentInput{Paused: true, ExpectedMembershipVersion: 1})
			return err
		}},
		{name: "transfer", method: http.MethodPost, path: "/api/relay/v1/rooms/room-1/transfer", call: func(ctx context.Context, client *Client) error {
			_, err := client.TransferOwnership(ctx, "token", "room-1", "command-1", relaycontract.TransferRoomOwnershipInput{NewOwnerUserID: "user-2", ExpectedMembershipVersion: 1})
			return err
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				if request.Method != test.method || request.URL.Path != test.path || request.Header.Get("Authorization") != "Bearer token" {
					t.Errorf("request = %s %s auth=%q", request.Method, request.URL.Path, request.Header.Get("Authorization"))
				}
				if test.method != http.MethodGet && request.Header.Get("Idempotency-Key") != "command-1" {
					t.Errorf("Idempotency-Key = %q", request.Header.Get("Idempotency-Key"))
				}
				if test.name == "get room" {
					writeRelayTestData(t, writer, http.StatusOK, relaycontract.RoomDetails{RoomView: relaycontract.RoomView{Conversation: relaycontract.Conversation{StreamEpoch: "epoch-1"}}})
					return
				}
				if test.name == "list invitations" {
					writeRelayTestData(t, writer, http.StatusOK, relaycontract.RoomInvitationList{})
					return
				}
				writeRelayTestData(t, writer, http.StatusOK, relaycontract.RoomMembershipMutation{RoomID: "room-1", MembershipVersion: 2})
			}))
			t.Cleanup(server.Close)
			client, err := NewClient(server.URL, time.Second)
			if err != nil {
				t.Fatal(err)
			}
			if err = test.call(context.Background(), client); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestClientReturnsRelayErrorEnvelope(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"code": "full_snapshot_required", "message": "snapshot required", "request_id": "req-1",
		})
	}))
	t.Cleanup(server.Close)
	client, err := NewClient(server.URL, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Difference(context.Background(), "token", "stream-1", relaycontract.DifferenceOptions{Limit: 100})
	var remoteError *relaycontract.RemoteError
	if !errors.As(err, &remoteError) || remoteError.StatusCode != http.StatusConflict ||
		remoteError.Code != "full_snapshot_required" || remoteError.RequestID != "req-1" {
		t.Fatalf("Difference() error = %#v", err)
	}
}

func TestClientRejectsInvalidIdempotencyKey(t *testing.T) {
	client, err := NewClient("https://relay.example.com", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"", "contains space", "line\nbreak"} {
		if _, err = client.PostMessage(
			context.Background(), "token", "conversation-1", key, relaycontract.CreateMessageInput{},
		); err == nil {
			t.Fatalf("PostMessage() accepted Idempotency-Key %q", key)
		}
	}
}

func TestClientRequiresStreamEpochForNonzeroCursor(t *testing.T) {
	client, err := NewClient("https://relay.example.com", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = client.Snapshot(
		context.Background(), "token", "conversation-1", relaycontract.SnapshotOptions{AfterMessageSeq: 1, Limit: 100},
	); err == nil {
		t.Fatal("Snapshot() accepted a nonzero cursor without stream_epoch")
	}
	if _, err = client.Difference(
		context.Background(), "token", "stream-1", relaycontract.DifferenceOptions{AfterSeq: 1, Limit: 100},
	); err == nil {
		t.Fatal("Difference() accepted a nonzero cursor without stream_epoch")
	}
}

func TestClientRejectsChangedResponseStreamEpoch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writeRelayTestData(t, writer, http.StatusOK, relaycontract.Difference{StreamEpoch: "epoch-new"})
	}))
	t.Cleanup(server.Close)
	client, err := NewClient(server.URL, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Difference(context.Background(), "token", "stream-1", relaycontract.DifferenceOptions{
		AfterSeq: 1, Limit: 100, StreamEpoch: "epoch-old",
	})
	if err == nil {
		t.Fatal("Difference() accepted a response from another stream epoch")
	}
}

func TestClientRejectsTrailingResponseData(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(`{"code":"0000","message":"success","request_id":"req","data":{}} trailing`))
	}))
	t.Cleanup(server.Close)
	client, err := NewClient(server.URL, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = client.ListRooms(context.Background(), "token"); err == nil {
		t.Fatal("ListRooms() accepted trailing response data")
	}
}

func TestClientAcceptsFullRelayPageAfterJSONEscaping(t *testing.T) {
	messages := make([]relaycontract.Message, 70)
	for index := range messages {
		messages[index] = relaycontract.Message{
			ID:         "message",
			MessageSeq: int64(index + 1),
			Content: relaycontract.MessageContent{
				Version: relaycontract.ContentVersionV1,
				Blocks: []relaycontract.ContentBlock{{
					Type: relaycontract.BlockTypeMarkdown,
					Text: strings.Repeat(`\`, 60_000),
				}},
			},
		}
	}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writeRelayTestData(t, writer, http.StatusOK, relaycontract.Snapshot{StreamEpoch: "epoch-1", Messages: messages})
	}))
	t.Cleanup(server.Close)
	client, err := NewClient(server.URL, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := client.Snapshot(
		context.Background(), "token", "conversation-1", relaycontract.SnapshotOptions{Limit: 100},
	)
	if err != nil || len(snapshot.Messages) != len(messages) {
		t.Fatalf("Snapshot() messages=%d err=%v", len(snapshot.Messages), err)
	}
}

func TestClientDoesNotForwardCredentialsAcrossRedirect(t *testing.T) {
	redirected := make(chan struct{}, 1)
	target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		redirected <- struct{}{}
	}))
	t.Cleanup(target.Close)

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Location", target.URL+"/capture")
		writer.WriteHeader(http.StatusTemporaryRedirect)
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"code": "temporary_redirect", "message": "redirect", "request_id": "req-redirect",
		})
	}))
	t.Cleanup(server.Close)

	client, err := NewClient(server.URL, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.PostMessage(
		context.Background(),
		"relay-user-token",
		"conversation-1",
		"command-1",
		relaycontract.CreateMessageInput{},
	)
	var remoteError *relaycontract.RemoteError
	if !errors.As(err, &remoteError) || remoteError.StatusCode != http.StatusTemporaryRedirect {
		t.Fatalf("PostMessage() error = %#v", err)
	}
	select {
	case <-redirected:
		t.Fatal("Relay client followed redirect and forwarded the credential-bearing request")
	default:
	}
}

func writeRelayTestData(t *testing.T, writer http.ResponseWriter, status int, data any) {
	t.Helper()
	writer.WriteHeader(status)
	if err := json.NewEncoder(writer).Encode(map[string]any{
		"code": "0000", "message": "success", "request_id": "req-test", "data": data,
	}); err != nil {
		t.Errorf("encode response: %v", err)
	}
}
