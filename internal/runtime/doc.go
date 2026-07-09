// Package runtime 驱动 Claude Code runtime 的 round 执行与会话生命周期。
//
// L2 | 父级: internal（L1 见 AGENTS.md）
//
// 成员清单：
//   - client.go：Client 接口、Factory 与 sdkClientAdapter（runtime 需要的最小 SDK 能力抽象）。
//   - manager*.go：Manager 管理 session_key → SDK client 与运行中 round——会话获取/复用、
//     空闲回收、中断、streaming input、后台任务、MCP、Goal 结算。
//   - guidance.go / contextual_input.go / input_options.go：注入下一轮的引导与隐藏上下文、输入选项剥离。
//   - diagnostics_env.go / stderr_line.go：诊断开关、stderr 归一化。
//   - goal_usage.go / summary_system_message.go：Goal token 口径换算、系统消息面向前端的摘要。
//   - round_timeout.go / text_util.go：跨 core/exec 共用的常量与小工具。
//
// 子包：exec/（轮次执行内核，ExecuteRound 主链）、trace/（SDK 消息调试字段与摘要）。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package runtime
