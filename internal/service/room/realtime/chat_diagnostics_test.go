package realtime

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	"github.com/nexus-research-lab/nexus/internal/infra/logx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
)

func TestRoomRuntimeDiagnosticsLogsStderrWhenEnabled(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		var output bytes.Buffer
		logger := logx.New(logx.Options{Output: &output, Format: "json"})
		value := "0"
		if enabled {
			value = "1"
		}
		options := withRoomRuntimeDiagnosticsLogger(agentclient.Options{
			Env: map[string]string{runtimectx.AgentSDKDiagnosticsEnvName: value},
		}, logger)
		options.Callbacks.Stderr("startup error")
		if got := strings.Contains(output.String(), "startup error"); got != enabled {
			t.Fatalf("enabled=%t, stderr logged=%t", enabled, got)
		}
	}
}

func TestCancelledRoomChatStopsBeforePreparationAndLogsIdentity(t *testing.T) {
	var output bytes.Buffer
	logger := logx.New(logx.Options{Output: &output, Format: "json"}).With("request_id", "req-socket")
	ctx, cancel := context.WithCancel(logx.WithLogger(context.Background(), logger))
	cancel()
	// 不装配上下文或存储依赖，取消请求若继续准备就会失败。
	service := &Service{rounds: newRoomRoundRegistry()}
	err := service.HandleChat(ctx, ChatRequest{
		SessionKey:      protocol.BuildRoomSharedSessionKey("conversation-a"),
		Content:         "private content must not appear in stage diagnostics",
		ClientRequestID: "req-chat", ClientMessageID: "msg-chat",
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("HandleChat error = %v", err)
	}
	for _, want := range []string{"req-socket", "req-chat", "msg-chat", "conversation-a", "dispatch_lock", "context canceled"} {
		if !strings.Contains(output.String(), want) {
			t.Fatalf("诊断缺少 %q: %s", want, output.String())
		}
	}
	if strings.Contains(output.String(), "private content") {
		t.Fatal("阶段诊断不应记录用户正文")
	}
}
