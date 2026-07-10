package runtime

import (
	"testing"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

func TestSummarizeSystemMessageKeepsSubagentThreadMetadata(t *testing.T) {
	tests := []struct {
		name    string
		message *sdkprotocol.SystemMessage
		want    map[string]any
	}{
		{
			name: "started",
			message: &sdkprotocol.SystemMessage{
				Subtype: "task_started",
				TaskStarted: &sdkprotocol.TaskStartedMessage{
					TaskID:       "task-1",
					AgentID:      "subagent-1",
					AgentType:    "worker",
					Description:  "检查实现",
					TaskType:     "local_agent",
					OutputFile:   "/tmp/task.out",
					ParentTaskID: "parent-1",
					Prompt:       "请检查实现",
					Additional: map[string]any{
						"child_session_id": "child-1",
						"model":            "gpt-5",
						"name":             "实现审计",
					},
				},
			},
			want: map[string]any{
				"agent_id": "subagent-1", "agent_type": "worker", "child_session_id": "child-1",
				"description": "检查实现", "task_type": "local_agent", "output_file": "/tmp/task.out",
				"parent_task_id": "parent-1", "prompt": "请检查实现", "model": "gpt-5", "name": "实现审计",
			},
		},
		{
			name: "progress",
			message: &sdkprotocol.SystemMessage{
				Subtype: "task_progress",
				TaskProgress: &sdkprotocol.TaskProgressMessage{
					TaskID:       "task-1",
					AgentID:      "subagent-1",
					AgentType:    "worker",
					Description:  "正在检查",
					LastToolName: "Read",
					ParentTaskID: "parent-1",
					Summary:      "读取核心实现",
					Additional: map[string]any{
						"child_session_id": "child-1",
						"task_type":        "local_agent",
					},
				},
			},
			want: map[string]any{
				"agent_id": "subagent-1", "agent_type": "worker", "child_session_id": "child-1",
				"description": "正在检查", "last_tool_name": "Read", "parent_task_id": "parent-1",
				"summary": "读取核心实现", "task_type": "local_agent",
			},
		},
		{
			name: "notification",
			message: &sdkprotocol.SystemMessage{
				Subtype: "task_notification",
				TaskNotification: &sdkprotocol.TaskNotificationMessage{
					TaskID:         "task-1",
					AgentID:        "subagent-1",
					AgentType:      "worker",
					ParentTaskID:   "parent-1",
					Status:         "completed",
					Summary:        "检查完成",
					TranscriptPath: "/tmp/subagent.jsonl",
					Additional:     map[string]any{"child_session_id": "child-1"},
				},
			},
			want: map[string]any{
				"agent_id": "subagent-1", "agent_type": "worker", "child_session_id": "child-1",
				"parent_task_id": "parent-1", "status": "completed", "summary": "检查完成",
				"transcript_path": "/tmp/subagent.jsonl",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			summary, ok := SummarizeSystemMessage(test.message)
			if !ok {
				t.Fatal("SummarizeSystemMessage() = false")
			}
			for key, want := range test.want {
				if got := summary.Metadata[key]; got != want {
					t.Fatalf("metadata[%q] = %#v, want %#v; all=%+v", key, got, want, summary.Metadata)
				}
			}
		})
	}
}

func TestSummarizeSystemMessageKeepsMemorySavedPaths(t *testing.T) {
	summary, ok := SummarizeSystemMessage(&sdkprotocol.SystemMessage{
		Subtype: "memory_saved",
		MemorySaved: &sdkprotocol.MemorySavedMessage{
			Verb:         "Improved",
			WrittenPaths: []string{"/memory/project.md"},
		},
	})
	if !ok || summary.Subtype != "memory_saved" || summary.Content != "长期记忆已整理" {
		t.Fatalf("summary = %#v, want memory_saved", summary)
	}
	paths, ok := summary.Metadata["written_paths"].([]string)
	if !ok || len(paths) != 1 || paths[0] != "/memory/project.md" {
		t.Fatalf("written_paths = %#v", summary.Metadata["written_paths"])
	}
}
