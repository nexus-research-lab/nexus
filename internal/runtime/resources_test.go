// INPUT: 并发启动、资源不足、未完成工作及清理失败。
// OUTPUT: 准入不超卖、不误回收、不阻塞退出的回归证据。
// POS: Manager 资源保护的关键生命周期测试。
package runtime

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	bridge "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
)

func TestResourceAdmissionBoundsStartupQueue(t *testing.T) {
	m := NewManager()
	m.SetResourceLimits(ResourceLimits{MaxSessions: 1})
	var startups []*ClientStartup
	defer func() {
		for _, startup := range startups {
			startup.Close()
		}
	}()
	for i := range runtimeMaxPendingStartups {
		startup, err := m.BeginClientStartup(t.Context(), strconv.Itoa(i), "")
		if err != nil {
			t.Fatal(err)
		}
		startups = append(startups, startup)
	}
	if _, err := m.BeginClientStartup(t.Context(), "overflow", ""); !errors.Is(err, ErrRuntimeResourceBusy) {
		t.Fatalf("unbounded startup queue: %v", err)
	}
	startups[0].Close()
	startup, err := m.BeginClientStartup(t.Context(), "after-release", "")
	if err != nil {
		t.Fatal(err)
	}
	startup.Close()
}

func TestResourceAdmissionReservesBeforeFactoryAndReleasesFailure(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	m := NewManagerWithFactory(runtimeFactoryFunc(func(bridge.Options) Client {
		if calls.Add(1) == 1 {
			close(entered)
			<-release
			return nil
		}
		return &fakeRuntimeClient{}
	}))
	m.SetResourceLimits(ResourceLimits{MaxSessions: 1})
	first := make(chan error, 1)
	go func() { _, err := m.GetOrCreate(t.Context(), "first", bridge.Options{}); first <- err }()
	<-entered
	var releaseOnce sync.Once
	defer releaseOnce.Do(func() { close(release) })
	ctx, cancel := context.WithTimeout(t.Context(), 40*time.Millisecond)
	defer cancel()
	if _, err := m.GetOrCreate(ctx, "second", bridge.Options{}); !errors.Is(err, ErrRuntimeResourceBusy) || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("pending factory was oversubscribed or cancellation lost: %v", err)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("factory calls = %d, want 1", got)
	}
	releaseOnce.Do(func() { close(release) })
	if err := <-first; !errors.Is(err, bridge.ErrNotConnected) {
		t.Fatalf("first startup = %v", err)
	}
	if _, err := m.GetOrCreate(t.Context(), "third", bridge.Options{}); err != nil {
		t.Fatalf("failed startup retained reservation: %v", err)
	}
}

func TestResourceAdmissionReclaimsOnlyCompletedWork(t *testing.T) {
	m := NewManagerWithFactory(runtimeFactoryFunc(func(bridge.Options) Client { return &fakeRuntimeClient{} }))
	for _, key := range []string{"idle", "finishing", "background", "child"} {
		if _, err := m.GetOrCreate(t.Context(), key, bridge.Options{}); err != nil {
			t.Fatal(err)
		}
	}
	if err := m.StartRound(t.Context(), "finishing", "round", nil); err != nil {
		t.Fatal(err)
	}
	m.MarkRoundTerminal("finishing", "round")
	backgroundDone := make(chan struct{})
	if !m.StartBackgroundTask("background", func(context.Context) { <-backgroundDone }) {
		t.Fatal("background task rejected")
	}
	defer func() { close(backgroundDone); _ = m.WaitBackgroundTasks(t.Context(), "background") }()
	m.mu.Lock()
	m.sessions["child"].SubagentHookBindings["tool"] = subagentHookBinding{Started: true}
	for _, state := range m.sessions {
		state.LastUsedAt = time.Now().Add(-time.Hour)
	}
	m.mu.Unlock()
	m.SetResourceLimits(ResourceLimits{MaxSessions: 4})
	if _, err := m.GetOrCreate(t.Context(), "new", bridge.Options{}); err != nil {
		t.Fatal(err)
	}
	if m.SessionClient("idle") != nil {
		t.Fatal("idle client was not reclaimed")
	}
	if closed, err := m.CloseIdleSessions(t.Context(), time.Minute); err != nil || closed != 0 {
		t.Fatalf("TTL cleanup closed unfinished work: closed=%d err=%v", closed, err)
	}
	for _, key := range []string{"finishing", "background", "child"} {
		if m.SessionClient(key) == nil {
			t.Fatalf("unfinished %s was reclaimed", key)
		}
	}
	m.MarkRoundFinished("finishing", "round")
	if _, err := m.GetOrCreate(t.Context(), "next", bridge.Options{}); err != nil {
		t.Fatal(err)
	}
	// finishing 刚刚完成，LRU 应先回收没有工作的 new，而非后台任务。
	if m.SessionClient("new") != nil || m.SessionClient("background") == nil {
		t.Fatal("pressure cleanup did not follow idle LRU order")
	}
}

