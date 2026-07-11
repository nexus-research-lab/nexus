package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEmotionCommand(t *testing.T) {
	cfg := newCLITestConfig(t)
	workspace := t.TempDir()
	t.Setenv(nexusctlWorkspacePathEnvName, workspace)

	basePayload := runCLICommand(
		t,
		cfg,
		"emotion",
		"reset",
		"--mood",
		"playful",
		"--energy",
		"7",
		"--valence",
		"8",
		"--note",
		"curious, warm, lightly mischievous",
	)
	baseView := asMap(t, basePayload["view"])
	base := asMap(t, baseView["base"])
	if base["mood"] != "playful" || base["energy"] != float64(7) || base["valence"] != float64(8) {
		t.Fatalf("reset 输出不正确: %+v", basePayload)
	}

	contextPayload := runCLICommand(
		t,
		cfg,
		"emotion",
		"--workspace",
		workspace,
		"note",
		"--context-id",
		"dm:abc",
		"--mood",
		"annoyed",
		"--valence",
		"4",
		"--reason",
		"user said the draft feels wrong",
	)
	contextView := asMap(t, contextPayload["view"])
	contextValue := asMap(t, contextView["context"])
	if contextValue["mood"] != "annoyed" || contextValue["valence"] != float64(4) {
		t.Fatalf("note 输出不正确: %+v", contextPayload)
	}
	composite := asMap(t, contextView["composite"])
	if composite["mood"] != "annoyed" || composite["valence"] != float64(6) {
		t.Fatalf("合成情绪不正确: %+v", contextPayload)
	}

	statusPayload := runCLICommand(
		t,
		cfg,
		"emotion",
		"--workspace",
		workspace,
		"status",
		"--context-id",
		"dm:abc",
	)
	statusView := asMap(t, statusPayload["view"])
	if statusView["context_id"] != "dm:abc" {
		t.Fatalf("status context_id 不正确: %+v", statusPayload)
	}
	if _, err := os.Stat(filepath.Join(workspace, ".agents", "emotion.json")); err != nil {
		t.Fatalf("emotion state 未写入 .agents/emotion.json: %v", err)
	}
}
