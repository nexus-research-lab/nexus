// round_runner_test.go 覆盖 DM 轮次执行器、上下文注入、连接器通告、子代理任务与唤醒测试。
package dm

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	conversationsvc "github.com/nexus-research-lab/nexus/internal/service/conversation"
	"github.com/nexus-research-lab/nexus/internal/service/runtimehost"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
	_ "modernc.org/sqlite"
)

func TestRoundRunnerMaintainsExternalTypingState(t *testing.T) {
	t.Parallel()

	dispatcher := &fakeExternalReplyDispatcher{}
	runner := &roundRunner{
		service:    &Service{replies: dispatcher, Host: runtimehost.Host{Runtime: runtimectx.NewManager()}},
		agent:      &protocol.Agent{AgentID: "agent-1"},
		sessionKey: "agent:agent-1:weixin-personal:dm:user-1",
		roundID:    "round-1",
		externalReplyTarget: &ExternalReplyTarget{
			Mode:     "explicit",
			Channel:  "weixin-personal",
			To:       "user-1",
			ThreadID: "context-token-1",
		},
	}
	accelerateDMExternalTyping(runner)

	stop := runner.startExternalReplyTyping(context.Background())
	deadline := time.After(2 * time.Second)
	for {
		if calls := dispatcher.typingCallsSnapshot(); len(calls) > 0 && calls[0].active {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("未发送 typing start: %+v", dispatcher.typingCallsSnapshot())
		case <-time.After(10 * time.Millisecond):
		}
	}
	stop()

	calls := dispatcher.typingCallsSnapshot()
	if len(calls) < 2 {
		t.Fatalf("typing start/stop 调用不足: %+v", calls)
	}
	if !calls[0].active || calls[len(calls)-1].active {
		t.Fatalf("typing 状态顺序不正确: %+v", calls)
	}
	if calls[0].target.ThreadID != "context-token-1" || calls[len(calls)-1].target.To != "user-1" {
		t.Fatalf("typing 目标不正确: %+v", calls)
	}
}

func TestBoundLocalDMForwardsFinalReplyToOriginalIM(t *testing.T) {
	dispatcher := &fakeExternalReplyDispatcher{}
	runner := &roundRunner{service: &Service{replies: dispatcher, Host: runtimehost.Host{Runtime: runtimectx.NewManager()}}, agent: &protocol.Agent{AgentID: "amy"}, sessionKey: "agent:amy:ws:dm:existing", externalReplyTarget: &ExternalReplyTarget{PairingID: "pair", BindingVersion: 2, Channel: "weixin-personal", To: "person", SessionKey: "agent:amy:weixin-personal:dm:person"}}
	runner.deliverExternalAssistantReply(t.Context(), protocol.Message{"role": "assistant", "content": []map[string]any{{"type": "text", "text": "原会话的回答"}}})
	calls := dispatcher.callsSnapshot()
	if len(calls) != 1 || calls[0].target.PairingID != "pair" || calls[0].target.BindingVersion != 2 {
		t.Fatalf("reply=%+v", calls)
	}
	runner.externalReplyTarget.PairingID = ""
	runner.deliverExternalAssistantReply(t.Context(), protocol.Message{"role": "assistant", "content": []map[string]any{{"type": "text", "text": "未绑定"}}})
	if len(dispatcher.callsSnapshot()) != 1 {
		t.Fatal("unbound local session forwarded")
	}
}

// 恢复上下文改读历史尾页后，结果必须与读取完整历史时一致。
func TestRecoveryContextualInputsMatchFullHistory(t *testing.T) {
	cases := map[string]func(t *testing.T, h *workspacestore.AgentHistoryStore, ws, key string, ts *int64){
		"failed previous round": func(t *testing.T, h *workspacestore.AgentHistoryStore, ws, key string, ts *int64) {
			for i := range 12 {
				appendRecoveryRound(t, h, ws, key, fmt.Sprintf("round-%d", i), "success", ts)
			}
			appendRecoveryRound(t, h, ws, key, "round-failed", "error", ts)
		},
		"automation deliveries after last user": func(t *testing.T, h *workspacestore.AgentHistoryStore, ws, key string, ts *int64) {
			appendRecoveryRound(t, h, ws, key, "round-user", "success", ts)
			for i := range 7 {
				appendAutomationDeliveryRound(t, h, ws, key, i, ts)
			}
		},
	}
	for name, seed := range cases {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			workspacePath := filepath.Join(root, "agent-1")
			history := workspacestore.NewAgentHistoryStore(root)
			sessionKey := "agent:agent-1:web:dm:recovery"
			session := protocol.Session{SessionKey: sessionKey, AgentID: "agent-1"}
			timestamp := int64(1000)
			seed(t, history, workspacePath, sessionKey, &timestamp)
			if err := history.AppendRoundMarker(workspacePath, sessionKey, "round-current", "now", timestamp+10); err != nil {
				t.Fatal(err)
			}

			full, err := history.ReadMessages(workspacePath, session, nil)
			if err != nil {
				t.Fatal(err)
			}
			want := append(
				conversationsvc.AutomationDeliveryContextualInputs(full, "round-current"),
				conversationsvc.RoundRecoveryContextualInputs(full, "", "round-current")...,
			)
			if len(want) == 0 {
				t.Fatal("fixture 应产生恢复上下文")
			}
			execution := &dmChatExecution{
				service: &Service{Host: runtimehost.Host{History: history}},
				ctx:     context.Background(),
				request: Request{RoundID: "round-current"},
				agent:   &protocol.Agent{AgentID: "agent-1", WorkspacePath: workspacePath},
				session: session,
			}
			if got := execution.recoveryContextualInputs(); !reflect.DeepEqual(got, want) {
				t.Fatalf("恢复上下文与完整历史不一致: got=%d %v want=%d %v", len(got), blockNames(got), len(want), blockNames(want))
			}
		})
	}
}

