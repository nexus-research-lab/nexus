// Package realtime 编排 Room 的聊天、round、queue 与 Agent runtime。
// Room runtime 语义见 docs/specs/room-collaboration-spec.md；
// room-collaboration-mechanism*.md 仅是 Skill 作者指南。WorkGraph 语义见
// execution specs。本头部只保留代码地图。
//
// L2 | 父级: internal/service/room（L1 见 AGENTS.md）
//
// 成员地图：
//   - service.go / member_participation.go：装配、广播与成员 participation gate。
//   - chat.go / attachments.go / state.go / conversation_rounds.go：输入、附件、
//     conversation 派发锁和 round/slot 状态。
//   - execution*.go / runtime_policy.go / recovery_context.go / interrupt.go：
//     Work/Review binding admission、Room dispatch、Attempt 终态、Goal authority、
//     runtime context、取消与 usage 对账。
//   - input_queue*.go / guidance_input.go：持久输入队列、派发与运行中 guidance。
//   - directed_message.go / public_*.go：公开消息、mention conversation handoff、
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
