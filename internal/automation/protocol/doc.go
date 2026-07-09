// Package protocol 是 automation 特性域内共享的类型词汇：定时任务/heartbeat 的
// 模型、枚举、输入与其 Validate/Normalized（消息是否良构）。
//
// L2 | 父级: internal/automation（L1 见 AGENTS.md）
//
// 与顶层 internal/protocol 的区别：这些类型只在 automation 簇内流转、不跨前端 codegen，
// 故按特性域下沉；顶层 protocol 才是跨 HTTP/WS/前端/运行时的真相源。父域 automation
// 的行为代码（调度计算、会话解析、执行观测等）依赖本包，反向不依赖。
//
// 成员清单：
//   - automation.go：调度/目标/唤醒/投递/执行/来源/运行状态等枚举常量。
//   - automation_job.go / automation_report.go：CronJob、CronRun、日报等对外视图。
//   - automation_input.go：CreateJobInput / UpdateJobInput 及校验、归一。
//   - automation_schedule.go / automation_target.go：Schedule / SessionTarget / DeliveryTarget / Source 及 Validate/Normalized。
//   - automation_heartbeat.go：HeartbeatConfig / HeartbeatWakeInput 等 heartbeat 协议。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package protocol
