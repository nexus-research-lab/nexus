// Package session 编排文件会话与 Room SQL 会话的统一视图。
//
// L2 | 父级: internal/service（L1 见 AGENTS.md）
//
// 成员清单：
//   - service.go / query.go / history.go：Service、按字段所有权合并 Room SQL/workspace 投影的查询、历史消息，以及按当前 Goal 聚合真相刷新完成收据。
//   - mutation.go / model.go / util.go：增删改、模型、辅助。
//   - runtime.go / context_usage.go / subagent_task.go / subagent_tool_run.go / workspace.go：运行时、
//     Session 元数据上下文快照恢复、父会话可见子任务生命周期、独立 transcript 聚合及脱敏 ToolRun 历史投影、workspace。
//   - repository.go：持久化。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package session
