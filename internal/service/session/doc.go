// Package session 编排文件会话与 Room SQL 会话的统一视图。
//
// L2 | 父级: internal/service（L1 见 AGENTS.md）
//
// 成员清单：
//   - service.go / query.go / history.go：Service、查询、历史消息。
//   - mutation.go / model.go / util.go：增删改、模型、辅助。
//   - runtime.go / subagent_task.go / workspace.go：运行时、父会话可见子任务、workspace。
//   - repository.go：持久化。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package session
