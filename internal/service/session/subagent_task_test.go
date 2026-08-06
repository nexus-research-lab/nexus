package session

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

func buildSubagentTasks(sessionKey string, messages []protocol.Message) []SubagentTask {
	return buildSubagentTasksWithRuntime(sessionKey, messages, string(agentclient.RuntimeNXS))
}

// syscall 只在 Windows 构建导出该名称；数值来自 ERROR_PRIVILEGE_NOT_HELD。
const windowsSymlinkPrivilegeNotHeld syscall.Errno = 1314

func createSubagentTestSymlink(t *testing.T, target string, link string) {
	t.Helper()

	err := os.Symlink(target, link)
	if err == nil {
		return
	}
	if runtime.GOOS == "windows" && (errors.Is(err, windowsSymlinkPrivilegeNotHeld) ||
		errors.Is(err, os.ErrPermission) ||
		errors.Is(err, errors.ErrUnsupported)) {
		t.Skipf("symlink unavailable: %v", err)
	}
	t.Fatalf("创建测试符号链接失败: %v", err)
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
	if task.Name != "统计HTML小游戏数量" {
		t.Fatalf("task name = %q, want model-provided description", task.Name)
	}
	if task.Usage["total_tokens"] != 321 || task.UpdatedAt != 1000 {
		t.Fatalf("task metrics = %+v/%d, want usage and updated time", task.Usage, task.UpdatedAt)
	}
}

func TestBuildSubagentTasksSettlesAgentProgressFromToolResult(t *testing.T) {
	for _, testCase := range []struct {
		name       string
		isError    bool
		wantStatus string
	}{
		{name: "completed", wantStatus: "completed"},
		{name: "failed", isError: true, wantStatus: "failed"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			messages := []protocol.Message{
				{
					"content": []any{
						map[string]any{
							"type":        "task_progress",
							"task_id":     "agent-child-1",
							"tool_use_id": "call-agent",
							"agent_id":    "agent-child-1",
							"agent_type":  "Explore",
							"description": "调研产品规格",
							"status":      "running",
						},
					},
					"round_id":  "round-1",
					"role":      "assistant",
					"timestamp": int64(1000),
				},
				{
					"content": []any{
						map[string]any{
							"type":        "tool_result",
							"tool_use_id": "call-agent",
							"is_error":    testCase.isError,
						},
					},
					"round_id":  "round-1",
					"role":      "assistant",
					"timestamp": int64(2000),
				},
			}

			tasks := buildSubagentTasks("agent:host:ws:dm:test", messages)
			if len(tasks) != 1 {
				t.Fatalf("len(tasks) = %d, want 1", len(tasks))
			}
			if tasks[0].Status != testCase.wantStatus || tasks[0].UpdatedAt != 2000 {
				t.Fatalf("task = %+v, want status %s at 2000", tasks[0], testCase.wantStatus)
			}
		})
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

func TestReadSubagentTaskThreadUsesIndependentAgentTranscript(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv(appfs.NexusStateRootEnvName, stateRoot)
	ownerUserID := "owner-subagent-thread"
	workspacePath := filepath.Join(
		appfs.UserWorkspaceRootAt(stateRoot, ownerUserID),
		"host-agent",
	)
	if err := os.MkdirAll(workspacePath, 0o700); err != nil {
		t.Fatal(err)
	}
	childSessionID := "agent-100a6a9587387094a687c45764874d8c"
	projectDir := filepath.Join(
		appfs.UserRuntimeRootAt(stateRoot, ownerUserID),
		"projects",
		workspacestore.TranscriptProjectDirectoryName(workspacePath),
	)
	if err := os.MkdirAll(projectDir, 0o700); err != nil {
		t.Fatal(err)
	}
	transcript := "" +
		`{"type":"user","uuid":"child-user","parentUuid":null,"sessionId":"` + childSessionID + `","timestamp":"2026-07-28T07:34:00Z","message":{"role":"user","content":"检查上下文文件"}}` + "\n" +
		`{"type":"assistant","uuid":"child-assistant-tool","parentUuid":"child-user","sessionId":"` + childSessionID + `","timestamp":"2026-07-28T07:34:01Z","message":{"role":"assistant","id":"child-message","model":"glm","content":[{"type":"thinking","thinking":"先读取配置"},{"type":"tool_use","id":"read-1","name":"Read","input":{"file_path":"AGENTS.md"}}],"stop_reason":"tool_use"}}` + "\n" +
		`{"type":"user","uuid":"child-tool-result","parentUuid":"child-assistant-tool","sessionId":"` + childSessionID + `","timestamp":"2026-07-28T07:34:02Z","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"read-1","content":"项目约束"}]}}` + "\n" +
		`{"type":"assistant","uuid":"child-assistant-final","parentUuid":"child-tool-result","sessionId":"` + childSessionID + `","timestamp":"2026-07-28T07:34:03Z","message":{"role":"assistant","id":"child-message-final","model":"glm","content":[{"type":"text","text":"已读取并继续调研"}],"stop_reason":"end_turn"}}` + "\n"
	if err := os.WriteFile(
		filepath.Join(projectDir, childSessionID+".jsonl"),
		[]byte(transcript),
		0o600,
	); err != nil {
		t.Fatal(err)
	}

	service := &Service{
		history: workspacestore.NewAgentHistoryStore(appfs.UsersRoot()),
	}
	messages, outputIsTranscript, err := service.readSubagentTaskThreadAtOwner(
		ownerUserID,
		true,
		SubagentTask{
			TaskID:      childSessionID,
			SessionKey:  "agent:host-agent:ws:dm:conversation-1",
			AgentID:     childSessionID,
			HostAgentID: "host-agent",
		},
		workspacePath,
	)
	if err != nil {
		t.Fatalf("readSubagentTaskThreadAtOwner() error = %v", err)
	}
	if !outputIsTranscript || len(messages) < 2 {
		t.Fatalf("独立 Agent transcript 未投影成普通线程: used=%v messages=%+v", outputIsTranscript, messages)
	}
	messages = subagentTaskOutputMessages(messages)
	for _, message := range messages {
		if message["role"] == "user" {
			t.Fatalf("子 Agent 详情不应显示父 Agent 输入: %+v", messages)
		}
	}
	content, err := json.Marshal(messages)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`"thinking":"先读取配置"`,
		`"name":"Read"`,
		`"type":"tool_result"`,
		`"text":"已读取并继续调研"`,
	} {
		if !strings.Contains(string(content), want) {
			t.Fatalf("独立 Agent thread 缺少 %s: %s", want, content)
		}
	}
	if strings.Contains(string(content), "检查上下文文件") {
		t.Fatalf("子 Agent thread 泄露父 Agent 任务提示: %s", content)
	}
}

