// Package subagent 适配父会话的原生子智能体控制。
//
// L2 | 父级: internal/mcp/command（L1 见 AGENTS.md）
// 成员: command.go 定义按需 operation contract、closed input 校验、可信工具身份与同轮 mutation 回执。
// 暴露: NewHandler、WithToolUseID；生命周期与工作图准入仍归 runtime/SDK 与 orchestration。
// [PROTOCOL]: 变更时更新此头部，然后检查父级 doc.go。
package subagent
