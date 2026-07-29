// Package agentrepo 是 Agent 记录的跨方言 SQL 仓储。
//
// L2 | 父级: internal/storage（L1 见 AGENTS.md）
//
// 成员清单：
//   - sql.go：Agent SQL 读写与 Skill 绑定列的窄更新。
//   - model.go / scan.go：落库记录模型、Skill 启用/停用集合与行扫描。
//
// SQLRepository 根据 driver 选择 SQLDialect，不在上层复制 SQLite/PostgreSQL 门面。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package agentrepo
