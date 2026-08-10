// Package connectorauthorization 提供受控的 nexus_connector_auth 内建 MCP。
//
// L2 | 父级: internal/mcp（L1 见 AGENTS.md）
//
// 成员清单：
//   - server.go：SDK 内建 MCP server 入口。
//   - contract/：服务端固定 owner/main DM、human principal 与 runtime lease。
//   - tool/：start/status/cancel 严格参数工具；不接受 owner/session/state/code/token。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package connectorauthorization
