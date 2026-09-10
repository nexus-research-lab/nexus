// INPUT: 认证上下文、Origin、Team HTTP 请求与 Relay/Control stub 结果。
// OUTPUT: 身份隔离、四端点转发、幂等键和稳定 FailureCore 断言。
// POS: Team gateway 消费侧合同回归；不启动真实 Control、Relay 或数据库。
package team

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/go-chi/chi/v5"

	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	relaycontract "github.com/nexus-research-lab/nexus/internal/relay"
	authsvc "github.com/nexus-research-lab/nexus/internal/service/auth"
	teamsvc "github.com/nexus-research-lab/nexus/internal/service/team"
)

type teamTokenStub struct {
	token     string
	err       error
	principal *authsvc.Principal
	calls     int
}

func (stub *teamTokenStub) ExchangeRelayUserToken(
	_ context.Context,
	principal *authsvc.Principal,
) (string, error) {
	stub.calls++
	stub.principal = principal
	return stub.token, stub.err
}

type teamRelayStub struct {
	err               error
	rooms             relaycontract.RoomList
	room              relaycontract.RoomView
	commit            relaycontract.MessageCommit
	snapshot          relaycontract.Snapshot
	difference        relaycontract.Difference
	tokens            []string
	conversationID    string
	idempotencyKey    string
	roomInput         relaycontract.CreateRoomInput
	messageInput      relaycontract.CreateMessageInput
	snapshotOptions   relaycontract.SnapshotOptions
	streamID          string
	differenceOptions relaycontract.DifferenceOptions
	listRoomsCalls    int
	createRoomCalls   int
	messageCalls      int
	snapshotCalls     int
	differenceCalls   int
	watchCalls        int
	watchStreamEpoch  string
	watchUpdate       relaycontract.StreamUpdated
	watchErr          error
}

type teamProjectorStub struct {
	err             error
	roomCalls       int
	commitCalls     int
	snapshotCalls   int
	differenceCalls int
}

func (stub *teamProjectorStub) ProjectRoom(context.Context, string, string, relaycontract.RoomView) error {
	stub.roomCalls++
	return stub.err
}

func (stub *teamProjectorStub) ProjectCommit(context.Context, string, relaycontract.MessageCommit) error {
	stub.commitCalls++
	return stub.err
}

func (stub *teamProjectorStub) ProjectSnapshot(context.Context, string, relaycontract.Snapshot) error {
	stub.snapshotCalls++
	return stub.err
}

func (stub *teamProjectorStub) ProjectDifference(context.Context, string, relaycontract.Difference) error {
	stub.differenceCalls++
	return stub.err
}

func (stub *teamRelayStub) ListRooms(
	_ context.Context,
	token string,
) (relaycontract.RoomList, error) {
	stub.listRoomsCalls++
	stub.tokens = append(stub.tokens, token)
	return stub.rooms, stub.err
}

func (stub *teamRelayStub) CreateRoom(
	_ context.Context,
	token string,
	idempotencyKey string,
	input relaycontract.CreateRoomInput,
) (relaycontract.RoomView, error) {
	stub.createRoomCalls++
	stub.tokens = append(stub.tokens, token)
	stub.idempotencyKey = idempotencyKey
	stub.roomInput = input
	return stub.room, stub.err
}

func (stub *teamRelayStub) PostMessage(
	_ context.Context,
	token string,
	conversationID string,
	idempotencyKey string,
	input relaycontract.CreateMessageInput,
) (relaycontract.MessageCommit, error) {
	stub.messageCalls++
	stub.tokens = append(stub.tokens, token)
	stub.conversationID = conversationID
	stub.idempotencyKey = idempotencyKey
	stub.messageInput = input
	return stub.commit, stub.err
}

func (stub *teamRelayStub) Snapshot(
	_ context.Context,
	token string,
	conversationID string,
	options relaycontract.SnapshotOptions,
) (relaycontract.Snapshot, error) {
	stub.snapshotCalls++
	stub.tokens = append(stub.tokens, token)
	stub.conversationID = conversationID
	stub.snapshotOptions = options
	return stub.snapshot, stub.err
}

