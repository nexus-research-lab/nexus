//go:build darwin

// INPUT: 入口已持有的 sidecar 锁、随包可信 helper 与持久 Manager。
// OUTPUT: 完成进程及资源两阶段恢复后才交付服务；失败项保留原记录、只阻断各自会话，并交给 owner 显式处理。
// POS: App 默认桌面监督装配；任务不能选择 helper 或恢复身份。
package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
	"github.com/nexus-research-lab/nexus/internal/infra/runtimebootstrap"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
)

func (s *AppServices) prepareDesktopSandbox(cfg config.Config, ownership runtimectx.SandboxProcessRecoveryOwnership) error {
	if !strings.EqualFold(strings.TrimSpace(cfg.AppMode), "desktop") {
		return nil
	}
	if ownership == nil {
		return errors.New("desktop sandbox requires sidecar ownership")
	}
	artifact, err := runtimebootstrap.LoadCurrent()
	if err != nil {
		return fmt.Errorf("load desktop sandbox helper: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	return s.installDesktopSandbox(ctx, ownership, artifact)
}

// installDesktopSandbox 的 artifact 只允许来自宿主校验，测试可提供固定原生产物。
func (s *AppServices) installDesktopSandbox(ctx context.Context, ownership runtimectx.SandboxProcessRecoveryOwnership, artifact runtimebootstrap.Artifact) error {
	if s.Runtime == nil || ownership == nil {
		return errors.New("desktop sandbox startup dependencies missing")
	}
	var jobs *confinedfs.Root
	err := ownership.WithOwnership(func(appRoot string) error {
		root, err := confinedfs.Open(appRoot)
		if err != nil {
			return err
		}
		defer root.Close()
		jobs, err = root.OpenOrCreateRootNoSymlink("processes", 0700)
		return err
	})
	if err != nil {
		return fmt.Errorf("open desktop process registry: %w", err)
	}
	if err := s.Runtime.SetSandboxProcessSupervisor(runtimectx.SandboxProcessSupervisor{Root: jobs, HelperPath: artifact.Path, HelperSHA256: artifact.Manifest.SHA256}); err != nil {
		return errors.Join(err, jobs.Close())
	}
	// 句柄在 Manager 完成关闭后释放；失败路径也沿 AppServices.Close 收口。
	s.closeSandboxResources = sync.OnceValue(jobs.Close)
	// 恢复失败不阻断整个 App：未收口的持久记录继续按 owner/session 阻断各自会话，
	// 其他会话照常准入；失败项交给 owner 在设置页显式处理。
	failures, err := recoverDesktopSandbox(ctx, s.Runtime, ownership)
	if err != nil {
		slog.Error("desktop sandbox startup recovery left records pending; affected sessions stay fenced",
			"failed_records", len(failures), "err", err)
	}
	s.Runtime.RecordSandboxStartupRecovery(ownership, failures)
	return nil
}

type desktopSandboxRecovery interface {
	RecoverPendingSandboxProcesses(context.Context, runtimectx.SandboxProcessRecoveryOwnership, string, int) (runtimectx.SandboxProcessRecoveryBatch, error)
	RecoverPendingSandboxLifecycles(context.Context, runtimectx.SandboxProcessRecoveryOwnership, string, int) (runtimectx.SandboxLifecycleRecoveryBatch, error)
}

// recoverDesktopSandbox 完成全部原生页后才处理资源；单项失败不能被页末覆盖。
// 返回的失败项保留原 key，供 owner 定位；错误汇总仍完整返回给调用方记录。
func recoverDesktopSandbox(ctx context.Context, manager desktopSandboxRecovery, ownership runtimectx.SandboxProcessRecoveryOwnership) ([]runtimectx.SandboxStartupRecoveryFailure, error) {
	var errs []error
	var records []runtimectx.SandboxStartupRecoveryFailure
	for _, lifecycle := range []bool{false, true} {
		cursor := ""
		for {
			if err := ctx.Err(); err != nil {
				return records, errors.Join(append(errs, err)...)
			}
			var next string
			var more bool
			var err error
			if lifecycle {
				var batch runtimectx.SandboxLifecycleRecoveryBatch
				batch, err = manager.RecoverPendingSandboxLifecycles(ctx, ownership, cursor, 64)
				next, more = batch.NextCursor, batch.HasMore
				for _, item := range batch.Items {
					if item.Err != nil {
						records = append(records, runtimectx.SandboxStartupRecoveryFailure{Key: item.Key, Stage: runtimectx.SandboxStartupRecoveryLifecycle, Reason: item.Err.Error()})
					}
				}
			} else {
				var batch runtimectx.SandboxProcessRecoveryBatch
				batch, err = manager.RecoverPendingSandboxProcesses(ctx, ownership, cursor, 64)
				next, more = batch.NextCursor, batch.HasMore
				for _, item := range batch.Items {
					if item.Err != nil {
						records = append(records, runtimectx.SandboxStartupRecoveryFailure{Key: item.Key, Stage: runtimectx.SandboxStartupRecoveryProcess, Reason: item.Err.Error()})
					}
				}
			}
			if err != nil {
				errs = append(errs, err)
			}
			if !more {
				break
			}
			if next <= cursor {
				return records, errors.Join(append(errs, errors.New("desktop sandbox recovery made no cursor progress"))...)
			}
			cursor = next
		}
	}
	return records, errors.Join(errs...)
}
