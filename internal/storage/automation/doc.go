// Package automation 是定时任务与 heartbeat 的 SQL 仓储。
//
// L2 | 父级: internal/storage（L1 见 AGENTS.md）
//
// 成员清单：
//   - repository.go / repository_job.go / repository_run*.go / repository_event.go /
//     repository_task_event.go / repository_heartbeat.go / repository_retry.go /
//     repository_runtime.go：任务、运行、事件、heartbeat、重试、运行时读写。
//   - scan_automation.go / value_sql.go：行扫描与 SQL 值编码。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package automation
