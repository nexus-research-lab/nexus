// Package usage 负责用户级 token usage ledger。
//
// L2 | 父级: internal/service（L1 见 AGENTS.md）
//
// 成员清单：
//   - service.go：usage 模型、ledger 记账与 cache segment 聚合；观测归因失败不反向影响主账。
//   - cache_attribution.go：固定枚举与低基数脱敏归因。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package usage
