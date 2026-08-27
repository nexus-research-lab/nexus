// Package connectorauthorization 提供 nexus MCP 中受控的单一 Connector 授权 action 工具。
//
// L2 | 父级: internal/mcp（L1 见 AGENTS.md）
//
// 成员清单：
//   - server.go：Connector 授权工具定义装配入口。
//   - contract/：服务端固定 owner/main DM、human principal 与 runtime lease。
//   - tool/：承载 start/status/cancel 的单一严格 action 工具；不接受 owner/session/state/code/token。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package connectorauthorization