func TestResourceAdmissionRetainsFailedCleanupSlot(t *testing.T) {
	want := errors.New("descendant cleanup unknown")
	client := &fakeRuntimeClient{disconnectErr: want}
	var calls int
	m := NewManagerWithFactory(runtimeFactoryFunc(func(bridge.Options) Client { calls++; return client }))
	m.SetResourceLimits(ResourceLimits{MaxSessions: 1})
	if _, err := m.GetOrCreate(t.Context(), "old", bridge.Options{}); err != nil {
		t.Fatal(err)
	}
	if _, err := m.GetOrCreate(t.Context(), "new", bridge.Options{}); !errors.Is(err, want) {
		t.Fatalf("cleanup failure lost: %v", err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 40*time.Millisecond)
	defer cancel()
	if _, err := m.GetOrCreate(ctx, "retry", bridge.Options{}); !errors.Is(err, ErrRuntimeResourceBusy) {
		t.Fatalf("cleanup uncertainty released capacity: %v", err)
	}
	m.mu.RLock()
	state := m.sessions["old"]
	retained := state != nil && state.Client == client && state.Closing && errors.Is(state.CloseError, want)
	m.mu.RUnlock()
	if calls != 1 || client.disconnectCalls != 1 || !retained {
		t.Fatalf("unknown cleanup was retried or forgotten: factory=%d disconnect=%d", calls, client.disconnectCalls)
	}
}

func TestResourceAdmissionMemoryWaitRecoversOrStopsOnShutdown(t *testing.T) {
	for _, shutdown := range []bool{false, true} {
		name := "memory_recovers"
		if shutdown {
			name = "shutdown"
		}
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
			defer cancel()
			var available atomic.Bool
			var calls atomic.Int32
			entered := make(chan struct{})
			var once sync.Once
			m := NewManagerWithFactory(runtimeFactoryFunc(func(bridge.Options) Client { calls.Add(1); return &fakeRuntimeClient{} }))
			m.SetResourceLimits(ResourceLimits{MaxSessions: 1, MemoryAvailable: func() (bool, error) {
				once.Do(func() { close(entered) })
				return available.Load(), nil
			}})
			done := make(chan error, 1)
			go func() { _, err := m.GetOrCreate(ctx, "waiting", bridge.Options{}); done <- err }()
			<-entered
			if calls.Load() != 0 {
				t.Fatal("factory ran under memory pressure")
			}
			if shutdown {
				if err := m.Close(ctx); err != nil {
					t.Fatal(err)
				}
				if err := <-done; !errors.Is(err, ErrRuntimeManagerClosed) {
					t.Fatalf("waiting startup survived shutdown: %v", err)
				}
				return
			}
			available.Store(true)
			if err := <-done; err != nil {
				t.Fatal(err)
			}
			if calls.Load() != 1 {
				t.Fatal("factory not admitted after memory recovered")
			}
		})
	}
}
