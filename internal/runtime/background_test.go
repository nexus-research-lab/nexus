package runtime

import (
	"context"
	"testing"
	"time"
)

func TestManagerBackgroundTaskRejectsCrossOwnerReuse(t *testing.T) {
	manager := NewManager()
	started := make(chan struct{})
	stopped := make(chan struct{})
	if !manager.StartBackgroundTaskForOwner(
		"room:group:conversation-1",
		"owner-a",
		func(ctx context.Context) {
			close(started)
			<-ctx.Done()
			close(stopped)
		},
	) {
		t.Fatal("首个 owner 后台任务未启动")
	}
	<-started

	crossOwnerRan := false
	if manager.StartBackgroundTaskForOwner(
		"room:group:conversation-1",
		"owner-b",
		func(context.Context) {
			crossOwnerRan = true
		},
	) {
		t.Fatal("同一 session 不应接受其他 owner 的后台任务")
	}
	if crossOwnerRan {
		t.Fatal("跨 owner 后台任务不应执行")
	}

	if _, err := manager.CloseOwnerSessions(context.Background(), "owner-a"); err != nil {
		t.Fatalf("关闭 owner 后台任务失败: %v", err)
	}
	select {
	case <-stopped:
	default:
		t.Fatal("关闭 owner 必须取消并等待无 client 的后台任务")
	}
}

func TestManagerCloseIdleSessionsWaitsForBackgroundOnlySession(t *testing.T) {
	now := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)
	manager := NewManager()
	manager.now = func() time.Time { return now }
	started := make(chan struct{})
	stopped := make(chan struct{})
	if !manager.StartBackgroundTaskForOwner(
		"room:group:conversation-idle",
		"owner-a",
		func(ctx context.Context) {
			close(started)
			<-ctx.Done()
			close(stopped)
		},
	) {
		t.Fatal("后台任务未启动")
	}
	<-started

	now = now.Add(11 * time.Minute)
	closed, err := manager.CloseIdleSessions(context.Background(), 10*time.Minute)
	if err != nil {
		t.Fatalf("回收后台 session 失败: %v", err)
	}
	if closed != 1 {
		t.Fatalf("回收数量=%d，want 1", closed)
	}
	select {
	case <-stopped:
	default:
		t.Fatal("idle 回收必须取消并等待无 client 的后台任务")
	}
}

func TestManagerClosingSessionRejectsLateBackgroundTask(t *testing.T) {
	manager := NewManager()
	sessionKey := "agent:main:ws:dm:closing-race"
	started := make(chan struct{})
	release := make(chan struct{})
	if !manager.StartBackgroundTaskForOwner(
		sessionKey,
		"owner-a",
		func(ctx context.Context) {
			close(started)
			<-release
			<-ctx.Done()
		},
	) {
		t.Fatal("首个后台任务未启动")
	}
	<-started

	closeDone := make(chan struct{})
	go func() {
		_ = manager.CloseSession(context.Background(), sessionKey)
		close(closeDone)
	}()

	deadline := time.After(time.Second)
	for {
		manager.mu.RLock()
		state := manager.sessions[sessionKey]
		closing := state != nil && state.Closing
		manager.mu.RUnlock()
		if closing {
			break
		}
		select {
		case <-deadline:
			t.Fatal("session 未进入 closing 状态")
		case <-time.After(time.Millisecond):
		}
	}

	ranLate := false
	if manager.StartBackgroundTaskForOwner(
		sessionKey,
		"owner-a",
		func(context.Context) {
			ranLate = true
		},
	) {
		t.Fatal("closing session 不应接受迟到后台任务")
	}
	if ranLate {
		t.Fatal("迟到后台任务不应执行")
	}

	close(release)
	select {
	case <-closeDone:
	case <-time.After(time.Second):
		t.Fatal("CloseSession 未完成")
	}
	if manager.HasSession(sessionKey) {
		t.Fatal("关闭完成后 session 不应继续可见")
	}
}

func TestManagerCloseSessionWaitsForTerminalRoundFinalizer(t *testing.T) {
	manager := NewManager()
	sessionKey := "agent:main:ws:dm:terminal-finalizer-race"
	roundID := "round-terminal"
	if !manager.StartRound(sessionKey, roundID, nil) {
		t.Fatal("round 未启动")
	}
	manager.MarkRoundTerminal(sessionKey, roundID)
	if running := manager.GetRunningRoundIDs(sessionKey); len(running) != 0 {
		t.Fatalf("终态 round 不应继续占用运行态: %+v", running)
	}

	closeResult := make(chan error, 1)
	go func() {
		closeResult <- manager.CloseSession(context.Background(), sessionKey)
	}()

	deadline := time.After(time.Second)
	for {
		manager.mu.RLock()
		state := manager.sessions[sessionKey]
		closing := state != nil && state.Closing
		manager.mu.RUnlock()
		if closing {
			break
		}
		select {
		case err := <-closeResult:
			t.Fatalf("round 收尾前 CloseSession 不应返回: %v", err)
		case <-deadline:
			t.Fatal("session 未进入 closing 状态")
		case <-time.After(time.Millisecond):
		}
	}

	if manager.StartBackgroundTaskForOwner(
		sessionKey,
		"owner-a",
		func(context.Context) {},
	) {
		t.Fatal("terminal round 收尾期间不应接受迟到后台任务")
	}

	manager.MarkRoundFinished(sessionKey, roundID)
	select {
	case err := <-closeResult:
		if err != nil {
			t.Fatalf("CloseSession() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("round 收尾完成后 CloseSession 未返回")
	}
}

func TestManagerCloseSessionRetainsStateUntilBackgroundTaskStops(t *testing.T) {
	manager := NewManager()
	sessionKey := "room:group:conversation-close-timeout"
	started := make(chan struct{})
	release := make(chan struct{})
	stopped := make(chan struct{})
	if !manager.StartBackgroundTaskForOwner(
		sessionKey,
		"owner-a",
		func(context.Context) {
			close(started)
			<-release
			close(stopped)
		},
	) {
		t.Fatal("后台任务未启动")
	}
	<-started

	closeCtx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	if err := manager.CloseSession(closeCtx, sessionKey); err == nil {
		t.Fatal("后台任务未结束时 CloseSession 应报告超时")
	}

	manager.mu.RLock()
	state := manager.sessions[sessionKey]
	stillClosing := state != nil && state.Closing
	manager.mu.RUnlock()
	if !stillClosing {
		t.Fatal("后台任务未结束时 session 不应从 manager 中移除")
	}

	close(release)
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("后台任务未停止")
	}
	deadline := time.After(time.Second)
	for {
		manager.mu.RLock()
		_, exists := manager.sessions[sessionKey]
		manager.mu.RUnlock()
		if !exists {
			return
		}
		select {
		case <-deadline:
			t.Fatal("后台任务结束后 session 仍未移除")
		case <-time.After(time.Millisecond):
		}
	}
}
