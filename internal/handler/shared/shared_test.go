// shared_test.go 覆盖公共 Handler API、失败分类、中间件与 WebSocket 发送器测试。
package shared

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	authsvc "github.com/nexus-research-lab/nexus/internal/service/auth"
)

func TestWriteErrorDoesNotExposeCauseOrInventRequestID(t *testing.T) {
	api := newFailureTestAPI()
	request := httptest.NewRequest(http.MethodPost, "/workgraphs/one", nil)
	recorder := httptest.NewRecorder()

	api.WriteError(recorder, request, http.StatusInternalServerError, FailureSpec{
		Cause: errors.New("sqlite secret path"),
	})

	body := recorder.Body.String()
	if strings.Contains(body, "sqlite secret path") {
		t.Fatalf("内部 Cause 泄露到响应: %s", body)
	}
	if strings.Contains(body, "transport_request_id") || strings.Contains(body, `"request_id"`) {
		t.Fatalf("缺少 middleware 时不应另造 request ID: %s", body)
	}
	if !strings.Contains(body, `"code":"common.request_failed"`) ||
		!strings.Contains(body, `"effect":"unknown"`) {
		t.Fatalf("空结构化事实没有安全回退: %s", body)
	}
}

func TestWriteErrorKeepsCancellationStatusAndCategoryConsistent(t *testing.T) {
	api := newFailureTestAPI()
	request := httptest.NewRequest(http.MethodGet, "/runs", nil)
	recorder := httptest.NewRecorder()

	api.WriteError(recorder, request, http.StatusInternalServerError, FailureSpec{
		Code:     "automation.run_history_unavailable",
		Category: protocol.FailureCategoryInternal,
		Effect:   protocol.FailureEffectNotApplicable,
		Cause:    context.Canceled,
	})

	if recorder.Code != 499 ||
		!strings.Contains(recorder.Body.String(), `"category":"canceled"`) ||
		!strings.Contains(recorder.Body.String(), `"effect":"not_applicable"`) {
		t.Fatalf("取消投影不一致: status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func newFailureTestAPI() *API {
	return NewAPI(slog.New(slog.NewTextHandler(io.Discard, nil)))
}

type webAccessAuthority struct {
	authsvc.Authority
	local    bool
	disabled bool
}

func (a webAccessAuthority) InspectRequest(context.Context, *http.Request) (*authsvc.Principal, authsvc.State, error) {
	return &authsvc.Principal{UserID: "user", WebAccessDisabled: a.disabled}, authsvc.State{AuthRequired: !a.local}, nil
}

func TestWebAccessDoesNotGrantServerResourcesOrBlockDesktopRelay(t *testing.T) {
	for _, path := range []string{"/nexus/v1/agents", "/nexus/v1/rooms", "/nexus/v1/ws", "/nexus/v1/team-node/room", "/nexus/v1/team/rooms", "/nexus/v1/team/rooms/r/events", "/nexus/v1/auth/status"} {
		for _, local := range []bool{false, true} {
			for _, disabled := range []bool{false, true} {
				handler := AuthMiddleware(NewAPI(nil), webAccessAuthority{local: local, disabled: disabled})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
				r := httptest.NewRequest(http.MethodGet, path, nil)
				r.Header.Set("User-Agent", "Nexus Desktop")
				w := httptest.NewRecorder()
				handler.ServeHTTP(w, r)
				want := http.StatusNoContent
				if disabled && !local && !strings.HasPrefix(path, "/nexus/v1/team/") && path != "/nexus/v1/auth/status" {
					want = http.StatusForbidden
				}
				if w.Code != want {
					t.Fatalf("path=%s local=%v disabled=%v: status=%d want=%d", path, local, disabled, w.Code, want)
				}
			}
		}
	}
}

// A retired connection's request context can still reach a newly bound sender
// through a session broadcast. It must not retire that healthy connection.
func TestWebSocketSenderCanceledBroadcastKeepsConnection(t *testing.T) {
	sender, peer := websocketSenderPair(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := sender.SendJSON(ctx, map[string]string{"event": "old"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled send = %v", err)
	}
	if sender.IsClosed() {
		t.Fatal("canceled broadcast retired the healthy replacement connection")
	}
	assertWebSocketSenderDelivery(t, sender, peer, map[string]string{"event": "new-round"})
}

// Once an event enters the connection writer, caller cancellation must not
// interrupt its frame and poison subsequent events on the shared connection.
func TestWebSocketSenderAdmittedWriteSurvivesCallerCancellation(t *testing.T) {
	sender, peer := websocketSenderPair(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	payload := cancelDuringJSON{cancel: cancel}
	if err := sender.SendJSON(ctx, payload); err != nil {
		t.Fatalf("admitted write = %v", err)
	}
	if !errors.Is(ctx.Err(), context.Canceled) {
		t.Fatal("fixture did not cancel the originating request")
	}
	assertWebSocketPeerEvent(t, peer, "admitted")
	assertWebSocketSenderDelivery(t, sender, peer, map[string]string{"event": "next-round"})
}

func TestWebSocketSenderTransportFailureRetiresConnection(t *testing.T) {
	sender, _ := websocketSenderPair(t)
	_ = sender.conn.CloseNow()
	if err := sender.SendJSON(context.Background(), map[string]string{"event": "closed"}); err == nil {
		t.Fatal("send on closed transport succeeded")
	}
	if !sender.IsClosed() {
		t.Fatal("failed transport remained available")
	}
}

type cancelDuringJSON struct{ cancel context.CancelFunc }

func (p cancelDuringJSON) MarshalJSON() ([]byte, error) {
	p.cancel()
	return json.Marshal(map[string]string{"event": "admitted"})
}

func websocketSenderPair(t *testing.T) (*WebSocketSender, *websocket.Conn) {
	t.Helper()
	accepted := make(chan *websocket.Conn, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err == nil {
			accepted <- conn
		}
	}))
	t.Cleanup(server.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	peer, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = peer.CloseNow() })
	select {
	case conn := <-accepted:
		t.Cleanup(func() { _ = conn.CloseNow() })
		return NewWebSocketSender(conn), peer
	case <-ctx.Done():
		t.Fatal("server did not accept the WebSocket")
		return nil, nil
	}
}

func assertWebSocketSenderDelivery(t *testing.T, sender *WebSocketSender, peer *websocket.Conn, payload map[string]string) {
	t.Helper()
	if err := sender.SendJSON(context.Background(), payload); err != nil {
		t.Fatalf("next event failed: %v", err)
	}
	assertWebSocketPeerEvent(t, peer, payload["event"])
}

func assertWebSocketPeerEvent(t *testing.T, peer *websocket.Conn, want string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var event map[string]string
	if err := wsjson.Read(ctx, peer, &event); err != nil {
		t.Fatal(err)
	}
	if event["event"] != want {
		t.Fatalf("event = %v, want %q", event, want)
	}
}
