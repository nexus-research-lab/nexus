// Package agent 封装 Agent / Session 域的 HTTP handlers。
//
// L2 | 父级: internal/handler（L1 见 AGENTS.md）
//
// 成员清单：
//   - handlers.go：Handlers 及 Agent CRUD 路由。
//   - session.go：会话相关 handler。
//   - subagent_task.go：父会话可见子任务 handler。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package agent
