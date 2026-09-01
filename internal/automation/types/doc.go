// Package types 是 automation 特性域内共享的类型词汇：定时任务/heartbeat 的
// 模型、枚举、输入与其 Validate/Normalized（消息是否良构）。
// 不叫 protocol 是为避免与顶层 internal/protocol 基名撞车、逼所有双引用文件起别名。
//
// L2 | 父级: internal/automation（L1 见 AGENTS.md）
//
// 与顶层 internal/protocol 的区别：这些类型只在 automation 簇内流转、不跨前端 codegen，
// 故按特性域下沉；顶层 protocol 才是跨 HTTP/WS/前端/运行时的真相源。父域 automation
// 的行为代码（调度计算、会话解析、执行观测等）依赖本包，反向不依赖。
//
// 成员清单：
//   - automation.go：调度/目标/唤醒/投递/执行/来源/运行状态等枚举常量，以及日报输入的 typed 错误。
//   - task.go / report.go：带 configuration_version、durable deletion/review 状态、不可见 delivery grant 的 ScheduledTask，以及携带人工 client request 对账身份的 ScheduledTaskRun、日报等对外视图。
//   - permission.go：SDK 权限模式、工具 allow/deny 创建快照的任务 grant、run 阻塞、持久审批请求与决策协议。
//   - input.go：CreateJobInput（含可选创建幂等键）/ UpdateJobInput（含目标 Agent 重绑）及校验、归一。
//   - schedule.go / target.go：Schedule / SessionTarget / DeliveryTarget / Source 及 Validate/Normalized。
//   - task_compatibility.go / task_session_binding.go：历史投递线格式与合成收件箱只读兼容，以及 Session 删除后停用、重绑、active pairing 校验错误和恢复的任务级生命周期。
//   - delivery_scope.go：普通 Agent 的自身/当前 Room/当前外部会话键投递目标边界。
//   - heartbeat.go：带 configuration_version 的 HeartbeatConfig / HeartbeatWakeInput，以及 durable outbox claim 身份的内部 SystemEvent。
//   - command.go：Automation Skill、round-scoped nexus.command MCP、宿主 runtime 与 service 共用的 contract/inspect/plan/apply wire。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package types
