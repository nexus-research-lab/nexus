// Package subscription 只读取 Control entitlement 本地投影与 Nexus 用量事实。
//
// L2 | 父级: internal/storage（L1 见 AGENTS.md）
//
// 成员清单：
//   - repository.go / user.go：只读仓储入口、用户有效额度与当月用量聚合。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package subscription
