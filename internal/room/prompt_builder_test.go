package room

import (
	"testing"

	sessionmodel "github.com/nexus-research-lab/nexus/internal/model/session"
)

func TestBuildHistoryLinesFiltersIncompleteAssistant(t *testing.T) {
	history := []sessionmodel.Message{
		{"role": "user", "content": "你好"},
		{"role": "assistant", "agent_id": "a1", "content": []map[string]any{{"type": "text", "text": "半成品"}}, "is_complete": false},
		{"role": "assistant", "agent_id": "a1", "content": []map[string]any{{"type": "text", "text": "已完成"}}, "is_complete": true},
	}
	lines := buildHistoryLines(history, map[string]string{"a1": "Agent1"})
	if len(lines) != 2 {
		t.Fatalf("应只保留 user 和 is_complete=true 的 assistant: %+v", lines)
	}
	if lines[0] != "User: 你好" {
		t.Fatalf("第一行不正确: %s", lines[0])
	}
	if lines[1] != "Assistant(Agent1): 已完成" {
		t.Fatalf("第二行不正确: %s", lines[1])
	}
}
