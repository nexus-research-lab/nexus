// Package manager 提供受控的 nexus_manager 内建 MCP server。
//
// L2 | 父级: internal/mcp（L1 见 AGENTS.md）
//
// 成员清单：
//   - server.go：SDK 内建 MCP server 入口。
//   - contract/：服务端固定 Actor 与服务窄接口。
//   - tool/：按 owner-main、agent-self、room-member 动态裁剪的只读查询工具。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package manager
