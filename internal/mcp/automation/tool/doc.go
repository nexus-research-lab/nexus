// Package tool 实现 nexus_automation MCP 的意图级工具集。
//
// L2 | 父级: internal/mcp/automation（L1 见 AGENTS.md）
//
// 成员清单：
//   - create.go / find.go / update.go / delete.go：任务生命周期。
//   - inspect.go / report.go：任务诊断与聚合报告。
//   - run.go / repair.go：立即执行与故障修复。
//   - registry.go / metadata.go / schema.go：工具注册、检索元数据与输入契约。
//   - scope.go / current_context_query.go / history_context_query.go / report_context.go：权限范围与当前会话解析。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package tool
