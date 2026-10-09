//go:build darwin

// INPUT: macOS 桌面 sidecar 的 canonical 状态根。
// OUTPUT: 保持到所有服务关闭之后的独占实例句柄。
// POS: 平台锁装配，不复用原生窗口进程的单实例锁。
package desktopinstance

import "io"

// AcquireForPlatform 按当前平台获取桌面实例锁；非 macOS 返回空句柄。
func AcquireForPlatform(root string) (io.Closer, error) { return Acquire(root) }
