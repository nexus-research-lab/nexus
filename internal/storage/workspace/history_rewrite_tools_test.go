package workspace

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestAgentHistoryStoreResolveTranscriptRoundTailIncludesInterruptedToolResults(t *testing.T) {
	root := t.TempDir()
	workspaceRoot := filepath.Join(root, "workspace")
	workspacePath := filepath.Join(workspaceRoot, "Amy")
	if err := os.MkdirAll(workspacePath, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("NEXUS_STATE_ROOT", "")
	t.Setenv("NEXUS_CONFIG_DIR", filepath.Join(root, "runtime"))

	sessionKey := "agent:nexus:ws:dm:rewrite-interrupted-tools"
	sessionID := "session-rewrite-interrupted-tools"
	history := NewAgentHistoryStore(workspaceRoot)
	for index, round := range []struct{ id, prompt string }{
		{"round-before", "只回复准备好了"},
		{"round-tools", "先执行 echo ready，完成后再执行 sleep 60"},
	} {
		if err := history.AppendRoundMarker(workspacePath, sessionKey, round.id, round.prompt, int64(index+1)*1000); err != nil {
			t.Fatal(err)
		}
	}

	// Use persisted transcript UUIDs: tool results are user messages but must
	// stay inside the same round tail as both independent assistant tool calls.
	entries := []struct {
		uuid    string
		role    string
		content any
	}{
		{"user-before", "user", "只回复准备好了"},
		{"assistant-before", "assistant", []map[string]any{{"type": "text", "text": "准备好了"}}},
		{"user-tools", "user", "先执行 echo ready，完成后再执行 sleep 60"},
		{"assistant-echo", "assistant", []map[string]any{{
			"type": "tool_use", "id": "call-echo", "name": "Bash", "input": map[string]any{"command": "echo ready"},
		}}},
		{"result-echo", "user", []map[string]any{{
			"type": "tool_result", "tool_use_id": "call-echo", "content": "ready", "is_error": false,
		}}},
		{"assistant-sleep", "assistant", []map[string]any{{
			"type": "tool_use", "id": "call-sleep", "name": "Bash", "input": map[string]any{"command": "sleep 60"},
		}}},
		{"result-interrupted", "user", []map[string]any{{
			"type": "tool_result", "tool_use_id": "call-sleep", "content": "[Request interrupted by user]", "is_error": true,
		}}},
	}
	rows := make([]map[string]any, 0, len(entries))
	parentUUID := ""
	for index, entry := range entries {
		rows = append(rows, map[string]any{
			"type": entry.role, "uuid": entry.uuid, "sessionId": sessionID,
			"parentUuid": parentUUID, "timestamp": int64(index+1) * 1000,
			"message": map[string]any{"role": entry.role, "content": entry.content},
		})
		parentUUID = entry.uuid
	}
	writeAgentTranscriptFixture(t, workspacePath, sessionID, rows)

	tail, err := history.ResolveTranscriptRoundTail(workspacePath, sessionKey, sessionID, "round-tools")
	if err != nil {
		t.Fatal(err)
	}
	want := TranscriptRoundTail{
		TargetRoundID:      "round-tools",
		TargetMessageUUID:  "user-tools",
		TargetRoundEndUUID: "result-interrupted",
		MessageUUIDs:       []string{"user-tools", "assistant-echo", "result-echo", "assistant-sleep", "result-interrupted"},
		RoundIDs:           []string{"round-tools"},
	}
	if !reflect.DeepEqual(tail, want) {
		t.Fatalf("rewrite must remove both tool pairs and preserve the preceding text round: got %#v, want %#v", tail, want)
	}
}
