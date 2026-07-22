// Package realtime 编排 Room 的实时聊天、round、queue 与 Agent runtime。
//
// L2 | 父级: internal/service/room（L1 见上级 AGENTS.md）
//
// 文件按职责前缀分组：
//   - service.go / context.go：实时服务装配与持久化 Room 上下文适配。
//   - chat*.go / active_slot_delivery.go：输入受理、目标解析和活跃 slot 投递。
//   - conversation_rounds.go / round_*.go / slot_*.go / dispatch_coordinator.go：conversation 状态、round 生命周期和 slot 状态。
//   - execution*.go / interrupt.go / subagent_idle_drain.go：runtime 执行、终态和中断。
//   - input_queue*.go / guidance_input.go：持久化输入队列和运行中引导。
//   - directed_message*.go / public_*.go / message_causality.go：Room 协作消息、handoff 和唤醒。
//   - goal_*.go：Room 与 Goal runtime 的适配。
//   - runtimepolicy/：Room runtime 工具和权限策略。
//
// queue、public wake、Goal continuation 与 execution 共享 conversation 状态；
// 派发顺序锁属于对应 conversation，禁止回退为 Service 级总锁。slot 的可变
// 数据统一进入 roomSlotMutableState，再由 runtime、goal、cursor、delivery
// 等子状态独立同步；只有无 I/O、无 runtime 状态的纯策略才下沉到子包。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级 room/doc.go 与上级 AGENTS.md
package realtime