func appendRecoveryRound(t *testing.T, h *workspacestore.AgentHistoryStore, ws, key, roundID, subtype string, ts *int64) {
	t.Helper()
	*ts += 10
	if err := h.AppendRoundMarker(ws, key, roundID, "question "+roundID, *ts); err != nil {
		t.Fatal(err)
	}
	appendRecoveryOverlay(t, h, ws, key, protocol.Message{
		"message_id": "assistant-" + roundID, "session_key": key, "agent_id": "agent-1", "round_id": roundID,
		"role": "assistant", "timestamp": *ts + 1, "is_complete": subtype == "success",
		"content": []map[string]any{{"type": "text", "text": "answer"}},
	})
	result := protocol.NewHostResultMessage("result-"+roundID, key, "agent-1", roundID, subtype, "done", subtype != "success")
	result["timestamp"] = *ts + 2
	appendRecoveryOverlay(t, h, ws, key, result)
}

func appendAutomationDeliveryRound(t *testing.T, h *workspacestore.AgentHistoryStore, ws, key string, index int, ts *int64) {
	t.Helper()
	*ts += 10
	roundID := fmt.Sprintf("automation_delivery_round_%d", index)
	if err := h.AppendRoundMarkerWithOptions(ws, key, roundID, "deliver", *ts, workspacestore.RoundMarkerOptions{
		HiddenFromUser: true, Synthetic: true, Purpose: "automation_delivery",
	}); err != nil {
		t.Fatal(err)
	}
	metadata := map[string]any{"source": "automation_delivery", "job_id": "job", "run_id": fmt.Sprintf("run-%d", index)}
	appendRecoveryOverlay(t, h, ws, key, protocol.Message{
		"message_id": fmt.Sprintf("automation_delivery_assistant_%d", index), "session_key": key, "agent_id": "agent-1",
		"round_id": roundID, "role": "assistant", "timestamp": *ts + 1, "stop_reason": "end_turn", "is_complete": true,
		"content":  []map[string]any{{"type": "text", "text": fmt.Sprintf("report %d", index)}},
		"metadata": metadata,
	})
	result := protocol.NewHostResultMessage(fmt.Sprintf("automation_delivery_result_%d", index), key, "agent-1", roundID, "success", "report", false)
	result["timestamp"] = *ts + 2
	result["metadata"] = metadata
	appendRecoveryOverlay(t, h, ws, key, result)
}

func appendRecoveryOverlay(t *testing.T, h *workspacestore.AgentHistoryStore, ws, key string, message protocol.Message) {
	t.Helper()
	if err := h.AppendOverlayMessage(ws, key, message); err != nil {
		t.Fatal(err)
	}
}

func blockNames(blocks []runtimectx.ContextualInputBlock) []string {
	names := make([]string, 0, len(blocks))
	for _, block := range blocks {
		names = append(names, block.Name+":"+fmt.Sprint(block.Metadata))
	}
	return names
}

func dmSubagentTaskMessage(subtype string, status string) protocol.Message {
	return protocol.Message{
		"metadata": map[string]any{
			"subtype":    subtype,
			"task_id":    "task-1",
			"agent_id":   "subagent-1",
			"agent_type": "worker",
			"status":     status,
		},
	}
}

// 交接期间普通输入让路并在交接结束后放行；自唤醒 round 自身不受闸门影响。
func TestRuntimeWakeGateYieldsOrdinaryInputUntilHandoffEnds(t *testing.T) {
	service := &Service{}
	release := service.RuntimeWakeGates.Begin("agent:a:ws:dm:c")
	ordinary := &dmChatExecution{service: service, sessionKey: "agent:a:ws:dm:c"}
	err := ordinary.yieldToRuntimeWake()
	var pending *runtimeWakePendingError
	if !errors.As(err, &pending) || !errors.Is(err, runtimehost.ErrRuntimeWakeHandoffPending) {
		t.Fatalf("ordinary yield err = %v", err)
	}
	wake := &dmChatExecution{service: service, sessionKey: "agent:a:ws:dm:c", request: Request{runtimeWake: &runtimeWakeStart{}}}
	if err := wake.yieldToRuntimeWake(); err != nil {
		t.Fatalf("wake round must pass its own gate: %v", err)
	}
	go func() {
		time.Sleep(10 * time.Millisecond)
		release()
		release()
	}()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := pending.wait(ctx); err != nil {
		t.Fatalf("wait err = %v", err)
	}
	if err := ordinary.yieldToRuntimeWake(); err != nil {
		t.Fatalf("after release err = %v", err)
	}
}
