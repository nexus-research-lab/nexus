package runtime

import (
	"context"
	"testing"
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
