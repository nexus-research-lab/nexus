// Package subscription 是订阅套餐与用户订阅的 SQL 仓储。
//
// L2 | 父级: internal/storage（L1 见 AGENTS.md）
//
// 成员清单：
//   - repository.go / plan.go / user.go：持久化模型、仓储入口、套餐与用户订阅读写。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package subscription