func TestSubagentToolRunsFromMessagesKeepsOnlyLifecycleIdentity(t *testing.T) {
	t.Parallel()

	task := SubagentTask{
		TaskID:    "agent-child-1",
		AgentID:   "agent-child-1",
		ToolUseID: "spawn-tool-1",
	}
	runs := subagentToolRunsFromMessages(task, []protocol.Message{
		{
			"timestamp": int64(1000),
			"content": []any{
				map[string]any{
					"type": "tool_use", "id": "read-1", "name": "Read",
					"input": map[string]any{"secret": "must not leave transcript"},
				},
				map[string]any{
					"type": "tool_result", "tool_use_id": "read-1",
					"content": "private output", "is_error": false,
				},
			},
		},
		{
			"timestamp": int64(2000),
			"content": []any{
				map[string]any{"type": "tool_use", "id": "bash-1", "name": "Bash"},
				map[string]any{"type": "tool_result", "tool_use_id": "bash-1", "is_error": true},
			},
		},
	})
	if len(runs) != 2 ||
		runs[0].ParentToolUseID != "spawn-tool-1" ||
		runs[0].ToolUseID != "read-1" || runs[0].Name != "Read" ||
		runs[0].Status != "succeeded" || runs[0].StartedAt != 1000 || runs[0].FinishedAt != 1000 ||
		runs[1].ToolUseID != "bash-1" || runs[1].Status != "failed" {
		t.Fatalf("subagent Tool runs = %+v", runs)
	}
}

