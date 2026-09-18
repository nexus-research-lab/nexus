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

func TestAgentClientCleanupReleasesHostScratchOnlyAfterBridgeClose(t *testing.T) {
	lease, err := Acquire(t.Context(), Input{
		OwnerUserID: "cleanup-owner",
		SessionKey:  "session-a",
		Root:        t.TempDir(),
	})
	if err != nil {
		t.Fatal(err)
	}
	path := lease.Path()
	client := &agentClient{closeSession: func(*bridge.Session) error { return nil }}
	cleanup := &agentClientSessionCleanup{done: make(chan struct{}), scratchRoot: path}
	client.startBridgeSessionCleanup(nil, nil, cleanup)
	select {
	case <-cleanup.done:
	case <-time.After(time.Second):
		t.Fatal("scratch cleanup did not finish")
	}
	if cleanup.err != nil {
		t.Fatalf("cleanup error = %v", cleanup.err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("scratch remains after successful bridge close: %v", err)
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
	cleanup := &agentClientSessionCleanup{done: make(chan struct{}), scratchRoot: path}
	client.startBridgeSessionCleanup(nil, nil, cleanup)
	<-cleanup.done
	if !errors.Is(cleanup.err, want) {
		t.Fatalf("cleanup error = %v", cleanup.err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("scratch removed after failed bridge close: %v", err)
	}
	if err := lease.Release(); err != nil {
		t.Fatal(err)
	}
}
