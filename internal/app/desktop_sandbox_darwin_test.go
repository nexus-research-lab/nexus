//go:build darwin

// INPUT: 分页恢复结果、持锁装配和资源关闭事件。
// OUTPUT: 两阶段顺序、错误保留、按阶段记录失败项及无锁拒绝。
// POS: App 启动装配回归，原生收口另有集成测试。
package app

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
)

type desktopRecoveryFixture struct {
	calls   []string
	failure error
	stuck   bool
}

func (f *desktopRecoveryFixture) RecoverPendingSandboxProcesses(_ context.Context, _ runtimectx.SandboxProcessRecoveryOwnership, cursor string, limit int) (runtimectx.SandboxProcessRecoveryBatch, error) {
	f.calls = append(f.calls, "process:"+cursor)
	if cursor == "" {
		next := "a"
		if f.stuck {
			next = ""
		}
		return runtimectx.SandboxProcessRecoveryBatch{NextCursor: next, HasMore: true}, f.failure
	}
	return runtimectx.SandboxProcessRecoveryBatch{NextCursor: cursor}, nil
}
func (f *desktopRecoveryFixture) RecoverPendingSandboxLifecycles(_ context.Context, _ runtimectx.SandboxProcessRecoveryOwnership, cursor string, limit int) (runtimectx.SandboxLifecycleRecoveryBatch, error) {
	f.calls = append(f.calls, "lifecycle:"+cursor)
	return runtimectx.SandboxLifecycleRecoveryBatch{NextCursor: cursor}, nil
}
func TestDesktopSandboxRecoveryKeepsFailuresAcrossBothStages(t *testing.T) {
	failure := errors.New("original process still unknown")
	fixture := &desktopRecoveryFixture{failure: failure}
	_, err := recoverDesktopSandbox(t.Context(), fixture, nil)
	if !errors.Is(err, failure) {
		t.Fatalf("lost recovery error: %v", err)
	}
	if !reflect.DeepEqual(fixture.calls, []string{"process:", "process:a", "lifecycle:"}) {
		t.Fatalf("order: %v", fixture.calls)
	}
}
func TestDesktopSandboxRecoveryRejectsStalledCursorAndCancellation(t *testing.T) {
	fixture := &desktopRecoveryFixture{stuck: true}
	if _, err := recoverDesktopSandbox(t.Context(), fixture, nil); err == nil || len(fixture.calls) != 1 {
		t.Fatalf("stalled scan: %v %v", err, fixture.calls)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	fixture = &desktopRecoveryFixture{}
	if _, err := recoverDesktopSandbox(ctx, fixture, nil); !errors.Is(err, context.Canceled) || len(fixture.calls) != 0 {
		t.Fatalf("canceled scan: %v %v", err, fixture.calls)
	}
}
func TestDesktopSandboxDefaultRequiresOwnership(t *testing.T) {
	services := &AppServices{}
	if err := services.prepareDesktopSandbox(config.Config{AppMode: "desktop"}, nil); err == nil {
		t.Fatal("desktop silently omitted supervisor")
	}
	if err := services.prepareDesktopSandbox(config.Config{AppMode: "web"}, nil); err != nil {
		t.Fatal(err)
	}
}

type desktopRecoveryItemFixture struct{}

func (desktopRecoveryItemFixture) RecoverPendingSandboxProcesses(context.Context, runtimectx.SandboxProcessRecoveryOwnership, string, int) (runtimectx.SandboxProcessRecoveryBatch, error) {
	failure := errors.New("process still registered")
	return runtimectx.SandboxProcessRecoveryBatch{Items: []runtimectx.SandboxProcessRecoveryItem{
		{Key: protocol.SandboxProcessKey{OwnerUserID: "owner", SessionKey: "dm", Generation: 1, LaunchID: "a"}, Err: failure},
		{Key: protocol.SandboxProcessKey{OwnerUserID: "owner", SessionKey: "ok", Generation: 1, LaunchID: "b"}},
	}}, failure
}

func (desktopRecoveryItemFixture) RecoverPendingSandboxLifecycles(context.Context, runtimectx.SandboxProcessRecoveryOwnership, string, int) (runtimectx.SandboxLifecycleRecoveryBatch, error) {
	failure := errors.New("scratch recovery parent identity mismatch")
	return runtimectx.SandboxLifecycleRecoveryBatch{Items: []runtimectx.SandboxLifecycleRecoveryItem{
		{Key: protocol.SandboxProcessKey{OwnerUserID: "owner", SessionKey: "room", Generation: 2, LaunchID: "c"}, Err: failure},
	}}, failure
}

func TestDesktopSandboxRecoveryReportsEachFailedRecordByStage(t *testing.T) {
	records, err := recoverDesktopSandbox(t.Context(), desktopRecoveryItemFixture{}, nil)
	if err == nil {
		t.Fatal("aggregate recovery error was dropped")
	}
	if len(records) != 2 ||
		records[0].Key.LaunchID != "a" || records[0].Stage != runtimectx.SandboxStartupRecoveryProcess || records[0].Discardable() ||
		records[1].Key.LaunchID != "c" || records[1].Stage != runtimectx.SandboxStartupRecoveryLifecycle || !records[1].Discardable() {
		t.Fatalf("records=%+v", records)
	}
}
