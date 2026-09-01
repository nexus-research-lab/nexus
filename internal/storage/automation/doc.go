// Package automation 是定时任务与 heartbeat 的 SQL 仓储。
//
// L2 | 父级: internal/storage（L1 见 AGENTS.md）
//
// 成员清单：
//   - repository.go：仓储类型与共享 SQL 方言入口。
//   - task*.go / heartbeat.go / heartbeat_wake.go：任务创建幂等、完整配置版本 CAS、durable deletion claim/finalize、scheduler 单列停用 CAS、创建 provenance/独立 delivery grant、Room 结果回复 Agent、会话绑定失效状态；heartbeat 配置与 wake acceptance 共用配置事务栅栏（首次无配置行时以稳定 Agent 行串行化），wake outbox 按 owner/request/intent 唯一，以 exact claim 收口；未领取行可恢复，过期 processing 只 fail closed、不重投。
//   - run*.go / event.go / retry.go / runtime.go / lease.go：
//     运行、事件、最早投递重试 deadline、运行时与调度租约/expiry 读写；执行领取以 exact owner/job/run/configuration/permission snapshot 与首条 run ledger 同事务提交，人工 request 在 owner 内唯一并可按 intent 重放 exact run；run 在开始时固化首次投递目标，重投递必须先以 owner/job/run、配置版本、attempts 和内部 token 原子领取，再按 exact token 完成。
//     execution terminal 与 exact task runtime 在同一事务提交；首次投递先保存 pending，
//     再以内部 attempt token 唯一领取。删除态 terminal 只允许 exact 私有 deletion token
//     的独立 suppressed CAS，强制 not_attempted/dead-letter 且不改任务摘要；人工停止
//     确认还必须同时匹配 review_required 与当前 configuration_version；scheduler 的 review-required 恢复审计以单次批量查询读取 active runs，不能按任务 N+1 扫描。
//   - permission.go：任务策略 CAS、owner-scoped 请求决策、原 IM 会话键、run 阻塞与 exact request 绑定的安全重试事务；权限拒绝或策略修订把无结果终态原子收口为 not_attempted，并只投影 exact 最新完成 run 摘要。
//   - runtime_command.go：Agent-facing Automation command 的 durable request/intent claim、结果重放与 uncertain fail-closed ledger；run 可用 exact run acceptance、wake 可用 exact outbox acceptance 对账 ledger 提交间隙。
//   - scan.go / value_sql.go：行扫描与 SQL 值编码。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package automation