func TestReadSubagentTaskThreadUsesCCOutputSymlinkAsTranscript(t *testing.T) {
	root := t.TempDir()
	transcriptPath := filepath.Join(root, "child.jsonl")
	transcript := "" +
		`{"type":"user","uuid":"user-1","parentUuid":null,"isSidechain":true,"agentId":"child-1","timestamp":"2026-07-10T10:00:00Z","message":{"role":"user","content":"检查实现"}}` + "\n" +
		`{"type":"attachment","uuid":"attachment-1","parentUuid":"user-1","isSidechain":true,"agentId":"child-1","timestamp":"2026-07-10T10:00:00.500Z","attachment":{"type":"skill_listing","content":"- nexus-manager"}}` + "\n" +
		`{"type":"assistant","uuid":"assistant-thinking","parentUuid":"attachment-1","isSidechain":true,"agentId":"child-1","timestamp":"2026-07-10T10:00:01Z","message":{"role":"assistant","id":"assistant-message","model":"claude","content":[{"type":"thinking","thinking":"先阅读核心代码"}],"stop_reason":null}}` + "\n" +
		`{"type":"assistant","uuid":"assistant-final","parentUuid":"assistant-thinking","isSidechain":true,"agentId":"child-1","timestamp":"2026-07-10T10:00:02Z","message":{"role":"assistant","id":"assistant-message","model":"claude","content":[{"type":"text","text":"检查完成"}],"stop_reason":"end_turn"}}` + "\n"
	if err := os.WriteFile(transcriptPath, []byte(transcript), 0o600); err != nil {
		t.Fatalf("写入 child transcript 失败: %v", err)
	}
	outputPath := filepath.Join(root, "task-output")
	createSubagentTestSymlink(t, transcriptPath, outputPath)

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
	messages = subagentTaskOutputMessages(messages)
	if len(messages) != 1 || messages[0]["role"] != "assistant" {
		t.Fatalf("CC thread 应只保留子 Agent 输出: %+v", messages)
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

func TestReadSubagentTaskThreadRejectsCCOutputSymlinkOutsideWorkspace(t *testing.T) {
	root := t.TempDir()
	outsideRoot := t.TempDir()
	transcriptPath := filepath.Join(outsideRoot, "child.jsonl")
	transcript := `{"type":"assistant","uuid":"assistant-final","parentUuid":null,"isSidechain":true,"agentId":"child-1","timestamp":"2026-07-10T10:00:02Z","message":{"role":"assistant","id":"assistant-message","model":"claude","content":[{"type":"text","text":"不应读取"}],"stop_reason":"end_turn"}}` + "\n"
	if err := os.WriteFile(transcriptPath, []byte(transcript), 0o600); err != nil {
		t.Fatalf("写入 workspace 外 transcript 失败: %v", err)
	}
	outputPath := filepath.Join(root, "task-output")
	createSubagentTestSymlink(t, transcriptPath, outputPath)

	service := &Service{history: workspacestore.NewAgentHistoryStore(root)}
	messages, outputIsTranscript, err := service.readSubagentTaskThread(SubagentTask{
		TaskID:      "task-cc",
		SessionKey:  "agent:host:ws:dm:conversation-1",
		HostAgentID: "host",
		TaskType:    "local_agent",
		OutputFile:  outputPath,
	}, root)
	if err != nil || outputIsTranscript || len(messages) != 0 {
		t.Fatalf("workspace 外 CC output_file 不应被读取: used=%v messages=%+v err=%v", outputIsTranscript, messages, err)
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
	output, err := readSubagentOutputFile(outputPath, root)
	if err != nil || output != "普通任务输出" {
		t.Fatalf("普通 output 回退失败: output=%q err=%v", output, err)
	}
}

func TestReadSubagentOutputFileRejectsCrossOwnerWorkspaceSymlink(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv(appfs.NexusStateRootEnvName, stateRoot)

	ownerAWorkspace := filepath.Join(
		appfs.UserWorkspaceRootAt(stateRoot, "user-a"),
		"agent-a",
	)
	ownerBWorkspace := filepath.Join(
		appfs.UserWorkspaceRootAt(stateRoot, "user-b"),
		"agent-b",
	)
	if err := os.MkdirAll(filepath.Dir(ownerAWorkspace), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(ownerBWorkspace, 0o700); err != nil {
		t.Fatal(err)
	}
	outputPath := filepath.Join(ownerBWorkspace, "output.txt")
	if err := os.WriteFile(outputPath, []byte("owner-b-secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	createSubagentTestSymlink(t, ownerBWorkspace, ownerAWorkspace)

	service := &Service{config: config.Config{WorkspacePath: appfs.UsersRoot()}}
	ctx := authctx.WithPrincipal(context.Background(), &authctx.Principal{
		UserID: "user-a",
	})
	output, err := service.readSubagentOutputFile(
		ctx,
		filepath.Join(ownerAWorkspace, "output.txt"),
		ownerAWorkspace,
	)
	if output != "" || !errors.Is(err, confinedfs.ErrSymlink) {
		t.Fatalf("跨 owner workspace symlink 应被拒绝: output=%q err=%v", output, err)
	}
}
