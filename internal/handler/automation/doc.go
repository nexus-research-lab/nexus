// Package automation 封装自动化域的 HTTP handlers。
//
// L2 | 父级: internal/handler（L1 见 AGENTS.md）
//
// 成员清单：
//   - handlers.go：Handlers、页面 Agent 任务 CRUD（含 Agent/Session 原子重绑、configuration_version CAS，并拒绝新建/编辑 script）与 owner-scoped 持久审批 API。
//   - heartbeat.go：heartbeat handler。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package automation
