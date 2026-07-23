// Package configurationmcp 提供仅注入主智能体的 nexus_config 进程内 MCP server。
//
// L2 | 父级: internal/mcp（L1 见 AGENTS.md）
//
// 成员清单：
//   - server.go：按可信 runtime 上下文构建配置 MCP server。
//   - contract/：配置服务与 Actor 契约。
//   - tool/：发现/核对、预检、幂等应用与审计查询工具。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package configurationmcp
