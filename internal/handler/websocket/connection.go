package websocket

import (
	"context"
	"time"

	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"

	"github.com/coder/websocket"
)

func (h *Handler) keepWebSocketAlive(
	ctx context.Context,
	connection *websocket.Conn,
	sender *handlershared.WebSocketSender,
) {
	ticker := time.NewTicker(websocketPingEvery)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if sender.IsClosed() {
				return
			}
			pingCtx, cancel := context.WithTimeout(ctx, handlershared.WebSocketWriteTimeout)
			err := connection.Ping(pingCtx)
			cancel()
			if err != nil {
				sender.MarkClosed()
				_ = connection.Close(websocket.StatusPolicyViolation, "ping timeout")
				return
			}
		}
	}
}
