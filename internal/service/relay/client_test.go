package relay

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
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
		if err = wsjson.Write(request.Context(), connection, StreamUpdated{
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
	err = client.Watch(context.Background(), token, "stream-1", "epoch-1", func(update StreamUpdated) error {
		if update.HighWaterSeq != 7 {
			t.Fatalf("high water = %d", update.HighWaterSeq)
		}
		return stop
	})
	if !errors.Is(err, stop) {
		t.Fatalf("Watch() error = %v", err)
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
	err = client.Watch(context.Background(), "token", "stream-1", "epoch-old", func(StreamUpdated) error {
		return nil
	})
	var remoteError *RemoteError
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
		case "POST /api/relay/v1/bootstrap":
			body, _ := io.ReadAll(request.Body)
			if len(body) != 0 {
				t.Errorf("bootstrap body = %q", body)
			}
			writeRelayTestData(t, writer, http.StatusOK, Bootstrap{
				Team: Team{ID: "team-1", DeploymentID: "deployment-1", Name: "Nexus"},
				Room: Room{ID: "room-1", TeamID: "team-1", Name: "General"},
				Conversation: Conversation{
					ID: "conversation-1", RoomID: "room-1", Type: "main",
					HighWaterMessageSeq: 3, SyncStreamID: "stream-1", StreamEpoch: "epoch-1",
					HighWaterSyncEventSeq: 4,
				},
			})
		case "POST /api/relay/v1/conversations/conversation-1/messages":
			if request.Header.Get("Idempotency-Key") != "command-1" {
				t.Errorf("Idempotency-Key = %q", request.Header.Get("Idempotency-Key"))
			}
			var input CreateMessageInput
			if err := json.NewDecoder(request.Body).Decode(&input); err != nil {
				t.Errorf("decode message: %v", err)
			}
			want := CreateMessageInput{Content: MessageContent{
				Version: ContentVersionV1,
				Blocks:  []ContentBlock{{Type: BlockTypeMarkdown, Text: "hello"}},
			}}
			if !reflect.DeepEqual(input, want) {
				t.Errorf("message input = %#v", input)
			}
			writeRelayTestData(t, writer, http.StatusCreated, MessageCommit{
				Message: Message{
					ID: "message-1", ConversationID: "conversation-1", MessageSeq: 4,
					AuthorType: AuthorTypeUser, ClientMessageID: "command-1",
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
			writeRelayTestData(t, writer, http.StatusOK, Snapshot{
				ConversationID: "conversation-1", StreamID: "stream-1",
				StreamEpoch: "epoch-1", SnapshotSeq: 12, ThroughMessageSeq: 9, AfterMessageSeq: 2,
			})
		case "GET /api/relay/v1/sync-streams/stream-1/difference":
			query := request.URL.Query()
			if query.Get("after_seq") != "5" || query.Get("limit") != "20" ||
				query.Get("stream_epoch") != "epoch-1" {
				t.Errorf("difference query = %q", request.URL.RawQuery)
			}
			writeRelayTestData(t, writer, http.StatusOK, Difference{
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
	bootstrap, err := client.Bootstrap(ctx, token)
	if err != nil || bootstrap.Conversation.SyncStreamID != "stream-1" {
		t.Fatalf("Bootstrap() = %+v, %v", bootstrap, err)
	}
	commit, err := client.PostMessage(ctx, token, "conversation-1", "command-1", CreateMessageInput{
		Content: MessageContent{
			Version: ContentVersionV1,
			Blocks:  []ContentBlock{{Type: BlockTypeMarkdown, Text: "hello"}},
		},
	})
	if err != nil || commit.Message.ID != "message-1" ||
		commit.Message.AuthorType != AuthorTypeUser || commit.Message.ClientMessageID != "command-1" {
		t.Fatalf("PostMessage() = %+v, %v", commit, err)
	}
	through, snapshotSeq := int64(9), int64(12)
	snapshot, err := client.Snapshot(ctx, token, "conversation-1", SnapshotOptions{
		AfterMessageSeq: 2, Limit: 100,
		ThroughMessageSeq: &through, SnapshotSeq: &snapshotSeq, StreamEpoch: "epoch-1",
	})
	if err != nil || snapshot.SnapshotSeq != 12 {
		t.Fatalf("Snapshot() = %+v, %v", snapshot, err)
	}
	difference, err := client.Difference(ctx, token, "stream-1", DifferenceOptions{
		AfterSeq: 5, Limit: 20, StreamEpoch: "epoch-1",
	})
	if err != nil || difference.NextSeq != 7 {
		t.Fatalf("Difference() = %+v, %v", difference, err)
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
	_, err = client.Difference(context.Background(), "token", "stream-1", DifferenceOptions{Limit: 100})
	var remoteError *RemoteError
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
			context.Background(), "token", "conversation-1", key, CreateMessageInput{},
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
		context.Background(), "token", "conversation-1", SnapshotOptions{AfterMessageSeq: 1, Limit: 100},
	); err == nil {
		t.Fatal("Snapshot() accepted a nonzero cursor without stream_epoch")
	}
	if _, err = client.Difference(
		context.Background(), "token", "stream-1", DifferenceOptions{AfterSeq: 1, Limit: 100},
	); err == nil {
		t.Fatal("Difference() accepted a nonzero cursor without stream_epoch")
	}
}

func TestClientRejectsChangedResponseStreamEpoch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writeRelayTestData(t, writer, http.StatusOK, Difference{StreamEpoch: "epoch-new"})
	}))
	t.Cleanup(server.Close)
	client, err := NewClient(server.URL, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Difference(context.Background(), "token", "stream-1", DifferenceOptions{
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
	if _, err = client.Bootstrap(context.Background(), "token"); err == nil {
		t.Fatal("Bootstrap() accepted trailing response data")
	}
}

func TestClientAcceptsFullRelayPageAfterJSONEscaping(t *testing.T) {
	messages := make([]Message, 70)
	for index := range messages {
		messages[index] = Message{
			ID:         "message",
			MessageSeq: int64(index + 1),
			Content: MessageContent{
				Version: ContentVersionV1,
				Blocks: []ContentBlock{{
					Type: BlockTypeMarkdown,
					Text: strings.Repeat(`\`, 60_000),
				}},
			},
		}
	}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writeRelayTestData(t, writer, http.StatusOK, Snapshot{StreamEpoch: "epoch-1", Messages: messages})
	}))
	t.Cleanup(server.Close)
	client, err := NewClient(server.URL, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := client.Snapshot(
		context.Background(), "token", "conversation-1", SnapshotOptions{Limit: 100},
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
		CreateMessageInput{},
	)
	var remoteError *RemoteError
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
