// Package room 封装房间域的 HTTP handlers（含 Agent 私域投影）。
//
// L2 | 父级: internal/handler（L1 见 AGENTS.md）
//
// 成员清单：
//   - handlers.go：Handlers 及房间路由。
//   - conversation.go：房间会话 handler。
//   - agent_private_domain.go：Agent 私域线程列表 handler。
//   - subagent_task.go：房间内子任务 handler。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package room
