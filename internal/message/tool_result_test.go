package message

import (
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestAssistantToolResultsMapsToolNames(t *testing.T) {
	message := protocol.Message{
		"role": "assistant",
		"content": []map[string]any{
			{"type": "text", "text": "working"},
			{"type": "tool_use", "id": "tool-1", "name": "read_file"},
			{"type": "tool_result", "tool_use_id": "tool-1"},
			{"type": "tool_result", "tool_use_id": "missing", "is_error": true},
		},
	}

	results := AssistantToolResults(message)
	if len(results) != 2 {
		t.Fatalf("len(results) = %d, want 2", len(results))
	}
	if results[0].ToolUseID != "tool-1" || results[0].ToolName != "read_file" || results[0].IsError {
		t.Fatalf("results[0] = %#v, want read_file success", results[0])
	}
	if results[1].ToolUseID != "missing" || results[1].ToolName != "" || !results[1].IsError {
		t.Fatalf("results[1] = %#v, want unmatched error", results[1])
	}
}

func TestAssistantToolResultsIgnoresNonAssistant(t *testing.T) {
	results := AssistantToolResults(protocol.Message{
		"role":    "user",
		"content": []any{map[string]any{"type": "tool_result", "tool_use_id": "tool-1"}},
	})
	if len(results) != 0 {
		t.Fatalf("results = %#v, want none", results)
	}
}
