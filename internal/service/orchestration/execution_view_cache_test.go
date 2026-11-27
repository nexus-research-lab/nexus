package orchestration

import (
	"context"
	"errors"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestExecutionViewCacheSkipsErrorsPendingHistoryAndForeignScopes(t *testing.T) {
	var cache executionViewCache
	execution := &protocol.Execution{ID: "execution-1", OwnerUserID: "owner", SessionKey: "session"}
	calls := 0
	failing := func(context.Context) (*protocol.ExecutionView, error) {
		calls++
		return nil, errors.New("read failed")
	}
	_, _ = cache.load(t.Context(), execution, failing)
	_, _ = cache.load(t.Context(), execution, failing)
	if calls != 2 {
		t.Fatalf("failed projection was cached: calls=%d", calls)
	}

	calls = 0
	pending := func(context.Context) (*protocol.ExecutionView, error) {
		calls++
		return &protocol.ExecutionView{Graph: protocol.ExecutionGraphView{HistoryPending: true}}, nil
	}
	_, _ = cache.load(t.Context(), execution, pending)
	_, _ = cache.load(t.Context(), execution, pending)
	if calls != 2 {
		t.Fatalf("history_pending projection was cached: calls=%d", calls)
	}

	calls = 0
	counting := func(context.Context) (*protocol.ExecutionView, error) {
		calls++
		return &protocol.ExecutionView{}, nil
	}
	_, _ = cache.load(t.Context(), execution, counting)
	foreign := *execution
	foreign.SessionKey = "other-session"
	_, _ = cache.load(t.Context(), &foreign, counting)
	if calls != 2 {
		t.Fatalf("foreign scope reused cached projection: calls=%d", calls)
	}
}

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

func TestPlanlessRuntimeWriteDropsViewOfClaimedRootRound(t *testing.T) {
	service := &Service{}
	execution := &protocol.Execution{ID: "execution-1", OwnerUserID: "owner", SessionKey: "session", RootRoundID: "root-1"}
	_, _ = service.viewCache.load(t.Context(), execution, func(context.Context) (*protocol.ExecutionView, error) {
		return &protocol.ExecutionView{}, nil
	})
	sink := &cacheObservingSink{service: service, executionID: execution.ID}
	service.SetExecutionInvalidationSink(sink)
	service.invalidateActor(t.Context(), ActorContext{
		OwnerUserID: "owner", SessionKey: "session", AgentID: "agent", RootRoundID: "root-1", AgentRoundID: "round-2",
	})
	if _, ok := service.viewCache.entries[execution.ID]; ok {
		t.Fatalf("planless write in claimed root round kept stale projection")
	}
	if sink.called {
		t.Fatalf("planless runtime write must not broadcast a WorkGraph invalidation")
	}
}

type cacheObservingSink struct {
	service        *Service
	executionID    string
	called         bool
	cachedAtNotify bool
}

func (s *cacheObservingSink) InvalidateExecution(context.Context, ExecutionInvalidation) {
	s.called = true
	s.service.viewCache.mu.Lock()
	_, s.cachedAtNotify = s.service.viewCache.entries[s.executionID]
	s.service.viewCache.mu.Unlock()
}
