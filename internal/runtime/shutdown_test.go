// INPUT: 宿主退出、并发启动/回执落盘与后台任务。
// OUTPUT: 关闭准入、终态持久化顺序、可重复等待及失败保留的回归证据。
// POS: Manager 整体生命周期测试。
package runtime

import (
	"context"
	"errors"
	"testing"
	"time"

	bridge "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
)

func TestManagerShutdownRejectsNewWorkAndPreservesFailure(t *testing.T) {
	want := errors.New("runtime descendant cleanup failed")
	client := &fakeRuntimeClient{disconnectErr: want}
	manager := NewManagerWithFactory(&fakeRuntimeFactory{client: client})
	if _, err := manager.GetOrCreate(t.Context(), "session", bridge.Options{}); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := manager.Close(t.Context()); !errors.Is(err, want) {
			t.Fatalf("Close=%v, want retained cleanup error", err)
		}
	}
	if client.disconnectCalls != 1 {
		t.Fatalf("shutdown repeated side effects: %d disconnects", client.disconnectCalls)
	}
	for _, owner := range []string{"", "owner"} {
		if _, err := manager.BeginClientStartup(t.Context(), "new", owner); !errors.Is(err, ErrRuntimeManagerClosed) {
			t.Fatalf("startup owner=%q err=%v", owner, err)
		}
	}
	if manager.StartBackgroundTask("new", func(context.Context) { t.Error("late background task ran") }) {
		t.Fatal("closed manager accepted background task")
	}
	if err := manager.StartRound(t.Context(), "new", "round", nil); !errors.Is(err, ErrRuntimeManagerClosed) {
		t.Fatalf("closed manager accepted round: %v", err)
	}
}

func TestManagerShutdownWaitsForLateFactory(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	client := &fakeRuntimeClient{}
	manager := NewManagerWithFactory(runtimeFactoryFunc(func(bridge.Options) Client {
		close(entered)
		<-release
		return client
	}))
	started := make(chan error, 1)
	go func() { _, err := manager.GetOrCreate(t.Context(), "session", bridge.Options{}); started <- err }()
	<-entered
	short, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()
	if err := manager.Close(short); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Close returned before factory disposed: %v", err)
	}
	close(release)
	if err := <-started; !errors.Is(err, ErrRuntimeManagerClosed) {
		t.Fatalf("late factory installed client: %v", err)
	}
	if err := manager.Close(t.Context()); err != nil {
		t.Fatal(err)
	}
	if client.disconnectCalls != 1 || manager.SessionClient("session") != nil {
		t.Fatalf("late client survived: disconnects=%d", client.disconnectCalls)
	}
}
