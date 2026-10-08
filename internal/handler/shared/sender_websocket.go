// INPUT: authenticated WebSocket connection、调用方上下文与待发送事件。
// OUTPUT: 串行且有界的完整帧发送；调用方取消不退休其他请求仍使用的连接。
// POS: handler 共享 transport 写边界；只由真实传输失败或连接关闭标记失效。
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

// ClosePolicy 以策略变更原因主动关闭连接。
func (s *WebSocketSender) ClosePolicy(reason string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed.Swap(true) {
		return nil
	}
	return s.conn.Close(websocket.StatusPolicyViolation, reason)
}

// SendEvent 发送协议事件。
func (s *WebSocketSender) SendEvent(ctx context.Context, event protocol.EventMessage) error {
	return s.SendJSON(ctx, event)
}

// SendJSON 发送原始 JSON payload，供 Codex app-server 兼容协议复用同一连接。
func (s *WebSocketSender) SendJSON(ctx context.Context, payload any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed.Load() {
		return context.Canceled
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	// An old request may broadcast to a newly rebound connection. Reject work
	// already canceled before admission, but let an admitted frame complete
	// under the connection's own deadline: aborting it would also poison every
	// later event on this shared transport.
	writeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), WebSocketWriteTimeout)
	defer cancel()
	if err := wsjson.Write(writeCtx, s.conn, payload); err != nil {
		s.MarkClosed()
		return err
	}
	return nil
}
