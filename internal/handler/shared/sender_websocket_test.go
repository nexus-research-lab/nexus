package shared

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

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
