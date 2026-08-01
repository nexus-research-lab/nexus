// Package runtime 驱动 bridge runtime 的 round 执行与会话生命周期。
//
// L2 | 父级: internal（L1 见 AGENTS.md）
//
// 成员清单：
//   - client.go：Client 接口、Factory 与 sdkClientAdapter（runtime 需要的最小 SDK 能力抽象），
//     并统一收口并发连接失败、永久撤销失去 Manager 所有权的 client、识别关闭态控制错误
//     及隔离未收口的 SDK 会话。
//   - session.go / round.go / idle*.go / owner.go / interrupt.go / streaming_input.go / task.go /
//     mcp.go / goal_accounting.go：Manager 管理 session_key → SDK client、owner、运行中 round、
//     key 级启动与关闭栅栏、client 换代、lease 条件关闭、Goal accounting、
//     scope-aware Goal create guard、ClearGoalAccountingRounds 部分 activation 回滚与
//     objective revision adoption，并支持按 owner 强制回收。
//   - guidance.go / contextual_input.go / input_options.go：轮内引导、协商后的 applied ACK 消费回调、隐藏上下文和输入选项剥离。
//   - diagnostics_env.go / stderr_line.go：诊断开关、stderr 归一化。
//   - goal_usage.go / subagent_usage.go / context_usage.go：Goal actual/budget token
//     口径换算、跨 round 的 nxs child task 累计量去重，以及 runtime 权威上下文快照
//     的归一化与按 Session/Agent 热缓存；跨进程恢复由 Session 服务负责。
//   - round_timeout.go / text_util.go：跨 core/exec 共用的常量与小工具。
//
// 子包：exec/（轮次执行内核，ExecuteRound 主链）、trace/（SDK 消息调试字段与摘要）。
// 系统消息到产品事件的投影统一由 internal/message 负责，runtime 不保留第二套展示语义。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package runtime
