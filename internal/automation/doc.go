// Package automation 是自动化（定时任务 + heartbeat）特性域：既定义特性内共享的类型，
// 也实现调度、执行观测与会话解析。
//
// L2 | 父级: internal（L1 见 AGENTS.md）
//
// 这些类型原在 internal/protocol，但只在 automation 特性簇（service/mcp/storage/cli/handler）
// 内流转、不进前端 codegen，按边界应归本域；作为叶子包（只依赖 protocol）供各层引用。
//
// 成员清单：
//   - automation.go：调度/目标/唤醒/投递/执行/来源等枚举常量。
//   - automation_job.go / automation_report.go：CronJob、CronRun、日报等对外视图。
//   - automation_input.go：CreateJobInput / UpdateJobInput 及校验、归一。
//   - automation_schedule.go / automation_target.go：Schedule / SessionTarget / DeliveryTarget / Source 及 Validate/Normalized。
//   - automation_heartbeat.go：HeartbeatConfig / HeartbeatWakeInput 等 heartbeat 协议。
//   - schedule.go：ComputeNextRunAt 计算下次触发时间。
//   - heartbeat_prompt.go / heartbeat_delivery.go：HEARTBEAT.md 周期任务提示与回复外发过滤。
//   - execution_sink.go：ExecutionObservation 执行轮次的最终观测结果。
//   - session.go：ResolveSessionKey 解析任务的真实执行会话。
//   - runtime_state.go：JobRuntimeState 进程内任务运行态、HeartbeatWakeRequest 内部唤醒命令。
//   - actor_context.go：标记本次自动化动作的发起 Agent。
//   - task_search.go：CronJobMatchesQuery 按口头描述匹配定时任务。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package automation
