// Package usage 是 token usage ledger 的 SQL 仓储。
//
// L2 | 父级: internal/storage（L1 见 AGENTS.md）
//
// 成员清单：
//   - repository.go：usage 主账读写、best-effort cache attribution 更新与 segment 聚合。
//   - model.go：Record、CacheSegment 等模型。
//
// 每日用量按 owner 隔离并聚合最近 365 个 UTC 自然日，空日期补零。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package usage
