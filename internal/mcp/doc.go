// Package mcp 保存 Nexus 内置 MCP 工具在一个 physical round 中共享的可信上下文与命令回执状态。
//
// RoundContext 同时携带宿主实际派发正文，供 IM ingress 证据核对；正文不授予权限。
//
// L2 | 父级: internal（L1 见 AGENTS.md）
//
// 具体工具实现位于各子包；宿主自有工具最终由 app/runtime 合并进单一 nexus MCP server。
// artifact/ 提供当前 Agent round 的 deliver_files，复用 workspace 校验并随原消息持久化交付。
// round_state.go 保存可信 round 上下文、父 runtime Subagent control、命令尝试计数与单调回执（含批量派发的精确 Goal progress）；
// Room 的业务 RootRoundID 与成员物理 AgentRoundID 分别用于消息关联和 runtime lease。
// command/ 的 start/finish 诊断只用于离线统计，不能替代 typed receipt、授予权限或触发重放。
//
// [PROTOCOL]: 变更时更新此头部，然后检查 AGENTS.md（L1）
package mcp
