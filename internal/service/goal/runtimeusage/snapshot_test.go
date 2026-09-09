package runtimeusage

import (
	"testing"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/runtime/exec"
)

func TestFinalSnapshotPreservesZeroMissingAndFallbackUsage(t *testing.T) {
	assistant := protocol.Message{"role": "assistant", "message_id": " turn ", "usage": map[string]any{"input_tokens": 80, "output_tokens": 20}}
	zero, ok := FinalSnapshot(exec.RoundExecutionResult{Usage: sdkprotocol.TokenUsage{Raw: map[string]any{"total_tokens": int64(0)}}, ElapsedTimeSeconds: 3}, assistant, 5)
	if !ok || !zero.Cumulative || !zero.Terminal || !zero.TokenUsageObserved || zero.Usage.ActualTokens() != 0 || zero.ElapsedSeconds != 3 {
		t.Fatalf("显式零 result 必须覆盖 assistant: %#v", zero)
	}
	fallback, ok := FinalSnapshot(exec.RoundExecutionResult{}, assistant, 5)
	if !ok || fallback.Cumulative || !fallback.TokenUsageObserved || fallback.Usage.ActualTokens() != 100 || fallback.TurnID != "turn" || fallback.ElapsedSeconds != 5 {
		t.Fatalf("缺失 result 时应保留逐 turn 回退: %#v", fallback)
	}
	missing, ok := FinalSnapshot(exec.RoundExecutionResult{}, nil, 5)
	if !ok || missing.TokenUsageObserved || missing.Cumulative || missing.ElapsedSeconds != 5 {
		t.Fatalf("仅耗时不能伪装成已观察 token: %#v", missing)
	}
}

func TestSubagentObservationsDistinguishTerminalEvidence(t *testing.T) {
	for _, tc := range []struct {
		name, taskType string
		tokens         int64
		known          bool
		wantCount      int
		wantUsage      bool
	}{
		{"带用量终态", "local_agent", 25, false, 1, true},
		{"缺失用量终态", "local_agent", 0, false, 1, false},
		{"已知任务省略身份", "", 0, true, 1, false},
		{"未知任务省略身份", "", 0, false, 0, false},
		{"本地 shell", "local_shell", 0, false, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			message := protocol.Message{"metadata": map[string]any{"task_id": " child ", "task_type": tc.taskType, "status": "completed", "usage": map[string]any{"total_tokens": tc.tokens}}}
			got := SubagentObservations(message, func(id string) bool { return tc.known && id == "child" })
			if len(got) != tc.wantCount {
				t.Fatalf("观察数量 = %d", len(got))
			}
			if len(got) == 0 {
				return
			}
			if got[0].TaskID != "child" || !got[0].Usage.Terminal || got[0].Usage.TerminalTokenUsageObserved != tc.wantUsage || got[0].Usage.ObservedAt.IsZero() {
				t.Fatalf("终态证据不正确: %#v", got[0])
			}
		})
	}
}
