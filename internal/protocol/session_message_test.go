package protocol

import "testing"

func TestIsTranscriptNativeMessageRequiresOverlayForTaskProgress(t *testing.T) {
	normalAssistant := Message{
		"role": "assistant",
		"content": []map[string]any{
			{"type": "text", "text": "done"},
		},
	}
	if !IsTranscriptNativeMessage(normalAssistant) {
		t.Fatal("普通 assistant 正文应由 transcript 提供")
	}

	taskProgressAssistant := Message{
		"role": "assistant",
		"content": []any{
			map[string]any{"type": "tool_use", "id": "call-agent", "name": "Agent"},
			map[string]any{"type": "task_progress", "task_id": "agent-1"},
		},
	}
	if IsTranscriptNativeMessage(taskProgressAssistant) {
		t.Fatal("Nexus 注入的 task_progress 不在 transcript 中，必须写入 overlay")
	}
}