func (stub *teamRelayStub) Difference(
	_ context.Context,
	token string,
	streamID string,
	options relaycontract.DifferenceOptions,
) (relaycontract.Difference, error) {
	stub.differenceCalls++
	stub.tokens = append(stub.tokens, token)
	stub.streamID = streamID
	stub.differenceOptions = options
	return stub.difference, stub.err
}

func (stub *teamRelayStub) Watch(
	_ context.Context,
	token string,
	streamID string,
	streamEpoch string,
	handle func(relaycontract.StreamUpdated) error,
) error {
	stub.watchCalls++
	stub.tokens = append(stub.tokens, token)
	stub.streamID = streamID
	stub.watchStreamEpoch = streamEpoch
	if stub.watchUpdate.Type != "" {
		if err := handle(stub.watchUpdate); err != nil {
			return err
		}
	}
	return stub.watchErr
}

func TestTeamStreamForwardsAuthenticatedRelayHints(t *testing.T) {
	update := relaycontract.StreamUpdated{
		Type: "stream.updated", StreamID: "stream-1", StreamEpoch: "epoch-1", HighWaterSeq: 9,
	}
	tokens := &teamTokenStub{token: "fixed-relay-token"}
	relay := &teamRelayStub{watchUpdate: update, watchErr: errors.New("stop")}
	server := httptest.NewServer(newTeamTestRouter(tokens, relay, teamTestPrincipal()))
	t.Cleanup(server.Close)
	endpoint := "ws" + strings.TrimPrefix(server.URL, "http") +
		"/nexus/v1/team/stream?stream_id=stream-1&stream_epoch=epoch-1"
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	connection, _, err := websocket.Dial(ctx, endpoint, &websocket.DialOptions{
		HTTPHeader: http.Header{"Origin": {server.URL}},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer connection.CloseNow()
	var got relaycontract.StreamUpdated
	if err = wsjson.Read(ctx, connection, &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, update) || relay.watchCalls != 1 ||
		relay.streamID != "stream-1" || relay.watchStreamEpoch != "epoch-1" ||
		tokens.principal == nil || tokens.principal.UserID != "local-owner" {
		t.Fatalf("update=%+v relay=%+v principal=%+v", got, relay, tokens.principal)
	}
}

func TestTeamStreamRejectsMissingOriginBeforeRelay(t *testing.T) {
	relay := &teamRelayStub{}
	recorder := teamRequest(
		t,
		newTeamTestRouter(&teamTokenStub{token: "token"}, relay, teamTestPrincipal()),
		http.MethodGet,
		"/nexus/v1/team/stream?stream_id=stream-1&stream_epoch=epoch-1",
		"",
		false,
	)
	if recorder.Code != http.StatusForbidden || relay.watchCalls != 0 {
		t.Fatalf("status=%d relay watch calls=%d", recorder.Code, relay.watchCalls)
	}
}

func TestTeamStreamForwardsSnapshotReset(t *testing.T) {
	relay := &teamRelayStub{watchErr: &relaycontract.RemoteError{
		StatusCode: http.StatusConflict,
		Code:       "full_snapshot_required",
	}}
	server := httptest.NewServer(newTeamTestRouter(
		&teamTokenStub{token: "token"}, relay, teamTestPrincipal(),
	))
	t.Cleanup(server.Close)
	endpoint := "ws" + strings.TrimPrefix(server.URL, "http") +
		"/nexus/v1/team/stream?stream_id=stream-1&stream_epoch=epoch-old"
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	connection, _, err := websocket.Dial(ctx, endpoint, &websocket.DialOptions{
		HTTPHeader: http.Header{"Origin": {server.URL}},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer connection.CloseNow()
	var got streamResetRequired
	if err = wsjson.Read(ctx, connection, &got); err != nil {
		t.Fatal(err)
	}
	if got.Type != "stream.reset_required" || got.StreamID != "stream-1" ||
		got.Reason != "full_snapshot_required" {
		t.Fatalf("reset = %+v", got)
	}
}

func TestTeamDifferenceDoesNotHideProjectionFailure(t *testing.T) {
	projector := &teamProjectorStub{err: errors.New("local database unavailable")}
	recorder := teamRequest(
		t,
		newTeamTestRouterWithProjector(
			&teamTokenStub{token: "token"},
			&teamRelayStub{difference: relaycontract.Difference{StreamEpoch: "epoch-1"}},
			projector,
			teamTestPrincipal(),
		),
		http.MethodGet,
		"/nexus/v1/team/sync-streams/stream-1/difference?after_seq=0&limit=100",
		"",
		false,
	)
	failure := decodeTeamFailure(t, recorder)
	if recorder.Code != http.StatusServiceUnavailable ||
		failure.Code != "team.local_projection_failed" || projector.differenceCalls != 1 {
		t.Fatalf("status=%d failure=%+v projector=%+v", recorder.Code, failure, projector)
	}
}

func TestTeamMessageKeepsCommittedSuccessWhenProjectionFails(t *testing.T) {
	projector := &teamProjectorStub{err: errors.New("local database unavailable")}
	relay := &teamRelayStub{commit: relaycontract.MessageCommit{
		Message:     relaycontract.Message{ID: "message-1"},
		StreamEpoch: "epoch-1",
	}}
	recorder := teamRequest(
		t,
		newTeamTestRouterWithProjector(
			&teamTokenStub{token: "token"}, relay, projector, teamTestPrincipal(),
		),
		http.MethodPost,
		"/nexus/v1/team/conversations/conversation-1/messages",
		`{"content":{"version":1,"blocks":[{"type":"markdown","text":"hello"}]}}`,
		true,
	)
	if recorder.Code != http.StatusOK || projector.commitCalls != 1 {
		t.Fatalf("status=%d projector=%+v body=%s", recorder.Code, projector, recorder.Body.String())
	}
}

func TestTeamHandlersUseAuthenticatedPrincipalAndFixedRelayToken(t *testing.T) {
	sessionID := "session-1"
	principal := &authsvc.Principal{
		UserID: "local-owner", ControlUserID: "control-user", DeploymentID: "deployment-1",
		Username: "lee", AuthMethod: authsvc.AuthMethodPassword, SessionID: &sessionID,
	}
	tokens := &teamTokenStub{token: "fixed-relay-token"}
	relay := &teamRelayStub{
		commit: relaycontract.MessageCommit{Message: relaycontract.Message{
			ID: "message-1", AuthorType: relaycontract.AuthorTypeUser, ClientMessageID: "command-1",
		}},
		snapshot:   relaycontract.Snapshot{SnapshotSeq: 12},
		difference: relaycontract.Difference{NextSeq: 7},
	}
	router := newTeamTestRouter(tokens, relay, principal)

	message := teamRequest(
		t,
		router,
		http.MethodPost,
		"/nexus/v1/team/conversations/conversation-1/messages",
		`{"content":{"version":1,"blocks":[{"type":"markdown","text":"hello"}]}}`,
		true,
	)
	if message.Code != http.StatusOK || message.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("message status=%d cache=%q body=%s", message.Code, message.Header().Get("Cache-Control"), message.Body.String())
	}
	var messageEnvelope struct {
		Data relaycontract.MessageCommit `json:"data"`
	}
	if err := json.Unmarshal(message.Body.Bytes(), &messageEnvelope); err != nil {
		t.Fatal(err)
	}
	if messageEnvelope.Data.Message.AuthorType != relaycontract.AuthorTypeUser ||
		messageEnvelope.Data.Message.ClientMessageID != "command-1" {
		t.Fatalf("message identity fields lost: %+v", messageEnvelope.Data.Message)
	}

	snapshot := teamRequest(
		t,
		router,
		http.MethodGet,
		"/nexus/v1/team/conversations/conversation-1/snapshot?"+
			"after_message_seq=2&limit=100&through_message_seq=9&snapshot_seq=12&stream_epoch=epoch-1",
		"",
		false,
	)
	if snapshot.Code != http.StatusOK {
		t.Fatalf("snapshot status=%d body=%s", snapshot.Code, snapshot.Body.String())
	}
	difference := teamRequest(
		t,
		router,
		http.MethodGet,
		"/nexus/v1/team/sync-streams/stream-1/difference?after_seq=5&limit=20&stream_epoch=epoch-1",
		"",
		false,
	)
	if difference.Code != http.StatusOK {
		t.Fatalf("difference status=%d body=%s", difference.Code, difference.Body.String())
	}

	if tokens.calls != 3 || tokens.principal != principal {
		t.Fatalf("token exchange = calls %d principal %+v", tokens.calls, tokens.principal)
	}
	if !reflect.DeepEqual(relay.tokens, []string{
		"fixed-relay-token", "fixed-relay-token", "fixed-relay-token",
	}) {
		t.Fatalf("Relay tokens = %#v", relay.tokens)
	}
	if relay.conversationID != "conversation-1" || relay.idempotencyKey != "command-1" ||
		relay.messageInput.Content.Version != relaycontract.ContentVersionV1 ||
		len(relay.messageInput.Content.Blocks) != 1 ||
		relay.messageInput.Content.Blocks[0].Text != "hello" {
		t.Fatalf("message relay input = id %q key %q input %+v", relay.conversationID, relay.idempotencyKey, relay.messageInput)
	}
	if relay.snapshotOptions.AfterMessageSeq != 2 || relay.snapshotOptions.Limit != 100 ||
		relay.snapshotOptions.StreamEpoch != "epoch-1" ||
		relay.snapshotOptions.ThroughMessageSeq == nil || *relay.snapshotOptions.ThroughMessageSeq != 9 ||
		relay.snapshotOptions.SnapshotSeq == nil || *relay.snapshotOptions.SnapshotSeq != 12 {
		t.Fatalf("snapshot options = %+v", relay.snapshotOptions)
	}
	if relay.streamID != "stream-1" || relay.differenceOptions != (relaycontract.DifferenceOptions{
		AfterSeq: 5, Limit: 20, StreamEpoch: "epoch-1",
	}) {
		t.Fatalf("difference input = stream %q options %+v", relay.streamID, relay.differenceOptions)
	}
}

func TestTeamHandlersCreateAndListExplicitRooms(t *testing.T) {
	view := relaycontract.RoomView{
		Room: relaycontract.Room{ID: "room-1", TeamID: "team-1", Name: "研发群"},
		Conversation: relaycontract.Conversation{
			ID: "conversation-1", RoomID: "room-1", SyncStreamID: "stream-1", StreamEpoch: "epoch-1",
		},
		CurrentUserRole: "owner",
	}
	relay := &teamRelayStub{room: view, rooms: relaycontract.RoomList{Rooms: []relaycontract.RoomView{view}}}
	projector := &teamProjectorStub{}
	router := newTeamTestRouterWithProjector(
		&teamTokenStub{token: "relay-token"}, relay, projector, teamTestPrincipal(),
	)
	created := teamRequest(
		t, router, http.MethodPost, "/nexus/v1/team/rooms", `{"name":"研发群","avatar":"room://avatar","agent_ids":["agent-1"],"member_user_ids":["user-2"]}`, true,
	)
	listed := teamRequest(t, router, http.MethodGet, "/nexus/v1/team/rooms", "", false)
	if created.Code != http.StatusOK || listed.Code != http.StatusOK ||
		relay.createRoomCalls != 1 || relay.listRoomsCalls != 1 ||
		relay.idempotencyKey != "command-1" || projector.roomCalls != 2 {
		t.Fatalf("create=%d list=%d relay=%+v projector=%+v", created.Code, listed.Code, relay, projector)
	}
	if relay.roomInput.Avatar != "room://avatar" || !reflect.DeepEqual(relay.roomInput.AgentIDs, []string{"agent-1"}) ||
		!reflect.DeepEqual(relay.roomInput.MemberUserIDs, []string{"user-2"}) {
		t.Fatalf("room input = %+v", relay.roomInput)
	}
}

func TestTeamMutationsRequireStrictSameOrigin(t *testing.T) {
	tests := []struct {
		name      string
		origin    string
		forwarded string
		status    int
		calls     int
	}{
		{name: "missing", status: http.StatusForbidden},
		{name: "cross origin", origin: "https://evil.example", status: http.StatusForbidden},
		{name: "same origin", origin: "http://example.com", status: http.StatusOK, calls: 1},
		{name: "same origin explicit port", origin: "http://example.com:8443", status: http.StatusOK, calls: 1},
		{name: "same origin behind tls proxy", origin: "https://example.com", forwarded: "https", status: http.StatusOK, calls: 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tokens := &teamTokenStub{token: "relay-token"}
			relay := &teamRelayStub{}
			router := newTeamTestRouter(tokens, relay, teamTestPrincipal())
			request := httptest.NewRequest(http.MethodPost, "/nexus/v1/team/rooms", strings.NewReader(`{"name":"研发群"}`))
			if strings.Contains(test.origin, ":8443") {
				request.Host = "example.com:8443"
			}
			request.Header.Set("Origin", test.origin)
			request.Header.Set("X-Forwarded-Proto", test.forwarded)
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Idempotency-Key", "command-1")
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)
			if recorder.Code != test.status || tokens.calls != test.calls || relay.createRoomCalls != test.calls {
				t.Fatalf(
					"status=%d token calls=%d relay calls=%d body=%s",
					recorder.Code,
					tokens.calls,
					relay.createRoomCalls,
					recorder.Body.String(),
				)
			}
		})
	}
}

func TestTeamHandlersRejectClientIdentityFields(t *testing.T) {
	tests := []struct {
		name string
		path string
		body string
	}{
		{name: "room user", path: "/nexus/v1/team/rooms", body: `{"name":"研发群","user_id":"forged"}`},
		{
			name: "message deployment",
			path: "/nexus/v1/team/conversations/conversation-1/messages",
			body: `{"deployment_id":"forged","content":{"version":1,"blocks":[]}}`,
		},
		{
			name: "message audience",
			path: "/nexus/v1/team/conversations/conversation-1/messages",
			body: `{"audience":"nexus-relay-node","content":{"version":1,"blocks":[]}}`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tokens := &teamTokenStub{token: "relay-token"}
			relay := &teamRelayStub{}
			router := newTeamTestRouter(tokens, relay, teamTestPrincipal())
			request := httptest.NewRequest(http.MethodPost, test.path, strings.NewReader(test.body))
			request.Header.Set("Origin", "http://example.com")
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Idempotency-Key", "command-1")
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)
			if recorder.Code != http.StatusBadRequest || tokens.calls != 0 ||
				relay.createRoomCalls != 0 || relay.messageCalls != 0 {
				t.Fatalf(
					"status=%d token calls=%d relay calls=(%d,%d) body=%s",
					recorder.Code,
					tokens.calls,
					relay.createRoomCalls,
					relay.messageCalls,
					recorder.Body.String(),
				)
			}
		})
	}
}

