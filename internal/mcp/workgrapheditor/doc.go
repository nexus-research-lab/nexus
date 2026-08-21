// Package workgrapheditor 提供只挂载到 exact 临时草图 Session 的受限 MCP server。
//
// L2 | 父级: internal/mcp（L1 见 AGENTS.md）
//
// 成员清单：
//   - server.go：只暴露 revise_workgraph_preview，接收带 revision 的完整草图并交给 WorkGraph 服务校验。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package workgrapheditor
