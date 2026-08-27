// Package automation 是自动化（定时任务 + heartbeat）特性域的行为层：调度计算、
// 执行观测、会话解析与进程内运行态。特性内共享的类型词汇下沉到子包 types/。
//
// L2 | 父级: internal（L1 见 AGENTS.md）
//
// 成员清单：
//   - schedule.go：ComputeNextRunAt 计算下次触发时间。
//   - heartbeat_prompt.go：按解析阶段读取 HEARTBEAT.md 周期任务，并过滤回复外发。
//   - execution_sink.go：ExecutionSink 按消息、错误与轮次终态收敛执行观测结果。
//   - runtime.go：进程内运行态、会话解析与动作发起 Agent 上下文。
//   - task_search.go：ScheduledTaskMatchesQuery 按口头描述匹配定时任务。
//
// 子包：types/（特性域共享类型、枚举、输入校验）。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package automation
