// Package runtime 驱动 bridge runtime 的 round 执行与会话生命周期。
//
// L2 | 父级: internal（L1 见 AGENTS.md）
//
// 成员清单：
//   - client.go：Client 接口、Factory 与 sdkClientAdapter（runtime 需要的最小 SDK 能力抽象）。
//   - session.go / round.go / idle*.go / interrupt.go / streaming_input.go / task.go / mcp.go /
//     goal_accounting.go：Manager 管理 session_key → SDK client 与运行中 round。
//   - guidance.go / contextual_input.go / input_options.go：注入下一轮的引导与隐藏上下文、输入选项剥离。
//   - diagnostics_env.go / stderr_line.go：诊断开关、stderr 归一化。
//   - goal_usage.go：Goal token 口径换算。
//   - round_timeout.go / text_util.go：跨 core/exec 共用的常量与小工具。
//
// 子包：exec/（轮次执行内核，ExecuteRound 主链）、trace/（SDK 消息调试字段与摘要）。
// 系统消息到产品事件的投影统一由 internal/message 负责，runtime 不保留第二套展示语义。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package runtime
