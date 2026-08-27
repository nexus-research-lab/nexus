// Package runtime 驱动 bridge runtime 的 round 执行与会话生命周期。
//
// L2 | 父级: internal（L1 见 AGENTS.md）
//
// 成员清单：
//   - client.go：Client 接口、Factory 与 agentClient（宿主管理 Agent runtime 的能力边界），
//     并统一收口并发连接失败、永久撤销失去 Manager 所有权的 client、取消换代中的
//     connect/config RPC、识别关闭态控制错误及隔离未收口的 SDK 会话。
//   - session.go / round.go / idle*.go / owner.go / interrupt.go / streaming_input.go / task.go /
//     goal_accounting.go：Manager 管理 session_key → SDK client、owner、运行中 round、
//     key 级启动与关闭栅栏、client 换代、lease 条件关闭、round keyed state、
//     兼容性在 GetOrCreate 前已失败时对既有 warm client 的 owner-fenced 退休、
//     idle 消息消费者租约，以及不占用全局锁的 owner epoch/reap flight；Goal accounting、
//     scope-aware Goal create guard、ClearGoalAccountingRounds 部分 activation 回滚与
//     objective revision adoption 均随 round state 统一清理；interrupt.go 额外区分唯一运行
//     round 的 provider interrupt 与 exact local context cancellation，并在 provider interrupt
//     窗口阻止 successor admission，共享 session 不回退为可能误伤 successor 的 interrupt；
//     Goal pause 使用 exact Goal/revision→round accounting identity 逐轮取消，不误伤同 session 其他工作。
//   - guidance.go / contextual_input.go / command_context.go / responsibility_authority.go / work_binding_state.go / goal_authority.go / subagent_hook.go：轮内引导、隐藏上下文与输入选项、runtime lease 与 Connector 选择、Goal/Execution/Work/Review 共用且由宿主 mutation receipt 原子推进的动态 responsibility snapshot（WorkBinding exact fail-close）、provider init/fork 后动态更新且 command 调用时读取的 SDK Session identity、Goal steering 与 mutation fence 分离，以及按 parent round/tool_use_id 冻结 lifecycle callback 的 Agent tool 强准入、迟到事件、固定 grace deadline 持久化、无上限退避 fallback 与重启时 process-cutoff orphan 对账。
//   - diagnostics_env.go / cache_surface.go：诊断开关、stderr 归一化，以及不持久化
//     prompt/tool schema 明文、也不冒充 provider cache key 的宿主 tool surface 脱敏归因；
//     同一指纹也为不支持会话内动态工具更新的 runtime 提供保守 resume/reset 栅栏。
//   - goal_usage.go / task.go / context_usage.go：Goal actual/budget token
//     口径换算（含矛盾 provider 零 total 的 breakdown 回退）、跨 round 的 nxs child task 累计量去重，以及 runtime 权威上下文快照
//     的归一化与按 Session/Agent 热缓存；跨进程恢复由 Session 服务负责。
//   - lifecycle.go：session 关闭栅栏与跨 core/exec 共用的 round 中断宽限。
//
// 子包：exec/（轮次执行内核，ExecuteRound 主链）、trace/（SDK 消息调试字段与摘要）。
// 系统消息到产品事件的投影统一由 internal/message 负责，runtime 不保留第二套展示语义。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package runtime
