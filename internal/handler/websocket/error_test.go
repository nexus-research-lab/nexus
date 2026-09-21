package websocket

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"

	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	"github.com/nexus-research-lab/nexus/internal/infra/logx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestGatewayErrorLogsCauseAndDeliveryFailureWithoutPayload(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, nil)).With("request_id", "connection-1")
	ctx, cancel := context.WithCancel(logx.WithLogger(context.Background(), logger))
	cancel()
	h := &Handler{}
	sender := handlershared.NewWebSocketSender(nil)
	sender.MarkClosed()
	cause := errors.New("room input_queue content must mention target agent")
	h.sendGatewayError(ctx, sender, "room:group:conversation-1", "input_queue_error", cause, map[string]any{
		"type": "input_queue", "action": "enqueue",
		"client_request_id": "request-1", "client_message_id": "message-1",
		"content": "private message body", "auth_token": "private credential",
	})
	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected cause and delivery logs, got %s", output.String())
	}
	for index, line := range lines {
		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatal(err)
		}
		for key, want := range map[string]any{
			"level": "WARN", "request_id": "connection-1", "session_key": "room:group:conversation-1",
			"error_type": "input_queue_error", "failure_code": string(protocol.ConversationFailureRequestRejected),
			"type": "input_queue", "action": "enqueue", "client_request_id": "request-1",
			"client_message_id": "message-1", "err": cause.Error(), "context_err": context.Canceled.Error(),
		} {
			if entry[key] != want {
				t.Fatalf("log %d %s = %v, want %v", index, key, entry[key], want)
			}
		}
		if strings.Contains(line, "private") || entry["content"] != nil || entry["auth_token"] != nil {
			t.Fatalf("business input leaked into diagnostics: %s", line)
		}
		if index == 1 && entry["send_err"] != context.Canceled.Error() {
			t.Fatalf("delivery error lost: %s", line)
		}
	}
	if got := h.errorEventDetail("input_queue_error", cause); got != "服务内部错误" {
		t.Fatalf("public error exposed internal cause: %q", got)
	}
}

func TestGatewayFailureCodeTokenLimit(t *testing.T) {
	got := gatewayFailureCode("chat_error", errors.New("context_length_exceeded"))
	if got != protocol.ConversationFailureUsageLimited {
		t.Fatalf("gatewayFailureCode() = %q, want %q", got, protocol.ConversationFailureUsageLimited)
	}
}

func TestChatErrorDetailTokenLimit(t *testing.T) {
	got := chatErrorDetail(errors.New("provider rejected request: too many tokens"))
	want := "模型的 Token 或上下文额度已达到上限。请缩短提示内容、清理会话上下文，或切换模型后重试。"
	if got != want {
		t.Fatalf("chatErrorDetail() = %q, want %q", got, want)
	}
}
