// INPUT: 有变化、无变化、已被即时通知覆盖与短暂失败的目录版本源。
// OUTPUT: 启动基线不通知、覆盖版本不重复通知、Agent 变化单独标注、失败后继续核对以及取消时退出。
// POS: 跨进程目录观察生命周期回归。
package room

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/storage/roomrepo"
)

type directoryVersionFunc func(context.Context) (roomrepo.DirectoryVersion, error)

func (f directoryVersionFunc) DirectoryVersion(ctx context.Context) (roomrepo.DirectoryVersion, error) {
	return f(ctx)
}

func TestWatchDirectoryNotifiesOnlyUncoveredChanges(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	notifications := make(chan DirectoryChange, 8)
	failures := make(chan error, 4)
	done := make(chan struct{})
	coverage := &DirectoryCoverage{}
	reads := 0
	source := directoryVersionFunc(func(context.Context) (roomrepo.DirectoryVersion, error) {
		reads++
		switch reads {
		case 1:
			return roomrepo.DirectoryVersion{Directory: 1}, nil
		case 2:
			return roomrepo.DirectoryVersion{Directory: 2}, nil
		case 3:
			// 宿主自身 mutation 的即时通知已覆盖版本 3。
			coverage.Cover(roomrepo.DirectoryVersion{Directory: 3}, false)
			return roomrepo.DirectoryVersion{}, errors.New("temporarily unavailable")
		case 4:
			return roomrepo.DirectoryVersion{Directory: 3}, nil
		default:
			return roomrepo.DirectoryVersion{Directory: 4, Agent: 1}, nil
		}
	})
	go func() {
		defer close(done)
		WatchDirectory(ctx, source, coverage, func(_ context.Context, change DirectoryChange) {
			notifications <- change
		}, func(err error) { failures <- err })
	}()
	next := func() DirectoryChange {
		t.Helper()
		select {
		case change := <-notifications:
			return change
		case <-time.After(5 * time.Second):
			t.Fatal("directory notification missing")
		}
		return DirectoryChange{}
	}
	if change := next(); change.Version.Directory != 2 || change.Agents {
		t.Fatalf("first uncovered change = %+v", change)
	}
	if change := next(); change.Version.Directory != 4 || !change.Agents {
		t.Fatalf("agent change = %+v", change)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("watcher did not stop")
	}
	if len(notifications) != 0 {
		t.Fatal("baseline, covered or unchanged version emitted notification")
	}
	if len(failures) != 1 {
		t.Fatal("read failure was not reported")
	}
}

func TestDirectoryCoverageSeparatesAgentInvalidation(t *testing.T) {
	coverage := &DirectoryCoverage{}
	previous := roomrepo.DirectoryVersion{Directory: 1, Agent: 1}
	current := roomrepo.DirectoryVersion{Directory: 2, Agent: 2}
	// 非 agent_ 即时通知只重读目录，Agent 变化仍需观察器补发。
	coverage.Cover(current, false)
	if change, ok := coverage.uncovered(previous, current); !ok || !change.Agents {
		t.Fatalf("agent change hidden by directory-only notification: %+v %v", change, ok)
	}
	coverage.Cover(current, true)
	if _, ok := coverage.uncovered(previous, current); ok {
		t.Fatal("covered change was notified again")
	}
}
