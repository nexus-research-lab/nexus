// Package desktopinstance 持有 macOS sidecar 的状态根独占锁。
//
// L2 | 父级: internal/infra（L1 见 AGENTS.md）
// desktop_instance_*.go 提供跨平台 AcquireForPlatform 入口，非 macOS 不获取锁。
// lock_darwin.go 在固定 app 目录句柄中创建稳定锁文件，并使用内核 flock；
// 锁不包含 PID、不因年龄删除、不解锁他人实例，进程退出由内核释放。
// 调用方在任何迁移/恢复/数据库打开前 Acquire，在所有服务关闭后 Close。
// WithOwnership 核验目录/锁 inode，并在整个恢复回调期间保持句柄，阻止并发 Close。
// 此锁只协调采用本协议的 sidecar，不能单独证明旧版未持锁的进程不存在。
package desktopinstance
