// Package executionmcp exposes the model-facing Execution Orchestration tools.
//
// L2 | 父级: internal/mcp（L1 见 AGENTS.md）
//
// 成员:
//   - contract: runtime identity and the orchestration service port.
//   - tool: the fixed semantic tool registry and its adapters.
//   - server.go: SDK MCP server assembly.
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package executionmcp
