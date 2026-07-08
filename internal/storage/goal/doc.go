// Package goal 是 Goal 领域的 SQL 仓储。
//
// L2 | 父级: internal/storage（L1 见 AGENTS.md）
//
// 成员清单：
//   - repository.go / repository_event.go：Goal 与事件读写。
//   - scan.go / value_sql.go：行扫描与 SQL 值编码。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package goal
