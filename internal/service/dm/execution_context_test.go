package dm

import (
	"context"
	"fmt"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	conversationsvc "github.com/nexus-research-lab/nexus/internal/service/conversation"
	"github.com/nexus-research-lab/nexus/internal/service/runtimehost"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

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
