package message

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

func TestSubagentTaskUsageSnapshot(t *testing.T) {
	taskID, totalTokens, ok := SubagentTaskUsageSnapshot(protocol.Message{
		"metadata": map[string]any{
			"task_id": "task-1",
			"usage":   map[string]any{"total_tokens": int64(150)},
		},
	})
	if !ok || taskID != "task-1" || totalTokens != 150 {
		t.Fatalf("snapshot = %q/%d/%v, want task-1/150/true", taskID, totalTokens, ok)
	}
}

func TestTaskUsageMapPreservesExplicitZeroFromRawAdditional(t *testing.T) {
	got := taskUsageMap(
		sdkprotocol.TaskUsage{},
		map[string]any{
			"total_tokens": 0,
			"tool_uses":    0,
			"duration_ms":  0,
		},
	)
	want := map[string]any{
		"total_tokens": int64(0),
		"tool_uses":    int64(0),
		"duration_ms":  int64(0),
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("task usage = %#v, want explicit zero presence %#v", got, want)
	}
}

func TestSubagentTaskUsageSnapshotsCollectsMetadataAndAssistantBlocks(t *testing.T) {
	message := protocol.Message{
		"metadata": map[string]any{
			"task_id": "task-b",
			"usage":   map[string]any{"total_tokens": int64(90)},
		},
		"content": []map[string]any{
			{
				"type":    "task_progress",
				"task_id": "task-b",
				"usage":   map[string]any{"total_tokens": 120},
			},
			{
				"type":    "task_progress",
				"task_id": "task-a",
				"usage":   map[string]any{"total_tokens": json.Number("75")},
			},
			{
				"type":    "task_progress",
				"task_id": "task-b",
				"usage":   map[string]any{"total_tokens": 110},
			},
			{
				"type":    "text",
				"task_id": "ignored",
				"usage":   map[string]any{"total_tokens": 999},
			},
		},
	}

	got := SubagentTaskUsageSnapshots(message)
	want := []SubagentTaskUsage{
		{TaskID: "task-a", TotalTokens: 75},
		{TaskID: "task-b", TotalTokens: 120},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("snapshots = %#v, want %#v", got, want)
	}
}

func TestSubagentTaskUsageSnapshotsSupportsDecodedContentAndRejectsPlaceholderZero(t *testing.T) {
	message := protocol.Message{
		"content": []any{
			map[string]any{
				"type":    "task_progress",
				"task_id": "task-c",
				"usage":   map[string]any{"total_tokens": float64(42)},
			},
			map[string]any{
				"type":    "task_progress",
				"task_id": "task-empty",
				"usage":   map[string]any{"total_tokens": 0},
			},
			map[string]any{
				"type":    "task_progress",
				"task_id": "task-invalid",
				"usage":   map[string]any{"total_tokens": "invalid"},
			},
			map[string]any{
				"type":    "task_progress",
				"task_id": " ",
				"usage":   map[string]any{"total_tokens": 88},
			},
		},
	}

	got := SubagentTaskUsageSnapshots(message)
	want := []SubagentTaskUsage{
		{TaskID: "task-c", TotalTokens: 42},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("snapshots = %#v, want %#v", got, want)
	}
}
