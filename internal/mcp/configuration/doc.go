// Package configurationmcp 提供注入可信交互式 Agent DM 与 Room runtime 的 nexus_config 进程内 MCP server。
//
// L2 | 父级: internal/mcp（L1 见 AGENTS.md）
//
// 成员清单：
//   - server.go：按可信业务上下文与 active runtime lease 构建配置 MCP server；权限由服务端逐次动态重验。
//   - contract/：不可由模型参数覆盖的 Agent/DM/Room 上下文与最小配置服务契约。
//   - tool/：按作用域发现/核对、plan digest 预检、幂等应用与审计查询工具。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package configurationmcp
