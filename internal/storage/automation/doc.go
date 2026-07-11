// Package automation 是定时任务与 heartbeat 的 SQL 仓储。
//
// L2 | 父级: internal/storage（L1 见 AGENTS.md）
//
// 成员清单：
//   - repository.go：仓储类型与共享 SQL 方言入口。
//   - task*.go / run*.go / event.go / heartbeat.go / retry.go / runtime.go / lease.go：
//     任务、运行、事件、heartbeat、重试、运行时与调度租约读写。
//   - scan_automation.go / value_sql.go：行扫描与 SQL 值编码。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package automation