func TestTeamMessageAcceptsMaximumContentAfterJSONEscaping(t *testing.T) {
	payload, err := json.Marshal(relaycontract.CreateMessageInput{Content: relaycontract.MessageContent{
		Version: relaycontract.ContentVersionV1,
		Blocks: []relaycontract.ContentBlock{{
			Type: relaycontract.BlockTypeMarkdown,
			Text: strings.Repeat("\\", 64*1024),
		}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	tokens := &teamTokenStub{token: "relay-token"}
	relay := &teamRelayStub{}
	recorder := teamRequest(
		t,
		newTeamTestRouter(tokens, relay, teamTestPrincipal()),
		http.MethodPost,
		"/nexus/v1/team/conversations/conversation-1/messages",
		string(payload),
		true,
	)
	if recorder.Code != http.StatusOK || relay.messageCalls != 1 {
		t.Fatalf("status=%d calls=%d body=%s", recorder.Code, relay.messageCalls, recorder.Body.String())
	}
}

func TestTeamHandlersFailClosedWithoutAuthenticatedPrincipal(t *testing.T) {
	tokens := &teamTokenStub{token: "relay-token"}
	relay := &teamRelayStub{}
	router := newTeamTestRouter(tokens, relay, nil)
	recorder := teamRequest(
		t,
		router,
		http.MethodPost,
		"/nexus/v1/team/conversations/conversation-1/messages",
		`{"content":{"version":1,"blocks":[]}}`,
		true,
	)
	failure := decodeTeamFailure(t, recorder)
	if recorder.Code != http.StatusUnauthorized || failure.Effect != protocol.FailureEffectNotApplied ||
		tokens.calls != 0 || relay.messageCalls != 0 {
		t.Fatalf("status=%d failure=%+v token calls=%d relay calls=%d body=%s", recorder.Code, failure, tokens.calls, relay.messageCalls, recorder.Body.String())
	}
}

func TestTeamHandlersRequireStreamEpochForNonzeroCursor(t *testing.T) {
	paths := []string{
		"/nexus/v1/team/conversations/conversation-1/snapshot?after_message_seq=1&limit=100",
		"/nexus/v1/team/sync-streams/stream-1/difference?after_seq=1&limit=100",
	}
	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			tokens := &teamTokenStub{token: "relay-token"}
			relay := &teamRelayStub{}
			recorder := teamRequest(
				t, newTeamTestRouter(tokens, relay, teamTestPrincipal()), http.MethodGet, path, "", false,
			)
			if recorder.Code != http.StatusBadRequest || tokens.calls != 0 ||
				relay.snapshotCalls != 0 || relay.differenceCalls != 0 {
				t.Fatalf(
					"status=%d token calls=%d relay calls=(%d,%d) body=%s",
					recorder.Code,
					tokens.calls,
					relay.snapshotCalls,
					relay.differenceCalls,
					recorder.Body.String(),
				)
			}
		})
	}
}

func TestTeamHandlersMapStableRelayFailures(t *testing.T) {
	tests := []struct {
		name     string
		method   string
		path     string
		err      error
		status   int
		code     string
		category protocol.FailureCategory
		effect   protocol.FailureEffect
	}{
		{
			name: "message invalid", method: http.MethodPost,
			path:   "/nexus/v1/team/conversations/conversation-1/messages",
			err:    &relaycontract.RemoteError{StatusCode: 400, Code: "request_invalid", Message: "secret"},
			status: http.StatusBadRequest, code: "team.request_invalid",
			category: protocol.FailureCategoryValidation, effect: protocol.FailureEffectNotApplied,
		},
		{
			name: "message too large", method: http.MethodPost,
			path:   "/nexus/v1/team/conversations/conversation-1/messages",
			err:    &relaycontract.RemoteError{StatusCode: 413, Code: "message_too_large"},
			status: http.StatusRequestEntityTooLarge, code: "team.message_too_large",
			category: protocol.FailureCategoryValidation, effect: protocol.FailureEffectNotApplied,
		},
		{
			name: "idempotency conflict", method: http.MethodPost,
			path:   "/nexus/v1/team/conversations/conversation-1/messages",
			err:    &relaycontract.RemoteError{StatusCode: 409, Code: "idempotency_conflict"},
			status: http.StatusConflict, code: "team.idempotency_conflict",
			category: protocol.FailureCategoryConflict, effect: protocol.FailureEffectNotApplied,
		},
		{
			name: "resource missing", method: http.MethodGet,
			path:   "/nexus/v1/team/conversations/missing/snapshot",
			err:    &relaycontract.RemoteError{StatusCode: 404, Code: "resource_not_found"},
			status: http.StatusNotFound, code: "team.resource_not_found",
			category: protocol.FailureCategoryNotFound, effect: protocol.FailureEffectNotApplicable,
		},
		{
			name: "cursor ahead", method: http.MethodGet,
			path:   "/nexus/v1/team/sync-streams/stream-1/difference",
			err:    &relaycontract.RemoteError{StatusCode: 409, Code: "cursor_ahead"},
			status: http.StatusConflict, code: "team.cursor_ahead",
			category: protocol.FailureCategoryConflict, effect: protocol.FailureEffectNotApplicable,
		},
		{
			name: "snapshot required", method: http.MethodGet,
			path:   "/nexus/v1/team/sync-streams/stream-1/difference",
			err:    &relaycontract.RemoteError{StatusCode: 409, Code: "full_snapshot_required"},
			status: http.StatusConflict, code: "team.full_snapshot_required",
			category: protocol.FailureCategoryConflict, effect: protocol.FailureEffectNotApplicable,
		},
		{
			name: "relay identity rejected", method: http.MethodPost,
			path:   "/nexus/v1/team/conversations/conversation-1/messages",
			err:    &relaycontract.RemoteError{StatusCode: 401, Code: "principal_invalid"},
			status: http.StatusBadGateway, code: "team.identity_rejected",
			category: protocol.FailureCategoryUnavailable, effect: protocol.FailureEffectNotApplied,
		},
		{
			name: "relay unavailable", method: http.MethodPost,
			path:   "/nexus/v1/team/conversations/conversation-1/messages",
			err:    &relaycontract.RemoteError{StatusCode: 503, Code: "relay_unavailable"},
			status: http.StatusServiceUnavailable, code: "team.unavailable",
			category: protocol.FailureCategoryUnavailable, effect: protocol.FailureEffectUnknown,
		},
		{
			name: "unknown upstream", method: http.MethodPost,
			path:   "/nexus/v1/team/conversations/conversation-1/messages",
			err:    errors.New("upstream response secret"),
			status: http.StatusBadGateway, code: "team.upstream_failed",
			category: protocol.FailureCategoryUnavailable, effect: protocol.FailureEffectUnknown,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tokens := &teamTokenStub{token: "relay-token"}
			relay := &teamRelayStub{err: test.err}
			router := newTeamTestRouter(tokens, relay, teamTestPrincipal())
			body := ""
			origin := false
			if test.method == http.MethodPost {
				body = `{"content":{"version":1,"blocks":[{"type":"markdown","text":"hello"}]}}`
				origin = true
			}
			recorder := teamRequest(t, router, test.method, test.path, body, origin)
			failure := decodeTeamFailure(t, recorder)
			if recorder.Code != test.status || failure.Code != test.code ||
				failure.Category != test.category || failure.Effect != test.effect {
				t.Fatalf("status=%d failure=%+v body=%s", recorder.Code, failure, recorder.Body.String())
			}
			if strings.Contains(recorder.Body.String(), "secret") {
				t.Fatalf("upstream detail leaked: %s", recorder.Body.String())
			}
		})
	}
}

