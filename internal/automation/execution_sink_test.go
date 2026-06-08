package automation

import (
	"context"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestExecutionSinkMarksPermissionDenialSummaryAsFailed(t *testing.T) {
	sink := NewExecutionSink("automation:test")
	const roundID = "round-permission-denied"

	if err := sink.SendEvent(context.Background(), protocol.EventMessage{
		EventType:  protocol.EventTypeMessage,
		SessionKey: "agent:agent-1:ws:dm:automation:run-1",
		Data: map[string]any{
			"round_id": roundID,
			"role":     "assistant",
			"content": []map[string]any{{
				"type": "text",
				"text": "无法完成搜索",
			}},
			"result_summary": map[string]any{
				"subtype": "success",
				"result":  "无法完成搜索：WebSearch 未被允许",
				"permission_denials": []map[string]any{{
					"tool_name": "WebSearch",
				}},
			},
		},
	}); err != nil {
		t.Fatalf("SendEvent 失败: %v", err)
	}
	if err := sink.SendEvent(context.Background(), protocol.NewRoundStatusEvent(
		"agent:agent-1:ws:dm:automation:run-1",
		roundID,
		"finished",
		"success",
	)); err != nil {
		t.Fatalf("SendEvent round status 失败: %v", err)
	}

	observation := sink.WaitForRound(context.Background(), roundID)
	if observation.Status != protocol.RunStatusFailed {
		t.Fatalf("权限拒绝的后台运行应标记为 failed，实际: %+v", observation)
	}
	if observation.ErrorMessage == nil || !strings.Contains(*observation.ErrorMessage, "WebSearch") {
		t.Fatalf("权限拒绝错误应包含工具名: %+v", observation.ErrorMessage)
	}
	if observation.ResultText != "无法完成搜索：WebSearch 未被允许" {
		t.Fatalf("result text 未保留: %+v", observation.ResultText)
	}
}
