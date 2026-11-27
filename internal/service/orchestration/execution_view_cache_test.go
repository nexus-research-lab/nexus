package orchestration

import (
	"context"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestExecutionViewCacheEvictsLeastRecentlyUsed(t *testing.T) {
	var cache executionViewCache
	compute := func(context.Context) (*protocol.ExecutionView, error) { return &protocol.ExecutionView{}, nil }
	first := &protocol.Execution{ID: "execution-first", OwnerUserID: "owner", SessionKey: "session"}
	_, _ = cache.load(t.Context(), first, compute)
	for index := range executionViewCacheLimit {
		execution := &protocol.Execution{ID: "execution-" + string(rune('a'+index)), OwnerUserID: "owner", SessionKey: "session"}
		_, _ = cache.load(t.Context(), execution, compute)
	}
	if len(cache.entries) != executionViewCacheLimit {
		t.Fatalf("entries = %d, want %d", len(cache.entries), executionViewCacheLimit)
	}
	if _, ok := cache.entries[first.ID]; ok {
		t.Fatalf("least recently used projection survived eviction")
	}
}
