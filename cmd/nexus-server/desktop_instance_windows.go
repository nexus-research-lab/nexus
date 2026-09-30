//go:build windows

// INPUT: Windows 桌面 sidecar 的 canonical 状态根。
// OUTPUT: 保持到所有服务关闭之后的独占实例句柄。
// POS: 可执行入口装配，不复用原生窗口进程的单实例锁。
package main

import (
	"io"

	"github.com/nexus-research-lab/nexus/internal/infra/desktopinstance"
)

func acquireDesktopInstanceLock(root string) (io.Closer, error) { return desktopinstance.Acquire(root) }
