// Package agent 封装 Agent / Session 域的 HTTP handlers。
//
// L2 | 父级: internal/handler（L1 见 AGENTS.md）
//
// 成员清单：
//   - handlers.go：Handlers、Agent CRUD 与 exact 创建回执路由。
//   - failure.go：Agent 创建/更新/删除的提交证据与 FailureCore 映射。
//   - contacts.go：Agent 双向联系人管理路由。
//   - communication.go：owner 以指定 Agent 视角打开联系人通道和发送消息，并按提交前/结果未知证据写出 FailureCore。
//   - session.go：会话相关 handler。
//   - subagent_task.go：父会话可见子任务 handler。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package agent
