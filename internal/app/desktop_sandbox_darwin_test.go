//go:build darwin

// INPUT: 分页恢复结果、持锁装配和资源关闭事件。
// OUTPUT: 两阶段顺序、错误保留及无锁拒绝。
// POS: App 启动装配回归，原生收口另有集成测试。
package app

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
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
	err := recoverDesktopSandbox(t.Context(), fixture, nil)
	if !errors.Is(err, failure) {
		t.Fatalf("lost recovery error: %v", err)
	}
	if !reflect.DeepEqual(fixture.calls, []string{"process:", "process:a", "lifecycle:"}) {
		t.Fatalf("order: %v", fixture.calls)
	}
}
func TestDesktopSandboxRecoveryRejectsStalledCursorAndCancellation(t *testing.T) {
	fixture := &desktopRecoveryFixture{stuck: true}
	if err := recoverDesktopSandbox(t.Context(), fixture, nil); err == nil || len(fixture.calls) != 1 {
		t.Fatalf("stalled scan: %v %v", err, fixture.calls)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	fixture = &desktopRecoveryFixture{}
	if err := recoverDesktopSandbox(ctx, fixture, nil); !errors.Is(err, context.Canceled) || len(fixture.calls) != 0 {
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
