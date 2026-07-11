package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

func buildSubagentTasks(sessionKey string, messages []protocol.Message) []SubagentTask {
	return buildSubagentTasksWithRuntime(sessionKey, messages, string(agentclient.RuntimeNXS))
}

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

func TestInferSubagentTaskProgressStatusEdgeCases(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"incomplete", ""},
		{"task is incomplete", ""},
		{"unfinished", ""},
		{"not completed", ""},
		{"not complete", ""},
		{"not done", ""},
		{"not finished", ""},
		{"not yet done", ""},
		{"未完成", ""},
		{"没完成", ""},
		{"failed to complete", "failed"},
		{"could not finish", "failed"},
		{"completed successfully", "completed"},
		{"complete", "completed"},
		{"done.", "completed"},
		{"已完成", "completed"},
		{"完成", "completed"},
		{"failed with error", "failed"},
		{"error occurred", "failed"},
		{"running", "running"},
		{"in_progress", "running"},
		{"in progress", "running"},
		{"正在处理", "running"},
		{"", ""},
		{"reading files", ""},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := inferSubagentTaskProgressStatus(tt.input)
			if got != tt.want {
				t.Errorf("inferSubagentTaskProgressStatus(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
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

func TestBuildSubagentTasksMergesFlatProgressAndKeepsLatestSnapshot(t *testing.T) {
	messages := []protocol.Message{
		{
			"agent_id":  "host-agent",
			"round_id":  "round-1",
			"timestamp": int64(1000),
			"metadata": map[string]any{
				"subtype":          "task_started",
				"task_id":          "task-1",
				"agent_id":         "subagent-1",
				"agent_type":       "worker",
				"child_session_id": "child-session-1",
				"task_type":        "local_agent",
				"runtime_kind":     "claude",
			},
		},
		{
			"agent_id":  "host-agent",
			"round_id":  "round-1",
			"timestamp": int64(2000),
			"metadata": map[string]any{
				"subtype":        "task_progress",
				"task_id":        "task-1",
				"summary":        "第一次进度",
				"last_tool_name": "Read",
				"usage":          map[string]any{"total_tokens": 10},
			},
		},
		{
			"agent_id":  "host-agent",
			"round_id":  "round-1",
			"timestamp": int64(3000),
			"metadata": map[string]any{
				"subtype":        "task_progress",
				"task_id":        "task-1",
				"summary":        "第二次进度",
				"last_tool_name": "Bash",
				"usage":          map[string]any{"total_tokens": 20},
			},
		},
	}

	tasks := buildSubagentTasks("room:group:conversation-1", messages)
	if len(tasks) != 1 {
		t.Fatalf("len(tasks) = %d, want 1", len(tasks))
	}
	task := tasks[0]
	if task.AgentID != "subagent-1" || task.HostAgentID != "host-agent" {
		t.Fatalf("task agent identity = %+v", task)
	}
	if task.ChildSessionID != "child-session-1" || task.TaskType != "local_agent" {
		t.Fatalf("task thread identity = %+v", task)
	}
	if task.Summary != "第二次进度" || task.LastToolName != "Bash" || task.UpdatedAt != 3000 {
		t.Fatalf("task latest progress = %+v", task)
	}
	if task.Usage["total_tokens"] != 20 {
		t.Fatalf("task usage = %+v, want latest usage", task.Usage)
	}
	if task.RuntimeKind != "claude" || !task.Capabilities.Stop || task.Capabilities.SendMessage || task.Capabilities.Resume {
		t.Fatalf("claude capabilities = %+v runtime=%q", task.Capabilities, task.RuntimeKind)
	}
}

func TestSubagentTaskRuntimeSessionKeyUsesHostAgent(t *testing.T) {
	task := SubagentTask{
		TaskID:      "task-1",
		SessionKey:  protocol.BuildRoomSharedSessionKey("conversation-1"),
		AgentID:     "sdk-subagent-1",
		HostAgentID: "host-agent-1",
	}
	want := protocol.BuildRoomAgentSessionKey("conversation-1", "host-agent-1", protocol.RoomTypeGroup)
	if got := subagentTaskRuntimeSessionKey(task); got != want {
		t.Fatalf("subagentTaskRuntimeSessionKey() = %q, want %q", got, want)
	}
}

func TestUnknownSubagentRuntimeCapabilitiesAreReadOnly(t *testing.T) {
	capabilities := subagentTaskCapabilities("unknown")
	if !capabilities.Observe || !capabilities.Transcript {
		t.Fatalf("unknown runtime 应保留可观测能力: %+v", capabilities)
	}
	if capabilities.Stop || capabilities.SendMessage || capabilities.Resume {
		t.Fatalf("unknown runtime 不应开放管理能力: %+v", capabilities)
	}
}

func TestBuildSubagentTasksExcludesLocalShellBackgroundTasks(t *testing.T) {
	messages := []protocol.Message{
		{
			"agent_id": "host-agent",
			"metadata": map[string]any{
				"subtype":    "task_started",
				"task_id":    "shell-task-1",
				"agent_id":   "host-agent",
				"agent_type": "shell",
				"task_type":  "local_shell",
			},
		},
		{
			"metadata": map[string]any{
				"subtype":   "task_progress",
				"task_id":   "shell-task-1",
				"task_type": "local_shell",
				"summary":   "npm test",
			},
		},
		{
			"metadata": map[string]any{
				"subtype": "task_notification",
				"task_id": "shell-task-1",
				"status":  "completed",
			},
		},
	}

	if tasks := buildSubagentTasks("agent:host:ws:dm:conversation-1", messages); len(tasks) != 0 {
		t.Fatalf("local_shell 不应进入 subagent 列表: %+v", tasks)
	}
}

func TestReadSubagentTaskThreadUsesCCOutputSymlinkAsTranscript(t *testing.T) {
	root := t.TempDir()
	transcriptPath := filepath.Join(root, "child.jsonl")
	transcript := "" +
		`{"type":"user","uuid":"user-1","parentUuid":null,"isSidechain":true,"agentId":"child-1","timestamp":"2026-07-10T10:00:00Z","message":{"role":"user","content":"检查实现"}}` + "\n" +
		`{"type":"attachment","uuid":"attachment-1","parentUuid":"user-1","isSidechain":true,"agentId":"child-1","timestamp":"2026-07-10T10:00:00.500Z","attachment":{"type":"skill_listing","content":"- memory-manager"}}` + "\n" +
		`{"type":"assistant","uuid":"assistant-thinking","parentUuid":"attachment-1","isSidechain":true,"agentId":"child-1","timestamp":"2026-07-10T10:00:01Z","message":{"role":"assistant","id":"assistant-message","model":"claude","content":[{"type":"thinking","thinking":"先阅读核心代码"}],"stop_reason":null}}` + "\n" +
		`{"type":"assistant","uuid":"assistant-final","parentUuid":"assistant-thinking","isSidechain":true,"agentId":"child-1","timestamp":"2026-07-10T10:00:02Z","message":{"role":"assistant","id":"assistant-message","model":"claude","content":[{"type":"text","text":"检查完成"}],"stop_reason":"end_turn"}}` + "\n"
	if err := os.WriteFile(transcriptPath, []byte(transcript), 0o600); err != nil {
		t.Fatalf("写入 child transcript 失败: %v", err)
	}
	outputPath := filepath.Join(root, "task-output")
	if err := os.Symlink(transcriptPath, outputPath); err != nil {
		t.Fatalf("创建 output_file 符号链接失败: %v", err)
	}

	service := &Service{history: workspacestore.NewAgentHistoryStore(root)}
	messages, outputIsTranscript, err := service.readSubagentTaskThread(SubagentTask{
		TaskID:      "task-cc",
		SessionKey:  "agent:host:ws:dm:conversation-1",
		HostAgentID: "host",
		TaskType:    "local_agent",
		OutputFile:  outputPath,
	}, root)
	if err != nil {
		t.Fatalf("readSubagentTaskThread() error = %v", err)
	}
	if !outputIsTranscript || len(messages) != 2 {
		t.Fatalf("CC output_file 未投影成富消息: used=%v messages=%+v", outputIsTranscript, messages)
	}
	if messages[0]["role"] != "user" || messages[len(messages)-1]["role"] != "assistant" {
		t.Fatalf("CC thread 角色序列不正确: %+v", messages)
	}
	content, err := json.Marshal(messages[len(messages)-1]["content"])
	if err != nil ||
		!strings.Contains(string(content), `"type":"thinking"`) ||
		!strings.Contains(string(content), `"thinking":"先阅读核心代码"`) ||
		!strings.Contains(string(content), `"type":"text"`) ||
		!strings.Contains(string(content), `"text":"检查完成"`) {
		t.Fatalf("CC 最终消息内容不正确: content=%s err=%v messages=%+v", content, err, messages)
	}
}

func TestReadSubagentTaskThreadFallsBackFromPlainOutput(t *testing.T) {
	root := t.TempDir()
	outputPath := filepath.Join(root, "task-output.txt")
	if err := os.WriteFile(outputPath, []byte("普通任务输出"), 0o600); err != nil {
		t.Fatalf("写入普通 output 失败: %v", err)
	}

	service := &Service{history: workspacestore.NewAgentHistoryStore(root)}
	messages, outputIsTranscript, err := service.readSubagentTaskThread(SubagentTask{
		TaskID:      "task-cc",
		SessionKey:  "agent:host:ws:dm:conversation-1",
		HostAgentID: "host",
		TaskType:    "local_agent",
		OutputFile:  outputPath,
	}, root)
	if err != nil || outputIsTranscript || len(messages) != 0 {
		t.Fatalf("普通 output 不应被当作 transcript: used=%v messages=%+v err=%v", outputIsTranscript, messages, err)
	}
	output, err := readSubagentOutputFile(outputPath)
	if err != nil || output != "普通任务输出" {
		t.Fatalf("普通 output 回退失败: output=%q err=%v", output, err)
	}
}
