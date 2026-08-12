// Package agent 封装 Agent / Session 域的 HTTP handlers。
//
// L2 | 父级: internal/handler（L1 见 AGENTS.md）
//
// 成员清单：
//   - handlers.go：Handlers 及 Agent CRUD 路由。
//   - contacts.go：Agent 双向联系人管理路由。
//   - communication.go：owner 以指定 Agent 视角打开联系人通道和发送消息。
//   - session.go：会话相关 handler。
//   - subagent_task.go：父会话可见子任务 handler。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package agent
