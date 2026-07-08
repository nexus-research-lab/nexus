// Package goal 封装 Goal 域的 HTTP handlers（含 Codex app-server 风格 thread/goal）。
//
// L2 | 父级: internal/handler（L1 见 AGENTS.md）
//
// 成员清单：
//   - handlers.go：Handlers 及 Goal 路由。
//   - appserver.go：HandleThreadGoalSet 等 app-server 风格接口。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package goal
