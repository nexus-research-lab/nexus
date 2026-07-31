// Package runtime 驱动 bridge runtime 的 round 执行与会话生命周期。
//
// L2 | 父级: internal（L1 见 AGENTS.md）
//
// 成员清单：
//   - client.go：Client 接口、Factory 与 sdkClientAdapter（runtime 需要的最小 SDK 能力抽象），并统一识别进程退出后的关闭态控制错误。
//   - session.go / round.go / idle*.go / owner.go / interrupt.go / streaming_input.go / task.go /
//     mcp.go / goal_accounting.go：Manager 管理 session_key → SDK client、owner、运行中 round、
//     Goal accounting、scope-aware Goal create guard、ClearGoalAccountingRounds 部分 activation
//     回滚与 objective revision adoption，并支持按 owner 强制回收；interrupt.go 额外区分
//     sole-running-round provider interrupt 与 exact local context cancellation，并在 provider
//     interrupt 窗口阻止 successor admission；shared session 不回退为可能误伤 successor 的 interrupt。
//   - guidance.go / contextual_input.go / input_options.go / execution_tool_context.go / subagent_hook.go：轮内引导、隐藏上下文、含 structured WorkBinding/ReviewBinding 的 Execution MCP identity、由 runtime exact Goal authority mint 的协调 capability，以及按 parent round/tool_use_id 冻结 lifecycle callback 的 Agent tool 强准入、迟到事件、固定 grace deadline 持久化与进程内 fallback 路由。
//   - diagnostics_env.go / stderr_line.go：诊断开关、stderr 归一化。
//   - goal_usage.go / subagent_usage.go：Goal actual/budget token 口径换算与跨 round 的 nxs child task 累计量去重。
//   - round_timeout.go / text_util.go：跨 core/exec 共用的常量与小工具。
//
// 子包：exec/（轮次执行内核，ExecuteRound 主链）、trace/（SDK 消息调试字段与摘要）。
// 系统消息到产品事件的投影统一由 internal/message 负责，runtime 不保留第二套展示语义。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package runtime
