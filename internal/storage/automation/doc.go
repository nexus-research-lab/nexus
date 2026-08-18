// Package automation 是定时任务与 heartbeat 的 SQL 仓储。
//
// L2 | 父级: internal/storage（L1 见 AGENTS.md）
//
// 成员清单：
//   - repository.go：仓储类型与共享 SQL 方言入口。
//   - task*.go / heartbeat.go：任务创建幂等、配置版本 CAS、创建 provenance/独立 delivery grant、Room 结果回复 Agent、会话绑定失效状态与 heartbeat 配置/运行态分离写入。
//   - run*.go / event.go / retry.go / runtime.go / lease.go：
//     运行、事件、最早投递重试 deadline、运行时与调度租约/expiry 读写；run 在开始时固化首次投递目标。
//   - permission.go：任务策略 CAS、owner-scoped 请求决策、原 IM 会话键、run 阻塞与安全重试事务。
//   - runtime_command.go：Agent-facing Automation CLI 的 durable request/intent claim、结果重放与 uncertain fail-closed ledger。
//   - scan_automation.go / value_sql.go：行扫描与 SQL 值编码。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package automation
