package shared

import (
	"context"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

var webSocketSenderSeq atomic.Uint64

// WebSocketWriteTimeout 是写出 WebSocket 事件的超时窗口。
const WebSocketWriteTimeout = 10 * time.Second

// WebSocketSender 封装单个 WebSocket 连接的并发安全发送能力。
type WebSocketSender struct {
	key    string
	conn   *websocket.Conn
	mu     sync.Mutex
	closed atomic.Bool
}

// NewWebSocketSender 创建发送器。
func NewWebSocketSender(conn *websocket.Conn) *WebSocketSender {
	return &WebSocketSender{
		key:  strconv.FormatUint(webSocketSenderSeq.Add(1), 10),
		conn: conn,
	}
}

// Key 返回发送器唯一键。
func (s *WebSocketSender) Key() string {
	return s.key
}

// IsClosed 返回连接是否已关闭。
func (s *WebSocketSender) IsClosed() bool {
	return s.closed.Load()
}

// MarkClosed 标记发送器已关闭。
func (s *WebSocketSender) MarkClosed() {
	s.closed.Store(true)
}

// SendEvent 发送协议事件。
func (s *WebSocketSender) SendEvent(ctx context.Context, event protocol.EventMessage) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed.Load() {
		return context.Canceled
	}
	writeCtx, cancel := context.WithTimeout(ctx, WebSocketWriteTimeout)
	defer cancel()
	return wsjson.Write(writeCtx, s.conn, event)
}
