// Package automation 是自动化（定时任务 + heartbeat）特性域的行为层：调度计算、
// 执行观测、会话解析与进程内运行态。特性内共享的类型词汇下沉到子包 protocol/。
//
// L2 | 父级: internal（L1 见 AGENTS.md）
//
// 成员清单：
//   - schedule.go：ComputeNextRunAt 计算下次触发时间。
//   - session.go：ResolveSessionKey 解析任务的真实执行会话。
//   - heartbeat_prompt.go / heartbeat_delivery.go：HEARTBEAT.md 周期任务提示与回复外发过滤。
//   - execution_sink.go：ExecutionSink 收敛一轮执行的最终观测结果。
//   - runtime_state.go：JobRuntimeState 进程内任务运行态、HeartbeatWakeRequest 内部唤醒命令。
//   - actor_context.go：标记本次自动化动作的发起 Agent。
//   - task_search.go：CronJobMatchesQuery 按口头描述匹配定时任务。
//
// 子包：protocol/（特性域共享类型、枚举、输入校验）。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package automation
