package relay

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/logx"
	relaycontract "github.com/nexus-research-lab/nexus/internal/relay"
)

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

func TestClientForwardsRequestIDAndHonorsGatewayRateLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("X-Request-ID") != "trace-1" {
			t.Errorf("X-Request-ID = %q", request.Header.Get("X-Request-ID"))
		}
		// 网关限流页不是 Relay JSON 信封，仍要保留状态与等待时长。
		writer.Header().Set("Retry-After", "7")
		writer.WriteHeader(http.StatusTooManyRequests)
		_, _ = writer.Write([]byte("<html>slow down</html>"))
	}))
	t.Cleanup(server.Close)
	client, err := NewClient(server.URL, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	ctx := logx.WithRequestID(context.Background(), "trace-1")
	_, err = client.Difference(ctx, "token", "stream-1", relaycontract.DifferenceOptions{Limit: 100})
	var remoteError *relaycontract.RemoteError
	if !errors.As(err, &remoteError) || remoteError.StatusCode != http.StatusTooManyRequests || remoteError.RetryDelay() != 7*time.Second {
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

func writeRelayTestData(t *testing.T, writer http.ResponseWriter, status int, data any) {
	t.Helper()
	writer.WriteHeader(status)
	if err := json.NewEncoder(writer).Encode(map[string]any{
		"code": "0000", "message": "success", "request_id": "req-test", "data": data,
	}); err != nil {
		t.Errorf("encode response: %v", err)
	}
}

func TestGetRoomWithMembersFollowsVersionFencedPages(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if strings.HasSuffix(r.URL.Path, "/members") {
			if r.URL.Query().Get("after") != "user:first" || r.URL.Query().Get("membership_version") != "7" || r.URL.Query().Get("stream_epoch") != "epoch" {
				t.Errorf("unfenced query: %s", r.URL)
			}
			writeRelayTestData(t, w, http.StatusOK, relaycontract.RoomMemberPage{Members: []relaycontract.RoomMember{{ID: "last"}}})
			return
		}
		writeRelayTestData(t, w, http.StatusOK, relaycontract.RoomDetails{RoomView: relaycontract.RoomView{Room: relaycontract.Room{MembershipVersion: 7}, Conversation: relaycontract.Conversation{StreamEpoch: "epoch"}}, Members: []relaycontract.RoomMember{{ID: "first"}}, NextMemberCursor: "user:first"})
	}))
	defer server.Close()
	client, err := NewClient(server.URL, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.GetRoomWithMembers(t.Context(), "token", "room")
	if err != nil || calls != 2 || len(result.Members) != 2 || result.Members[1].ID != "last" || result.NextMemberCursor != "" {
		t.Fatalf("partial members: %+v %v calls=%d", result, err, calls)
	}
}
