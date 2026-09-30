//go:build windows

// INPUT: 宿主显式预览配置、迁移前实例锁和机器安装的受保护清单。
// OUTPUT: 固定映像和日志根的 Windows 监督装配；失败阻止启动，不自动重放旧任务。
// POS: Windows App 入口；预览不替代 SDK 能力握手或发布验收。
package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/windowssandbox"
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
)

// prepareDesktopSandbox 仅接受宿主预览开关；任务配置不能选择映像或日志位置。
func (s *AppServices) prepareDesktopSandbox(cfg config.Config, ownership runtimectx.SandboxProcessRecoveryOwnership) error {
	if !cfg.WindowsSandboxPreview || !strings.EqualFold(strings.TrimSpace(cfg.AppMode), "desktop") {
		return nil
	}
	if s.Runtime == nil || ownership == nil {
		return errors.New("Windows sandbox preview requires runtime and sidecar ownership")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	installed, err := windowssandbox.LoadInstalled(ctx)
	if err != nil {
		// 校验错误若仍持有原文件句柄，沿 App 的关闭入口保留其关闭责任。
		var retained io.Closer
		if errors.As(err, &retained) {
			s.closeSandboxResources = retained.Close
		}
		return fmt.Errorf("load installed Windows sandbox: %w", err)
	}
	return s.installWindowsDesktopSandbox(ownership, installed)
}

// installWindowsDesktopSandbox 的映像只能来自机器清单；空网络端点明确拒绝网络。
// 冷启动 unknown 由 Manager 的持久启动栅栏保留，此处不凭 PID 或路径宣称回收成功。
func (s *AppServices) installWindowsDesktopSandbox(ownership runtimectx.SandboxProcessRecoveryOwnership, installed windowssandbox.Installed) error {
	if s.Runtime == nil || ownership == nil {
		return errors.New("Windows sandbox startup dependencies missing")
	}
	return ownership.WithOwnership(func(appRoot string) error {
		root, err := confinedfs.Open(appRoot)
		if err != nil {
			return err
		}
		defer root.Close()
		jobs, err := root.OpenOrCreateRootNoSymlink("processes", 0700)
		if err != nil {
			return fmt.Errorf("open Windows process registry: %w", err)
		}
		if err := s.Runtime.SetWindowsSandboxSupervisor(runtimectx.WindowsSandboxSupervisor{
			Root: jobs, HelperPath: installed.HelperPath, HelperSHA256: installed.HelperSHA256,
			ResolveNetworkEndpoints: resolveWindowsSandboxEndpoints,
		}); err != nil {
			return errors.Join(err, jobs.Close())
		}
		s.closeSandboxResources = sync.OnceValue(jobs.Close)
		return nil
	})
}
