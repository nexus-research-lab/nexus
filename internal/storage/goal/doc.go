// Package goal 是 Goal 领域的 SQL 仓储。
//
// L2 | 父级: internal/storage（L1 见 AGENTS.md）
//
// 成员清单：
//   - repository.go / create.go / update_event.go / event.go：Goal 双 token 总量、当前 Goal 恢复读面、Goal + created event 原子创建、event-bearing optimistic update、事件读写与显式事务级联删除。
//   - usage_recording.go：parent usage 聚合与对应审计事件的原子提交。
//   - usage_finalization.go：最终 usage、pending barrier、持久 fence 与事件的单事务提交。
//   - usage_source.go：runtime child checkpoint、durable scope 解析、Goal actual usage 与审计事件的原子提交。
//   - usage_evidence.go：每个 child/source-round 的 terminal/provider presence、external baseline-unavailable、from-now tombstone 与 finalization barrier。
//   - usage_parent.go：Room parent terminal source-round ledger、provider presence 与 exactly-once 全量 usage 回补。
//   - usage_scope.go / usage_scope_create.go / usage_scope_bind.go：open/bound/closed scope、model round-start 回补、external from-now 排除与删除 tombstone。
//   - scan.go / value_sql.go：actual/budget token 行扫描、历史矛盾零 total 回算与 SQL 值编码。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package goal
