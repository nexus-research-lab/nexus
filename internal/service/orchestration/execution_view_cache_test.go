package orchestration

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestExecutionViewCacheSharesOneComputeUntilInvalidated(t *testing.T) {
	var cache executionViewCache
	execution := &protocol.Execution{ID: "execution-1", OwnerUserID: "owner", SessionKey: "session", RootRoundID: "root"}
	var calls atomic.Int32
	release := make(chan struct{})
	compute := func(context.Context) (*protocol.ExecutionView, error) {
		calls.Add(1)
		<-release
		return &protocol.ExecutionView{ID: execution.ID}, nil
	}

	var group sync.WaitGroup
	results := make([]*protocol.ExecutionView, 8)
	for index := range results {
		group.Go(func() {
			view, err := cache.load(t.Context(), execution, compute)
			if err != nil {
				t.Errorf("load: %v", err)
			}
			results[index] = view
		})
	}
	for calls.Load() == 0 {
		// 等第一个读取方进入计算，其余读取方只能等待同一结果。
	}
	close(release)
	group.Wait()
	if calls.Load() != 1 {
		t.Fatalf("concurrent compute calls = %d, want 1", calls.Load())
	}
	for _, view := range results {
		if view != results[0] {
			t.Fatalf("readers did not share one projection")
		}
	}

	if _, err := cache.load(t.Context(), execution, compute); err != nil || calls.Load() != 1 {
		t.Fatalf("cached read recomputed: calls=%d err=%v", calls.Load(), err)
	}
	cache.invalidate(execution.ID)
	if _, err := cache.load(t.Context(), execution, compute); err != nil || calls.Load() != 2 {
		t.Fatalf("invalidated read did not recompute: calls=%d err=%v", calls.Load(), err)
	}
	cache.invalidateRootRound("owner", "session", "root")
	if _, err := cache.load(t.Context(), execution, compute); err != nil || calls.Load() != 3 {
		t.Fatalf("root round invalidation did not recompute: calls=%d err=%v", calls.Load(), err)
	}
	cache.invalidateRootRound("owner", "other-session", "root")
	if _, err := cache.load(t.Context(), execution, compute); err != nil || calls.Load() != 3 {
		t.Fatalf("foreign root round invalidation dropped cache: calls=%d", calls.Load())
	}
}

func TestExecutionViewCacheDropsInFlightResultInvalidatedBeforeCommit(t *testing.T) {
	var cache executionViewCache
	execution := &protocol.Execution{ID: "execution-1", OwnerUserID: "owner", SessionKey: "session"}
	started := make(chan struct{})
	release := make(chan struct{})
	stale := &protocol.ExecutionView{ID: "stale"}
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = cache.load(t.Context(), execution, func(context.Context) (*protocol.ExecutionView, error) {
			close(started)
			<-release
			return stale, nil
		})
	}()
	<-started
	// 写入在投影读取之后提交并失效：这次投影不得被后来的读取复用。
	cache.invalidate(execution.ID)
	close(release)
	<-done
	fresh := &protocol.ExecutionView{ID: "fresh"}
	view, err := cache.load(t.Context(), execution, func(context.Context) (*protocol.ExecutionView, error) {
		return fresh, nil
	})
	if err != nil || view != fresh {
		t.Fatalf("view = %#v err = %v, want fresh projection", view, err)
	}
}

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

func TestInvalidateExecutionDropsCachedViewBeforeNotifying(t *testing.T) {
	service := &Service{}
	execution := &protocol.Execution{ID: "execution-1", OwnerUserID: "owner", SessionKey: "session"}
	_, _ = service.viewCache.load(t.Context(), execution, func(context.Context) (*protocol.ExecutionView, error) {
		return &protocol.ExecutionView{}, nil
	})
	sink := &cacheObservingSink{service: service, executionID: execution.ID}
	service.SetExecutionInvalidationSink(sink)
	service.invalidateExecution(t.Context(), ExecutionInvalidation{OwnerUserID: "owner", SessionKey: "session", ExecutionID: execution.ID})
	if !sink.called || sink.cachedAtNotify {
		t.Fatalf("sink called=%v cached_at_notify=%v", sink.called, sink.cachedAtNotify)
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
