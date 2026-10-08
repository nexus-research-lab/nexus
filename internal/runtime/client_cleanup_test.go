// INPUT: 已结束但失败的 runtime 清理与并发重连。
// OUTPUT: 错误保持为启动栅栏，配置重试不能偷偷创建替代进程。
// POS: 宿主执行资源生命周期回归。
package runtime

import (
	"context"
	"errors"
	"io"
	"os"
	"sync/atomic"
	"testing"
	"time"

	bridge "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
)

// TestAgentClientCleanupFailureBlocksReconnect 不将完成清理尝试误认为清理成功。
func TestAgentClientCleanupFailureBlocksReconnect(t *testing.T) {
	want := errors.New("descendants remain")
	cleanup := &agentClientSessionCleanup{done: closedSignal(), err: want}
	var opened atomic.Int32
	client := &agentClient{cleanup: cleanup, newSession: func(context.Context, bridge.Options) (*bridge.Session, error) {
		opened.Add(1)
		return nil, errors.New("unexpected startup")
	}}
	for i := 0; i < 2; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		err := client.Connect(ctx)
		cancel()
		if !errors.Is(err, want) {
			t.Errorf("Connect %d lost cleanup failure: %v", i, err)
		}
	}
	if opened.Load() != 0 || client.cleanup != cleanup {
		t.Fatal("failed cleanup allowed new startup or discarded evidence")
	}
}

// TestCleanupFailureIsNotAnOrdinaryClosedTransport 防止批量回收把 joined 断管错误整体忽略。
func TestCleanupFailureIsNotAnOrdinaryClosedTransport(t *testing.T) {
	want := &bridge.ProcessCleanupError{SessionID: 9000, Err: io.ErrClosedPipe}
	for _, err := range []error{want, errors.Join(bridge.ErrNotConnected, want)} {
		if IsRuntimeTransportClosedError(err) {
			t.Fatalf("cleanup failure classified as ordinary transport closure: %v", err)
		}
	}
	if !IsRuntimeTransportClosedError(io.ErrClosedPipe) {
		t.Fatal("ordinary closed-pipe classification changed")
	}
}

// TestManagerBulkCleanupReportsAndRetainsFailure 批量关闭仍报告清理错误并保留原 Session 栅栏。
func TestManagerBulkCleanupReportsAndRetainsFailure(t *testing.T) {
	for _, entry := range []string{"owner", "idle", "agent_revocation"} {
		t.Run(entry, func(t *testing.T) {
			want := &bridge.ProcessCleanupError{SessionID: 9000, Err: io.ErrClosedPipe}
			client := &fakeRuntimeClient{disconnectFn: func(context.Context) error {
				return errors.Join(bridge.ErrNotConnected, want)
			}}
			manager := NewManager()
			state := &sessionState{Client: client, OwnerUserID: "cleanup-owner", AgentID: "cleanup-agent", LastUsedAt: time.Now().Add(-time.Hour)}
			manager.sessions["cleanup-failed"] = state
			closeAll := func() (int, error) {
				switch entry {
				case "owner":
					return manager.CloseOwnerSessions(t.Context(), state.OwnerUserID)
				case "idle":
					return manager.CloseIdleSessions(t.Context(), time.Minute)
				default:
					return manager.RevokeAgentSessions(t.Context(), state.OwnerUserID, state.AgentID)
				}
			}
			if _, err := closeAll(); !errors.Is(err, want) {
				t.Fatalf("bulk close lost cleanup error: %v", err)
			}
			if err := manager.CloseSession(t.Context(), "cleanup-failed"); !errors.Is(err, want) {
				t.Fatalf("exact session lost cleanup error: %v", err)
			}
			if entry != "idle" {
				if _, err := closeAll(); !errors.Is(err, want) {
					t.Fatalf("repeated bulk close lost cleanup error: %v", err)
				}
			}
			manager.mu.RLock()
			defer manager.mu.RUnlock()
			if manager.sessions["cleanup-failed"] != state || !state.Closing {
				t.Fatal("bulk cleanup failure released session fence")
			}
		})
	}
}

