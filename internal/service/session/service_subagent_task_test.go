package session

import (
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestBuildSubagentTasksMergesStartedAndNotification(t *testing.T) {
	messages := []protocol.Message{
		{
			"content":   "子 Agent 开始排查",
			"round_id":  "round-1",
			"timestamp": int64(1000),
			"metadata": map[string]any{
				"subtype":        "task_started",
				"task_id":        "task-1",
				"tool_use_id":    "toolu-1",
				"agent_id":       "agent-1",
				"agent_type":     "worker",
				"output_file":    "/tmp/task.out",
				"parent_task_id": "parent-1",
			},
		},
		{
			"content":   "子 Agent 已完成排查",
			"round_id":  "round-1",
			"timestamp": int64(2000),
			"metadata": map[string]any{
				"subtype":         "task_notification",
				"task_id":         "task-1",
				"status":          "completed",
				"transcript_path": "/tmp/subagent.jsonl",
				"usage": map[string]any{
					"total_tokens": 123,
					"tool_uses":    4,
					"duration_ms":  567,
				},
			},
		},
	}

	tasks := buildSubagentTasks("agent:nexus:ws:dm:test", messages)
	if len(tasks) != 1 {
		t.Fatalf("len(tasks) = %d, want 1", len(tasks))
	}
	task := tasks[0]
	if task.TaskID != "task-1" || task.Status != "completed" {
		t.Fatalf("task identity = %+v, want completed task-1", task)
	}
	if task.AgentID != "agent-1" || task.AgentType != "worker" {
		t.Fatalf("task agent fields = %+v, want agent-1/worker", task)
	}
	if task.OutputFile != "/tmp/task.out" || task.TranscriptPath != "/tmp/subagent.jsonl" {
		t.Fatalf("task files = %+v, want output/transcript paths", task)
	}
	if task.StartedAt != 1000 || task.UpdatedAt != 2000 {
		t.Fatalf("task timestamps = %+v, want started/updated", task)
	}
	if task.Usage["total_tokens"] != 123 || task.Usage["tool_uses"] != 4 {
		t.Fatalf("task usage = %+v, want tokens/tool uses", task.Usage)
	}
}

func TestBuildSubagentTasksMergesTaskUpdatedTerminal(t *testing.T) {
	messages := []protocol.Message{
		{
			"content":   "子 Agent 开始排查",
			"round_id":  "round-1",
			"timestamp": int64(1000),
			"metadata": map[string]any{
				"subtype":    "task_started",
				"task_id":    "task-1",
				"agent_id":   "agent-1",
				"agent_type": "worker",
			},
		},
		{
			"content":   "后台子 Agent 已停止",
			"round_id":  "round-1",
			"timestamp": int64(2000),
			"metadata": map[string]any{
				"subtype": "task_updated",
				"task_id": "task-1",
				"status":  "killed",
				"patch": map[string]any{
					"status": "killed",
				},
			},
		},
	}

	tasks := buildSubagentTasks("agent:nexus:ws:dm:test", messages)
	if len(tasks) != 1 {
		t.Fatalf("len(tasks) = %d, want 1", len(tasks))
	}
	if tasks[0].Status != "killed" || tasks[0].UpdatedAt != 2000 {
		t.Fatalf("task = %+v, want killed update", tasks[0])
	}
}

func TestBuildSubagentTasksIncludesAssistantTaskProgress(t *testing.T) {
	messages := []protocol.Message{
		{
			"content": []any{
				map[string]any{
					"type":           "task_progress",
					"task_id":        "task-1",
					"description":    "统计HTML小游戏数量",
					"tool_use_id":    "toolu-1",
					"last_tool_name": "Read",
					"usage": map[string]any{
						"total_tokens": 321,
					},
				},
			},
			"round_id":  "round-1",
			"role":      "assistant",
			"timestamp": int64(1000),
		},
	}

	tasks := buildSubagentTasks("agent:nexus:ws:dm:test", messages)
	if len(tasks) != 1 {
		t.Fatalf("len(tasks) = %d, want 1", len(tasks))
	}
	task := tasks[0]
	if task.TaskID != "task-1" || task.Status != "running" {
		t.Fatalf("task identity = %+v, want running task-1", task)
	}
	if task.Description != "统计HTML小游戏数量" || task.ToolUseID != "toolu-1" {
		t.Fatalf("task progress fields = %+v, want description/tool use id", task)
	}
	if task.Usage["total_tokens"] != 321 || task.UpdatedAt != 1000 {
		t.Fatalf("task metrics = %+v/%d, want usage and updated time", task.Usage, task.UpdatedAt)
	}
}
