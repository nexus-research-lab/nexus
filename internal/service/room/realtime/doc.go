// Package realtime 编排 Room 的聊天、round、queue 与 Agent runtime。
// Room runtime 语义见 docs/specs/room-collaboration-spec.md；
// room-collaboration-mechanism*.md 仅是 Skill 作者指南。WorkGraph 语义见
// execution specs。本头部只保留代码地图。
//
// L2 | 父级: internal/service/room（L1 见 AGENTS.md）
//
// 文件按业务内聚分组（一个业务一个文件，不按机械行数拆分）：
//   - service.go / member_participation.go：服务装配、依赖接口、事件广播，以及在 conversation 派发锁内以 Room CAS/authority epoch 持久化并暂停/恢复成员 queue、Goal 与 WorkGraph 调度。
//   - chat.go / attachments.go：输入受理、目标解析、共享消息持久化、直接或 queue/guide 物化用户消息的 draft 消费和活跃 slot 投递；附件归一化被 chat/execution/guidance 共用。
//   - state.go / conversation_rounds.go：round/slot 内存状态模型；conversation 级注册表、派发顺序锁、round 注册，以及可为空、按 slot 携带 root round_id 与 public handoff 关联的权威活跃快照。
//   - execution.go / execution_runtime.go / execution_dispatch.go / execution_review_dispatch.go / execution_cancellation_dispatch.go / execution_attempt_terminal.go / execution_evidence.go / execution_goal_authority.go / runtime_policy.go / recovery_context.go / execution_context.go / execution_slot_status.go / interrupt.go / subagent_idle_drain.go：slot 执行主链、带 current Spec/accepted dependency WorkContract 的 Assignment/Review admission、完整 binding 校验、Goal mutation authority、取消与 Attempt 终态、compact 持久证据、actor-specific WorkGraph 上下文、配置角色 Skill、连接诊断、中断与父子 usage 后台重试。
//     execution_context_usage.go 额外持久化每 Agent 终态上下文占用快照及 Session 元数据，并隔离中断控制值与展示文案；Automation slot 使用任务创建时工具策略覆盖而不回读 Agent 当前 allow/deny；精确 agent_round 中断对自然完成竞态保持幂等。
//   - input_queue.go / input_queue_dispatch.go / guidance_input.go：持久化输入队列（受理/上下文/存储）、队列派发和运行中引导。
//   - directed_message.go / public_*.go：公开消息、服务端分类为 handoff（区别于 queue/internal）的 mention conversation handoff、
//     visible context、携带非授权 Goal revision attribution、分离 target terminal/Goal handback 阶段并严格修复 legacy attribution 的持久 handoff、私域消息
//     两阶段写入修复、host command 幂等、immediate/delayed durable wake 调度与在线重试。
//     @ 不创建 Assignment；正式责任只来自 assign_work。
//   - goal_command.go：服务端验证 lead/协作门槛后写入共享 Goal 与完成态 public 控制记录；不占用普通 Agent slot。
//   - goal_runtime.go / goal_usage_scope_lock.go / goal_continuation.go / goal_completion_receipt.go / quota.go：
//     Goal scope、协作终态回连、continuation、终态、附着最终回复的完成收据和额度适配。
//
// conversation 共享 queue、public wake、Goal continuation 与 Execution slot；锁必须
// 保持 conversation-scoped。每个并行 slot 自带 round_id，聚合 RoundID 只作单 root
// 兼容。测试按 chat、collaboration、Goal、runtime policy 和 delivery 行为分组。
//
// 主要暴露接口：NewService/NewServiceWithFactory、HandleChat/HandleInputQueue/
// HandleDirectedMessage/HandlePublicMessage/HandleInterrupt，以及 orchestration
// 消费的 assignment/review/cancellation delivery、Goal continuation/readiness 与
// Start* 后台恢复入口。Set* 方法只负责应用层依赖注入。
//
// [PROTOCOL]: 行为变化时检查 Room specs、Execution specs、父级 room L2 与 AGENTS.md。
package realtime
