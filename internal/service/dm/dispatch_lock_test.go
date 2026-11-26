package dm

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestDispatchLocksSerializeOneSessionOnly(t *testing.T) {
	var locks dispatchLocks
	ctx := context.Background()
	unlockA, err := locks.lock(ctx, "session-a")
	if err != nil {
		t.Fatal(err)
	}
	otherDone := make(chan struct{})
	go func() {
		unlock, lockErr := locks.lock(ctx, "session-b")
		if lockErr == nil {
			unlock()
		}
		close(otherDone)
	}()
	select {
	case <-otherDone:
	case <-time.After(time.Second):
		t.Fatal("其他会话被 session-a 的锁挡住")
	}
	sameDone := make(chan struct{})
	go func() {
		unlock, lockErr := locks.lock(ctx, " session-a ")
		if lockErr == nil {
			unlock()
		}
		close(sameDone)
	}()
	select {
	case <-sameDone:
		t.Fatal("同一会话（含首尾空白的同一 key）没有串行")
	case <-time.After(50 * time.Millisecond):
	}
	unlockA()
	unlockA()
	select {
	case <-sameDone:
	case <-time.After(time.Second):
		t.Fatal("释放后等待者没有拿到锁")
	}
	if n := locks.size(); n != 0 {
		t.Fatalf("释放后锁表残留 %d 个会话", n)
	}
}

func TestDispatchLocksWaitHonoursContext(t *testing.T) {
	var locks dispatchLocks
	unlock, err := locks.lock(context.Background(), "session-a")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err = locks.lock(ctx, "session-a"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("等待应随 ctx 超时放弃: %v", err)
	}
	unlock()
	if n := locks.size(); n != 0 {
		t.Fatalf("放弃等待与释放后锁表残留 %d 个会话", n)
	}
	cancelled, stop := context.WithCancel(context.Background())
	stop()
	if _, err = locks.lock(cancelled, "session-a"); !errors.Is(err, context.Canceled) {
		t.Fatalf("已取消的 ctx 不应取锁: %v", err)
	}
}

func TestMainWindowStartsWhileAnotherSessionHoldsDispatchLock(t *testing.T) {
	f := newMainInboxFixture(t, "dispatch-lock-independent")
	unlock, err := f.service.dispatchLocks.lock(context.Background(), "agent:nexus:ws:dm:some-other-session")
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	f.send(t, "不受其他会话影响", "client-1")
	f.waitPrompts(t, 1)
	f.finishRound("dispatch-lock-independent")
}

// size 返回仍在使用中的会话数，供测试确认释放后不残留。
func (l *dispatchLocks) size() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.entries)
}
