// INPUT: 经 DM 服务受理的用户输入、chat_ack、durable 用户行广播与落盘历史。
// OUTPUT: 同一输入在 ACK、实时行与历史行上带同一个 input_seq，刷新与换新服务实例后不变。
// POS: DM 用户输入受理游标的端到端合同测试。
package dm

import (
	"context"
	"fmt"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"

	_ "modernc.org/sqlite"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

type dmInputCursorHarness struct {
	service        *Service
	runtimeManager *runtimectx.Manager
	sender         *dmTestSender
	sessionKey     string
}

// newDMInputCursorHarness 每次调用都装配全新的服务、runtime 与连接，只共用落盘状态，等价于进程重启后的宿主。
func newDMInputCursorHarness(t *testing.T, cfg config.Config, sessionKey string) *dmInputCursorHarness {
	t.Helper()
	client := newFakeDMClient()
	replies := 0
	client.onQuery = func(context.Context, string) {
		replies++
		reply := replies
		go func() {
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeAssistant,
				SessionID: client.sessionID,
				Assistant: &sdkprotocol.AssistantMessage{Message: sdkprotocol.ConversationEnvelope{
					ID:      fmt.Sprintf("assistant-%d", reply),
					Model:   "sonnet",
					Content: []sdkprotocol.ContentBlock{sdkprotocol.TextBlock{Text: "收到"}},
				}},
			}
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeResult,
				SessionID: client.sessionID,
				UUID:      fmt.Sprintf("result-%d", reply),
				Result:    &sdkprotocol.ResultMessage{Subtype: "success", NumTurns: 1, Result: "done"},
			}
		}()
	}
	permission := permissionctx.NewContext()
	runtimeManager := runtimectx.NewManagerWithFactory(&fakeDMFactory{client: client})
	service := NewService(cfg, newDMAgentService(t, cfg), runtimeManager, permission)
	service.SetUsageRecorder(&fakeTokenUsageRecorder{})
	sender := newDMTestSender("sender-input-cursor")
	permission.BindSession(sessionKey, sender)
	cleanupDMRuntimeSession(t, runtimeManager, sessionKey)
	return &dmInputCursorHarness{service: service, runtimeManager: runtimeManager, sender: sender, sessionKey: sessionKey}
}

// accept 受理一条会广播 durable 用户行的输入，返回 ACK 与实时用户行各自携带的身份和游标。
func (h *dmInputCursorHarness) accept(t *testing.T, content string) (userMessageID string, inputSeq int64) {
	t.Helper()
	if err := h.service.HandleChat(context.Background(), Request{
		SessionKey:           h.sessionKey,
		Content:              content,
		ClientMessageID:      "local-" + content,
		BroadcastUserMessage: true,
	}); err != nil {
		t.Fatalf("HandleChat 失败: %v", err)
	}
	events := collectEventsUntil(t, h.sender.events, func(event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus && event.Data["status"] == "finished"
	})
	waitForDMRuntimeIdle(t, h.runtimeManager, h.sessionKey)

	var ackSeq, liveSeq int64
	var liveMessageID string
	for _, event := range events {
		switch {
		case event.EventType == protocol.EventTypeChatAck:
			userMessageID = anyToString(event.Data["user_message_id"])
			ackSeq = protocol.Int64FromAny(event.Data[protocol.InputSeqField])
		case event.EventType == protocol.EventTypeMessage && event.Data["role"] == "user":
			liveMessageID = anyToString(event.Data["message_id"])
			liveSeq = protocol.Int64FromAny(event.Data[protocol.InputSeqField])
		}
	}
	if ackSeq <= 0 || liveSeq != ackSeq || liveMessageID != userMessageID {
		t.Fatalf("ACK 与实时用户行必须给出同一身份与游标: ack=(%s,%d) live=(%s,%d)", userMessageID, ackSeq, liveMessageID, liveSeq)
	}
	return userMessageID, ackSeq
}

// historyInputSeqs 用全新的历史 store 读取落盘历史，返回每条用户行的游标。
func (h *dmInputCursorHarness) historyInputSeqs(t *testing.T, cfg config.Config) map[string]int64 {
	t.Helper()
	cursors := map[string]int64{}
	for _, row := range readDMSessionHistory(t, cfg, h.service, h.sessionKey) {
		if row["role"] == "user" {
			cursors[anyToString(row["message_id"])] = protocol.Int64FromAny(row[protocol.InputSeqField])
		}
	}
	return cursors
}

func TestDMUserRowSharesInputSeqAcrossAckLiveRowAndHistory(t *testing.T) {
	cfg := newDMTestConfig(t)
	migrateDMSQLite(t, cfg.DatabaseURL)
	sessionKey := "agent:nexus:ws:dm:input-cursor"
	live := map[string]int64{}
	assertHistory := func(stage string, harness *dmInputCursorHarness) {
		t.Helper()
		got := harness.historyInputSeqs(t, cfg)
		if len(got) != len(live) {
			t.Fatalf("%s: 历史用户行 = %v，实时受理 = %v", stage, got, live)
		}
		for messageID, inputSeq := range live {
			if got[messageID] != inputSeq {
				t.Fatalf("%s: %s 的历史游标 = %d，受理时是 %d", stage, messageID, got[messageID], inputSeq)
			}
		}
	}

	first := newDMInputCursorHarness(t, cfg, sessionKey)
	firstID, firstSeq := first.accept(t, "第一条")
	live[firstID] = firstSeq
	assertHistory("受理后读取", first)

	secondID, secondSeq := first.accept(t, "第二条")
	if secondSeq <= firstSeq {
		t.Fatalf("后受理的输入游标必须更大: first=%d second=%d", firstSeq, secondSeq)
	}
	live[secondID] = secondSeq
	assertHistory("追加一轮后刷新", first)

	restarted := newDMInputCursorHarness(t, cfg, sessionKey)
	assertHistory("换新服务实例后读取", restarted)
	thirdID, thirdSeq := restarted.accept(t, "第三条")
	if thirdSeq <= secondSeq {
		t.Fatalf("重启后受理的输入游标必须继续增大: second=%d third=%d", secondSeq, thirdSeq)
	}
	live[thirdID] = thirdSeq
	assertHistory("重启后再受理一轮", restarted)
}
