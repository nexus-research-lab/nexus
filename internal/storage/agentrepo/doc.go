// Package agentrepo 是 Agent 记录的 SQL 仓储。
//
// L2 | 父级: internal/storage（L1 见 AGENTS.md）
//
// 成员清单：
//   - repository_sql.go：Agent SQL 读写。
//   - model_agent.go / scan_agent.go：落库记录模型与行扫描。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package agentrepo
