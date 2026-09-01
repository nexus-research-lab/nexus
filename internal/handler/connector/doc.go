// Package connector 封装连接器域的 HTTP handlers。
//
// L2 | 父级: internal/handler（L1 见 AGENTS.md）
//
// 成员清单：
//   - handlers.go：Handlers 及连接器授权/状态路由。
//   - custom_mcp.go：自定义 MCP 的 owner-scoped CRUD、历史密文恢复、启停与 Tools 发现路由。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package connector
