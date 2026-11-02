//go:build !darwin

// INPUT: 非 macOS 的现有启动入口。
// OUTPUT: 不获取 Go 层实例锁；Windows 单实例由桌面壳 mutex 与 Job 所有权保证。
// POS: 平台锁装配，不改变 Windows/Linux 启动策略。
package desktopinstance

import "io"

// AcquireForPlatform 按当前平台获取桌面实例锁；非 macOS 返回空句柄。
func AcquireForPlatform(string) (io.Closer, error) { return nil, nil }
