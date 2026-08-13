package conversation

import (
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestAutomationDeliveryContextualInputsBridgesTrailingScheduledResult(t *testing.T) {
	history := []protocol.Message{
		{"message_id": "user-1", "round_id": "round-1", "role": "user", "content": "设置一个任务"},
		{
			"message_id": "automation_delivery_assistant_run-1",
			"round_id":   "automation_delivery_round_run-1",
			"role":       "assistant",
			"content":    []map[string]any{{"type": "text", "text": "定时结果 123"}},
			"metadata": map[string]any{
				"source":    "automation_delivery",
				"job_id":    "task-1",
				"run_id":    "run-1",
				"task_name": "测试任务",
			},
		},
		{"message_id": "user-2", "round_id": "round-current", "role": "user", "content": "继续"},
	}

	blocks := AutomationDeliveryContextualInputs(history, "round-current")
	if len(blocks) != 1 || blocks[0].Name != "automation_delivery" {
		t.Fatalf("scheduled delivery context = %+v", blocks)
	}
	if !strings.Contains(blocks[0].Content, "定时结果 123") ||
		blocks[0].Metadata["job_id"] != "task-1" ||
		blocks[0].Metadata["run_id"] != "run-1" {
		t.Fatalf("scheduled delivery context 内容不完整: %+v", blocks[0])
	}
}

func TestAutomationDeliveryContextualInputsStopsAtLaterRealUser(t *testing.T) {
	history := []protocol.Message{
		{
			"message_id": "automation_delivery_assistant_run-1",
			"round_id":   "automation_delivery_round_run-1",
			"role":       "assistant",
			"content":    []map[string]any{{"type": "text", "text": "旧定时结果"}},
			"metadata":   map[string]any{"source": "automation_delivery", "job_id": "task-1", "run_id": "run-1"},
		},
		{"message_id": "user-2", "round_id": "round-2", "role": "user", "content": "已经讨论过了"},
		{"message_id": "assistant-2", "round_id": "round-2", "role": "assistant", "content": []map[string]any{{"type": "text", "text": "好的"}}},
		{"message_id": "user-3", "round_id": "round-current", "role": "user", "content": "新问题"},
	}

	if blocks := AutomationDeliveryContextualInputs(history, "round-current"); len(blocks) != 0 {
		t.Fatalf("已有后续真实用户轮时不应重复注入旧结果: %+v", blocks)
	}
}