// TestAgentClientStaleStartupCleanupFailureStopsRetry 启动中配置换代后，旧产物清理失败不得重试。
func TestAgentClientStaleStartupCleanupFailureStopsRetry(t *testing.T) {
	want := errors.New("stale runtime cleanup failed")
	started, release := make(chan struct{}), make(chan struct{})
	var opened atomic.Int32
	client := &agentClient{options: bridge.Options{Model: "old"}, newSession: func(context.Context, bridge.Options) (*bridge.Session, error) {
		if opened.Add(1) == 1 {
			close(started)
			<-release
			return &bridge.Session{}, nil
		}
		return nil, errors.New("unexpected retry")
	}, closeSession: func(*bridge.Session) error { return want }}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- client.Connect(ctx) }()
	<-started
	if err := client.Reconfigure(ctx, bridge.Options{Model: "new"}); err != nil {
		t.Fatal(err)
	}
	close(release)
	select {
	case err := <-done:
		if !errors.Is(err, want) {
			t.Errorf("stale cleanup error lost: %v", err)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	if opened.Load() != 1 || client.cleanup == nil {
		t.Fatal("stale process cleanup failure permitted retry")
	}
}

// TestManagerCleanupFailureRetainsSessionFence 同步失败和超时后失败都保留原状态与精确关闭结果。
func TestManagerCleanupFailureRetainsSessionFence(t *testing.T) {
	for _, delayed := range []bool{false, true} {
		t.Run(map[bool]string{false: "synchronous", true: "after_timeout"}[delayed], func(t *testing.T) {
			want := errors.New("runtime descendants unconfirmed")
			var calls atomic.Int32
			client := &fakeRuntimeClient{disconnectFn: func(context.Context) error {
				if delayed && calls.Add(1) == 1 {
					return context.DeadlineExceeded
				}
				return want
			}}
			manager := NewManager()
			state := &sessionState{Client: client}
			manager.sessions["cleanup-failed"] = state
			if err := manager.CloseSession(context.Background(), "cleanup-failed"); err == nil {
				t.Fatal("close failure not reported")
			}
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			if err := manager.CloseSession(ctx, "cleanup-failed"); !errors.Is(err, want) {
				t.Fatalf("repeat close lost failure: %v", err)
			}
			if _, err := manager.GetOrCreate(ctx, "cleanup-failed", bridge.Options{}); !errors.Is(err, ErrRuntimeSessionClosing) {
				t.Fatalf("failed cleanup allowed replacement: %v", err)
			}
			manager.mu.RLock()
			defer manager.mu.RUnlock()
			if manager.sessions["cleanup-failed"] != state || !errors.Is(state.CloseError, want) {
				t.Fatal("cleanup failure evidence removed")
			}
		})
	}
}

func TestAgentClientCleanupFailureRetainsHostScratchLease(t *testing.T) {
	lease, err := Acquire(t.Context(), Input{
		OwnerUserID: "cleanup-owner",
		SessionKey:  "session-b",
		Root:        t.TempDir(),
	})
	if err != nil {
		t.Fatal(err)
	}
	path := lease.Path()
	want := errors.New("descendants remain")
	client := &agentClient{closeSession: func(*bridge.Session) error { return want }}
	cleanup := &agentClientSessionCleanup{done: make(chan struct{}), scratchLease: lease}
	client.startBridgeSessionCleanup(nil, nil, cleanup)
	<-cleanup.done
	if err := cleanup.getErr(); !errors.Is(err, want) {
		t.Fatalf("cleanup error = %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("scratch removed after failed bridge close: %v", err)
	}
	if err := lease.Release(); err != nil {
		t.Fatal(err)
	}
}

func TestAgentClientCleanupUsesExactScratchHandle(t *testing.T) {
	root := t.TempDir()
	first, err := Acquire(t.Context(), Input{OwnerUserID: "cleanup-owner", SessionKey: "session-shared", Root: root})
	if err != nil {
		t.Fatal(err)
	}
	second, err := Acquire(t.Context(), Input{OwnerUserID: "cleanup-owner", SessionKey: "session-shared", Root: root})
	if err != nil {
		t.Fatal(err)
	}
	path := first.Path()
	client := &agentClient{closeSession: func(*bridge.Session) error { return nil }}
	cleanup := &agentClientSessionCleanup{done: make(chan struct{}), scratchLease: first}
	client.startBridgeSessionCleanup(nil, nil, cleanup)
	<-cleanup.done
	if err := cleanup.getErr(); err != nil {
		t.Fatalf("cleanup error = %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("old runtime cleanup removed scratch still held by preparation: %v", err)
	}
	if err := second.Release(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("last exact scratch handle did not remove resource: %v", err)
	}
}

func TestClientStartupBindFailureDoesNotConsumeLease(t *testing.T) {
	lease, err := Acquire(t.Context(), Input{
		OwnerUserID: "bind-owner",
		SessionKey:  "bind-failure",
		Root:        t.TempDir(),
	})
	if err != nil {
		t.Fatal(err)
	}
	path := lease.Path()
	client := &agentClient{retired: true}
	startup := &ClientStartup{
		expectedClient: client,
		release:        func() {},
	}
	consumed, bindErr := startup.BindSandboxLease(lease)
	if consumed || !errors.Is(bindErr, bridge.ErrAborted) {
		t.Fatalf("BindSandboxLease() = consumed=%v err=%v, want unconsumed aborted lease", consumed, bindErr)
	}
	if err := lease.Release(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("unconsumed lease was not released by caller: %v", err)
	}
}

func TestClientStartupBindLeaseFailsClosedForUnsupportedClient(t *testing.T) {
	lease, err := Acquire(t.Context(), Input{
		OwnerUserID: "bind-owner",
		SessionKey:  "unsupported-client",
		Root:        t.TempDir(),
	})
	if err != nil {
		t.Fatal(err)
	}
	path := lease.Path()
	startup := &ClientStartup{
		expectedClient: &fakeRuntimeClient{},
		release:        func() {},
	}
	consumed, bindErr := startup.BindSandboxLease(lease)
	if consumed || bindErr == nil {
		t.Fatalf("BindSandboxLease() = consumed=%v err=%v, want an unconsumed fail-closed lease", consumed, bindErr)
	}
	if err := lease.Release(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("unconsumed lease was not released by caller: %v", err)
	}
}

func TestAgentClientBindSandboxLeaseRejectsClaudeRuntime(t *testing.T) {
	lease, err := Acquire(t.Context(), Input{
		OwnerUserID: "bind-owner",
		SessionKey:  "claude-client",
		Root:        t.TempDir(),
	})
	if err != nil {
		t.Fatal(err)
	}
	path := lease.Path()
	client := &agentClient{options: bridge.Options{
		Runtime: bridge.RuntimeOptions{Kind: bridge.RuntimeClaude},
	}}
	if err := client.BindSandboxLease(lease); err == nil {
		t.Fatal("Claude runtime accepted a Nexus sandbox lease")
	}
	if err := lease.Release(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("rejected lease was not released by caller: %v", err)
	}
}

func TestDiscardUncleanSessionCleansLeaseWithoutInstalledSession(t *testing.T) {
	lease, err := Acquire(t.Context(), Input{
		OwnerUserID: "discard-owner",
		SessionKey:  "discard-without-session",
		Root:        t.TempDir(),
	})
	if err != nil {
		t.Fatal(err)
	}
	path := lease.Path()
	client := &agentClient{sandboxLease: lease}
	client.DiscardUncleanSession()
	client.mu.Lock()
	cleanup := client.cleanup
	client.mu.Unlock()
	if cleanup == nil {
		t.Fatal("discard did not create cleanup fence for the bound lease")
	}
	select {
	case <-cleanup.done:
	case <-time.After(time.Second):
		t.Fatal("discard cleanup did not finish")
	}
	if err := cleanup.getErr(); err != nil {
		t.Fatalf("discard cleanup error = %v", err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("bound lease remained after discard without session: %v", err)
	}
}
