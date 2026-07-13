package dm

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

func TestServiceHandleRewriteRemovesRuntimeTailBeforeQuery(t *testing.T) {
	cfg := newDMTestConfig(t)
	migrateDMSQLite(t, cfg.DatabaseURL)

	agentService := newDMAgentService(t, cfg)
	permission := permissionctx.NewContext()
	client := newFakeDMClient()
	client.onQuery = func(_ context.Context, _ string) {
		go func() {
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeAssistant,
				SessionID: client.sessionID,
				Assistant: &sdkprotocol.AssistantMessage{
					Message: sdkprotocol.ConversationEnvelope{
						ID: "assistant-rewrite",
						Content: []sdkprotocol.ContentBlock{
							sdkprotocol.TextBlock{Text: "回答"},
						},
					},
				},
			}
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeResult,
				SessionID: client.sessionID,
				UUID:      "result-rewrite",
				Result: &sdkprotocol.ResultMessage{
					Subtype:    "success",
					DurationMS: 1,
					Result:     "done",
				},
			}
		}()
	}

	factory := &fakeDMFactory{client: client}
	service := NewService(cfg, agentService, runtimectx.NewManagerWithFactory(factory), permission)
	sender := newDMTestSender("sender-rewrite")
	sessionKey := "agent:nexus:ws:dm:rewrite"
	permission.BindSession(sessionKey, sender)

	if err := service.HandleChat(context.Background(), Request{
		SessionKey: sessionKey,
		Content:    "旧问题",
		RoundID:    "round-old",
	}); err != nil {
		t.Fatalf("HandleChat 失败: %v", err)
	}
	collectEventsUntil(t, sender.events, func(event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus && event.Data["status"] == "finished"
	})

	sessionValue, workspacePath := mustFindDMSession(t, service, cfg, sessionKey)
	sessionID := stringPointer(t, sessionValue.SessionID)
	baseTime := time.Now().Add(-time.Second).UTC()
	writeTranscriptFixture(t, workspacePath, sessionID, []map[string]any{
		{
			"type":      "user",
			"uuid":      "user-old",
			"sessionId": sessionID,
			"timestamp": baseTime.Format(time.RFC3339Nano),
			"message": map[string]any{
				"role":    "user",
				"content": "旧问题",
			},
		},
		{
			"type":       "assistant",
			"uuid":       "assistant-old",
			"sessionId":  sessionID,
			"parentUuid": "user-old",
			"timestamp":  baseTime.Add(100 * time.Millisecond).Format(time.RFC3339Nano),
			"message": map[string]any{
				"id":      "assistant-old",
				"type":    "message",
				"role":    "assistant",
				"content": []map[string]any{{"type": "text", "text": "旧回答"}},
			},
		},
	})

	if err := service.HandleRewriteLastUserMessage(context.Background(), RewriteRequest{
		SessionKey:      sessionKey,
		TargetRoundID:   "round-old",
		ClientRequestID: "rewrite-request-1",
		ClientMessageID: "rewrite-message-1",
		Content:         "新问题",
	}); err != nil {
		t.Fatalf("HandleRewriteLastUserMessage 失败: %v", err)
	}
	collectEventsUntil(t, sender.events, func(event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus && event.Data["status"] == "finished"
	})

	client.mu.Lock()
	removeMessages := append([][]string(nil), client.removeMessages...)
	queryPrompts := append([]string(nil), client.queryPrompts...)
	disconnectCalls := client.disconnectCalls
	client.mu.Unlock()

	if len(removeMessages) != 1 {
		t.Fatalf("期望删除一次 runtime 历史，实际 %#v", removeMessages)
	}
	if got, want := strings.Join(removeMessages[0], ","), "user-old,assistant-old"; got != want {
		t.Fatalf("删除 UUID = %q, want %q", got, want)
	}
	if disconnectCalls != 0 {
		t.Fatalf("rewrite 不应重建 runtime session，disconnectCalls=%d", disconnectCalls)
	}
	overlayPath := workspacestore.New(cfg.WorkspacePath).SessionOverlayPath(workspacePath, sessionKey)
	overlayPayload, err := os.ReadFile(overlayPath)
	if err != nil {
		t.Fatalf("读取 overlay 失败: %v", err)
	}
	if strings.Contains(string(overlayPayload), "round-old") {
		t.Fatalf("rewrite 后 overlay 不应保留旧 round:\n%s", string(overlayPayload))
	}
	if len(queryPrompts) < 2 {
		t.Fatalf("期望 rewrite 触发第二次 query，实际 %#v", queryPrompts)
	}
	rewritePrompt := queryPrompts[len(queryPrompts)-1]
	if strings.Contains(rewritePrompt, "<nexus_history_context>") || strings.Contains(rewritePrompt, "旧回答") {
		t.Fatalf("rewrite query 不应注入 synthetic history:\n%s", rewritePrompt)
	}
	if !strings.Contains(rewritePrompt, "新问题") {
		t.Fatalf("rewrite query 应包含新问题:\n%s", rewritePrompt)
	}
}
