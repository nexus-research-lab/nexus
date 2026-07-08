// Package automation 实现定时任务与 heartbeat 的调度、执行观测和会话解析。
//
// L2 | 父级: internal（L1 见 AGENTS.md）
//
// 成员清单：
//   - schedule.go：ComputeNextRunAt 计算下次触发时间。
//   - heartbeat_prompt.go / heartbeat_delivery.go：HEARTBEAT.md 周期任务提示与回复外发过滤。
//   - execution_sink.go：ExecutionObservation 执行轮次的最终观测结果。
//   - session.go：ResolveSessionKey 解析任务的真实执行会话。
//   - runtime_state.go：JobRuntimeState 进程内任务运行态。
//   - actor_context.go：标记本次自动化动作的发起 Agent。
//   - task_search.go：CronJobMatchesQuery 按口头描述匹配定时任务。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package automation
