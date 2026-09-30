//go:build !darwin

// INPUT: 非 macOS 的现有启动入口。
// OUTPUT: 保持原平台实例管理行为，不启用 macOS 恢复凭据。
// POS: 本批仅装配 macOS sidecar，不改变 Windows/Linux 启动策略。
package main

import "io"

func acquireDesktopInstanceLock(string) (io.Closer, error) { return nil, nil }
