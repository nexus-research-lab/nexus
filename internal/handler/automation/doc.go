// Package automation 封装自动化域的 HTTP handlers。
//
// L2 | 父级: internal/handler（L1 见 AGENTS.md）
//
// 成员清单：
//   - handlers.go：Handlers、定时任务路由与 owner-scoped 持久审批 API。
//   - heartbeat.go：heartbeat handler。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package automation
