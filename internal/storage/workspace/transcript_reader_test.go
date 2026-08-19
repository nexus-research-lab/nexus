package workspace

import "testing"

func TestBuildPrimaryTranscriptChainWalksDuplicateSelfParentUUID(t *testing.T) {
	entries := []transcriptEntry{
		{Index: 0, Data: map[string]any{"uuid": "user-1", "type": "user"}},
		{Index: 1, Data: map[string]any{"uuid": "assistant-1", "parentUuid": "user-1", "type": "assistant"}},
		{Index: 2, Data: map[string]any{"uuid": "tool-result-1", "parentUuid": "assistant-1", "type": "user"}},
		{Index: 3, Data: map[string]any{"uuid": "system-duplicate", "parentUuid": "tool-result-1", "type": "system"}},
		{Index: 4, Data: map[string]any{"uuid": "system-duplicate", "parentUuid": "system-duplicate", "type": "system"}},
		{Index: 5, Data: map[string]any{"uuid": "assistant-2", "parentUuid": "system-duplicate", "type": "assistant"}},
	}

	chain := buildPrimaryTranscriptChain(entries)
	if len(chain) != len(entries) {
		t.Fatalf("chain length = %d, want %d: %+v", len(chain), len(entries), chain)
	}
	for index, entry := range chain {
		if entry.Index != index {
			t.Fatalf("chain[%d].Index = %d, want %d", index, entry.Index, index)
		}
	}
}
