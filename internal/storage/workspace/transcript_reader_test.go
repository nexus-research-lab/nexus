package workspace

import "testing"

func TestTranscriptContinuationPromptIsSkippedFromBothChains(t *testing.T) {
	entry := map[string]any{
		"type": "user",
		"message": map[string]any{
			"role":    "user",
			"content": "Output token limit hit. Resume directly — no apology, no recap of what you were doing. Pick up mid-thought if that is where the cut happened. Break remaining work into smaller pieces.",
		},
	}

	if !shouldSkipTranscriptEntry(entry) {
		t.Fatalf("主 transcript chain 应跳过内部续跑提示")
	}
	if !shouldSkipExplicitTranscriptEntry(entry) {
		t.Fatalf("显式 transcript chain 应跳过内部续跑提示")
	}
}

func TestTranscriptContinuationPromptDoesNotSkipSimilarUserContent(t *testing.T) {
	entry := map[string]any{
		"type": "user",
		"message": map[string]any{
			"role":    "user",
			"content": "Output token limit hit. Resume directly — please explain what happened.",
		},
	}

	if shouldSkipTranscriptEntry(entry) || shouldSkipExplicitTranscriptEntry(entry) {
		t.Fatalf("普通用户内容不应被当成内部续跑提示")
	}
}
