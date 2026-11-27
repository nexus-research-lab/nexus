// room_internal_test.go 覆盖 Room 内部 Agent 解析与目录监听测试。
package room

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/storage/roomrepo"
)

func TestResolveAgentWorkspacePathRejectsRoomOwnerMismatch(t *testing.T) {
	ctx := authctx.WithPrincipal(context.Background(), &authctx.Principal{
		UserID: "user-a",
	})

	_, err := (&Service{}).resolveAgentWorkspacePath(ctx, "user-b", "agent-a")
	if err == nil || !strings.Contains(err.Error(), "Room owner 与调用上下文不一致") {
		t.Fatalf("跨 owner 的 Room 清理应在查询 Agent 前拒绝: %v", err)
	}
}

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
