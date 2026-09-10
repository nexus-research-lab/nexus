// Package mcp 保存 Nexus 内置 MCP 工具在一个 physical round 中共享的可信上下文与命令回执状态。
//
// L2 | 父级: internal（L1 见 AGENTS.md）
//
// 具体工具实现位于各子包；宿主自有工具最终由 app/runtime 合并进单一 nexus MCP server。
// artifact/ 提供当前 Agent round 的 deliver_files，复用 workspace 校验并随原消息持久化交付。
// round_state.go 保存可信 round 上下文、父 runtime Subagent control、命令尝试计数与单调回执。
//
// [PROTOCOL]: 变更时更新此头部，然后检查 AGENTS.md（L1）
package mcp
