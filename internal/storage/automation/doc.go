// Package automation 是定时任务与 heartbeat 的 SQL 仓储。
//
// L2 | 父级: internal/storage（L1 见 AGENTS.md）
//
// 成员清单：
//   - repository.go：仓储类型与共享 SQL 方言入口。
//   - task*.go / heartbeat.go：任务创建幂等、配置版本 CAS 与 heartbeat 配置/运行态分离写入。
//   - run*.go / event.go / retry.go / runtime.go / lease.go：
//     运行、事件、重试、运行时与调度租约读写。
//   - permission.go：任务策略 CAS、owner-scoped 请求决策、run 阻塞与安全重试事务。
//   - scan_automation.go / value_sql.go：行扫描与 SQL 值编码。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package automation
