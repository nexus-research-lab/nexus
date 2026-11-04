// INPUT: 有变化、无变化与短暂失败的目录版本源。
// OUTPUT: 合并通知、失败后的补偿通知以及取消时退出。
// POS: 跨进程目录观察生命周期回归。
package room

import (
	"context"
	"errors"
	"testing"
	"time"
)

type directoryRevisionFunc func(context.Context) (int64, error)

func (f directoryRevisionFunc) DirectoryRevision(ctx context.Context) (int64, error) { return f(ctx) }

func TestWatchDirectoryReconcilesChangesAndStops(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	notifications := make(chan struct{}, 8)
	failures := make(chan error, 1)
	done := make(chan struct{})
	reads := 0
	source := directoryRevisionFunc(func(context.Context) (int64, error) {
		reads++
		switch reads {
		case 1, 2:
			return 1, nil
		case 3:
			return 0, errors.New("temporarily unavailable")
		default:
			return 4, nil
		}
	})
	go func() {
		defer close(done)
		WatchDirectory(ctx, source, func(context.Context) { notifications <- struct{}{} }, func(err error) { failures <- err })
	}()
	for range 2 {
		select {
		case <-notifications:
		case <-time.After(5 * time.Second):
			t.Fatal("directory notification missing")
		}
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("watcher did not stop")
	}
	if len(notifications) != 0 {
		t.Fatal("unchanged version emitted notification")
	}
	if len(failures) != 1 {
		t.Fatal("read failure was not reported")
	}
}
