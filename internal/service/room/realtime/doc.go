// Package realtime 编排 Room 的实时聊天、round、queue 与 Agent runtime。
//
// L2 | 父级: internal/service/room（L1 见上级 AGENTS.md）
//
// 文件按职责前缀分组：
//   - service.go / context.go：实时服务装配与持久化 Room 上下文适配。
//   - chat*.go / active_slot_delivery.go：输入受理、目标解析和活跃 slot 投递。
//   - conversation_rounds.go / round_*.go / slot_*.go / dispatch_coordinator.go：conversation 状态、round 生命周期和 slot 状态。
//   - execution*.go / runtime_policy.go / interrupt.go / subagent_idle_drain.go：runtime 执行、Room 工具权限策略、终态和中断。
//   - input_queue*.go / guidance_input.go：持久化输入队列和运行中引导。
//   - directed_message*.go / public_*.go / message_causality.go：Room 协作消息、handoff 和唤醒。
//   - goal_*.go：Room 与 Goal runtime 的适配。
//
// 测试按 package 边界和行为聚合：realtime 内部状态、Goal、协作测试分别归入
// state_test.go、goal_*.go、collaboration_test.go；realtime_test 的交付、生命周期
// 和共享夹具分别归入 chat_delivery_test.go、lifecycle_test.go、
// test_helpers_test.go、runtime_policy_tools_test.go。queue、guidance、session、directed message 等
// 大场景保持独立，避免把互不相关的夹具和断言堆成测试泥团。
//
// queue、public wake、Goal continuation 与 execution 共享 conversation 状态；
// 派发顺序锁属于对应 conversation，禁止回退为 Service 级总锁。slot 的可变
// 数据统一进入 roomSlotMutableState，再由 runtime、goal、cursor、delivery
// 等子状态独立同步。没有独立调用边界的纯策略就近归入所属业务，不为导出函数或测试
// 便利单独创建子包。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级 room/doc.go 与上级 AGENTS.md
package realtime
