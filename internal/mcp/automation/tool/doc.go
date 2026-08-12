// Package tool 实现 nexus_automation MCP 的意图级工具集。
//
// L2 | 父级: internal/mcp/automation（L1 见 AGENTS.md）
//
// 成员清单：
//   - create.go / find.go / update.go / delete.go：任务生命周期。
//   - inspect.go / report.go：任务诊断与聚合报告。
//   - run.go / repair.go：立即执行与故障修复。
//   - heartbeat.go：按 Agent 权限读取、CAS 更新 heartbeat，及独立的 wake 动作。
//   - configuration_verify.go：scheduled task 写后重读与配置版本核验。
//   - registry.go / metadata.go / schema.go：工具注册、检索元数据，以及按可信会话
//     动态隐藏宿主所有 IM 路由字段的输入契约。
//   - scope.go / delivery_scope.go / current_context_query.go / history_context_query.go / report_context.go：
//     可信写入、Agent 所有权、投递目标授权与外部会话只读边界。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package tool
