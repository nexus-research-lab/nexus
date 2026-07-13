// Package builder 把 MCP 工具入参里的调度对象翻译成 automation 领域结构。
//
// L2 | 父级: internal/mcp/automation（L1 见 AGENTS.md）
//
// 成员清单：
//   - schedule.go：解析、规范化并校验 schedule。
//
// 会话目标、投递目标和来源包含调用方语义，由上层 semantic 包统一解析。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package builder