func TestTeamMessageIdentityExchangeFailureIsKnownNotApplied(t *testing.T) {
	tokens := &teamTokenStub{err: errors.New("control secret")}
	relay := &teamRelayStub{}
	router := newTeamTestRouter(tokens, relay, teamTestPrincipal())
	recorder := teamRequest(
		t,
		router,
		http.MethodPost,
		"/nexus/v1/team/conversations/conversation-1/messages",
		`{"content":{"version":1,"blocks":[]}}`,
		true,
	)
	failure := decodeTeamFailure(t, recorder)
	if recorder.Code != http.StatusBadGateway || failure.Code != "team.identity_exchange_failed" ||
		failure.Effect != protocol.FailureEffectNotApplied || relay.messageCalls != 0 ||
		strings.Contains(recorder.Body.String(), "control secret") {
		t.Fatalf("status=%d failure=%+v relay calls=%d body=%s", recorder.Code, failure, relay.messageCalls, recorder.Body.String())
	}
}

func TestTeamRoomsRejectLocalPrincipalBeforeTokenExchange(t *testing.T) {
	tokens := &teamTokenStub{token: "must-not-be-used"}
	relay := &teamRelayStub{}
	recorder := teamRequest(
		t,
		newTeamTestRouter(tokens, relay, &authsvc.Principal{
			UserID: authsvc.SystemUserID, AuthMethod: authsvc.AuthMethodLocal,
		}),
		http.MethodGet,
		"/nexus/v1/team/rooms",
		"",
		false,
	)
	failure := decodeTeamFailure(t, recorder)
	if recorder.Code != http.StatusForbidden || failure.Code != "team.remote_account_required" ||
		failure.Effect != protocol.FailureEffectNotApplicable || tokens.calls != 0 || relay.listRoomsCalls != 0 {
		t.Fatalf("status=%d failure=%+v token calls=%d relay calls=%d", recorder.Code, failure, tokens.calls, relay.listRoomsCalls)
	}
}

