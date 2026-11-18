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
	"testing/synctest"
	"time"

	bridge "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

func TestResourceAdmissionBoundsStartupQueue(t *testing.T) {
	m := NewManagerWithFactory(runtimeFactoryFunc(func(bridge.Options) Client { return &fakeRuntimeClient{} }))
	m.SetResourceLimits(ResourceLimits{MaxSessions: 1})
	client, err := m.GetOrCreate(t.Context(), "rollback", bridge.Options{})
	if err != nil {
		t.Fatal(err)
	}
	lease, ok := m.CaptureClientLease("rollback", client)
	if !ok {
		t.Fatal("missing rollback lease")
	}
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
	if closed, err := m.CloseSessionIfLease(t.Context(), lease); !closed || err != nil {
		t.Fatalf("queued startups blocked rollback: %t, %v", closed, err)
	}
	startups[0].Close()
	startup, err := m.BeginClientStartup(t.Context(), "after-release", "")
	if err != nil {
		t.Fatal(err)
	}
	startup.Close()
}

// 同 key 排队与等待全局名额共享一次等待预算。
func TestResourceAdmissionWaitBudgetIncludesSameSessionQueue(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		m := NewManagerWithFactory(runtimeFactoryFunc(func(bridge.Options) Client { return &fakeRuntimeClient{} }))
		m.SetResourceLimits(ResourceLimits{MaxSessions: 1})
		if _, err := m.GetOrCreate(t.Context(), "occupied", bridge.Options{}); err != nil {
			t.Fatal(err)
		}
		if err := m.StartRound(t.Context(), "occupied", "r", nil); err != nil {
			t.Fatal(err)
		}
		started := time.Now()
		done := make(chan error, 3)
		for range cap(done) {
			go func() { _, err := m.GetOrCreate(t.Context(), "same", bridge.Options{}); done <- err }()
		}
		for range cap(done) {
			if err := <-done; !errors.Is(err, ErrRuntimeResourceBusy) || !errors.Is(err, context.DeadlineExceeded) {
				t.Fatal(err)
			}
		}
		if elapsed := time.Since(started); elapsed != runtimeResourceWaitTimeout {
			t.Fatalf("three concurrent requests waited %s", elapsed)
		}
	})
}

type followupReceiptClient struct {
	fakeRuntimeClient
	deliver func() (sdkprotocol.TaskRunState, error)
}

func (c *followupReceiptClient) SendTaskMessageWithState(context.Context, string, string, string) (sdkprotocol.TaskRunState, error) {
	return c.deliver()
}

// 回执与消息流可以乱序；续跑投递前占位，旧终态和迟到的 running 回执不能覆盖新状态。
func TestResourceAdmissionFollowupSurvivesEvictionAndReleasesOnTerminal(t *testing.T) {
	client := &followupReceiptClient{}
	m := NewManagerWithFactory(runtimeFactoryFunc(func(bridge.Options) Client { return client }))
	m.SetResourceLimits(ResourceLimits{MaxSubagents: 1})
	if _, err := m.GetOrCreate(t.Context(), "parent", bridge.Options{}); err != nil {
		t.Fatal(err)
	}
	update := func(gen int, status string) {
		m.ObserveSubagentRun("parent", client, sdkprotocol.ReceivedMessage{TaskUpdated: &sdkprotocol.TaskUpdatedMessage{TaskID: "child", RunGeneration: gen, Patch: sdkprotocol.TaskUpdatedPatch{Status: status}, Additional: map[string]any{"task_type": "local_agent"}}})
	}
	update(1, "completed")
	count := func() int { m.mu.RLock(); defer m.mu.RUnlock(); return activeSubagentBindings(m.sessions["parent"]) }
	client.deliver = func() (sdkprotocol.TaskRunState, error) {
		if closed, err := m.closeIdleSessions(t.Context(), 0, 1); closed != 0 || err != nil {
			t.Fatalf("evicted during delivery: %d, %v", closed, err)
		}
		if err := m.SendTaskMessage(t.Context(), "parent", "other", "message", ""); !errors.Is(err, ErrRuntimeResourceBusy) {
			t.Fatalf("second child admitted: %v", err)
		}
		return sdkprotocol.TaskRunState{Generation: 2, Status: "running"}, nil
	}
	if err := m.SendTaskMessage(t.Context(), "parent", "child", "resume", ""); err != nil {
		t.Fatal(err)
	}
	update(1, "completed")
	if got := count(); got != 1 {
		t.Fatalf("late previous completion released resumed run: %d", got)
	}
	client.deliver = func() (sdkprotocol.TaskRunState, error) {
		update(2, "completed")
		return sdkprotocol.TaskRunState{Generation: 2, Status: "running"}, nil
	}
	if err := m.SendTaskMessage(t.Context(), "parent", "child", "continue", ""); err != nil {
		t.Fatal(err)
	}
	if got := count(); got != 0 {
		t.Fatalf("late receipt resurrected completed run: %d", got)
	}
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
	now := time.Now()
	m.now = func() time.Time { return now }
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
	now = now.Add(time.Second)
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
