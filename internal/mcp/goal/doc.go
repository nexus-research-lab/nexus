// Package goalmcp 提供 nexus_goal 内建 MCP server 入口。
//
// L2 | 父级: internal/mcp（L1 见 AGENTS.md）
//
// 成员清单：
//   - server.go：NewServer 按当前会话上下文构建 Goal MCP server。
//
// 契约见 contract/，工具实现见 tool/。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package goalmcp