func newTeamTestRouter(
	tokens relayTokenExchanger,
	relay relayClient,
	principal *authsvc.Principal,
) http.Handler {
	return newTeamTestRouterWithProjector(tokens, relay, nil, principal)
}

func newTeamTestRouterWithProjector(
	tokens relayTokenExchanger,
	relay relayClient,
	projector teamsvc.Projector,
	principal *authsvc.Principal,
) http.Handler {
	api := handlershared.NewAPI(slog.New(slog.NewTextHandler(io.Discard, nil)))
	handler := New(api, tokens, teamsvc.New(relay, projector, api.BaseLogger()), relay)
	router := chi.NewRouter()
	router.Use(handlershared.RequestContextMiddleware(api.BaseLogger()))
	if principal != nil {
		router.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				ctx := authsvc.WithPrincipal(request.Context(), principal)
				next.ServeHTTP(writer, request.WithContext(ctx))
			})
		})
	}
	router.Get("/nexus/v1/team/rooms", handler.HandleListRooms)
	router.Post("/nexus/v1/team/rooms", handler.HandleCreateRoom)
	router.Post("/nexus/v1/team/conversations/{conversation_id}/messages", handler.HandlePostMessage)
	router.Get("/nexus/v1/team/conversations/{conversation_id}/snapshot", handler.HandleSnapshot)
	router.Get("/nexus/v1/team/sync-streams/{stream_id}/difference", handler.HandleDifference)
	router.Get("/nexus/v1/team/stream", handler.HandleStream)
	return router
}

func teamRequest(
	t *testing.T,
	router http.Handler,
	method string,
	path string,
	body string,
	mutation bool,
) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("X-Request-ID", "team-request")
	request.Header.Set("Authorization", "Bearer forged-browser-token")
	request.AddCookie(&http.Cookie{Name: "nexus_session", Value: "opaque-browser-session"})
	if mutation {
		request.Header.Set("Origin", "http://example.com")
		request.Header.Set("Idempotency-Key", "command-1")
		request.Header.Set("Content-Type", "application/json")
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

func teamTestPrincipal() *authsvc.Principal {
	sessionID := "session-1"
	return &authsvc.Principal{
		UserID: "local-owner", ControlUserID: "control-user", DeploymentID: "deployment-1",
		Username: "lee", AuthMethod: authsvc.AuthMethodPassword, SessionID: &sessionID,
	}
}

func decodeTeamFailure(t *testing.T, recorder *httptest.ResponseRecorder) protocol.FailureCore {
	t.Helper()
	var envelope struct {
		Data struct {
			Failure protocol.FailureCore `json:"failure"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode failure: %v body=%s", err, recorder.Body.String())
	}
	return envelope.Data.Failure
}

type relayClient interface {
	teamsvc.RelayClient
	relayStream
}
