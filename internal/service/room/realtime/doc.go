// Package realtime 编排 Room 的实时聊天、round、queue 与 Agent runtime。
//
// L2 | 父级: internal/service/room（L1 见上级 AGENTS.md）
//
// 文件按业务内聚分组（一个业务一个文件，不按机械行数拆分）：
//   - service.go：服务装配、依赖接口、事件广播与持久化 Room 上下文适配。
//   - chat.go / attachments.go：输入受理、目标解析、共享消息持久化和活跃 slot 投递；附件归一化被 chat/execution/guidance 共用。
//   - state.go / conversation_rounds.go：round/slot 内存状态模型；conversation 级注册表、派发顺序锁与 round 注册。
//   - execution.go / execution_runtime.go / runtime_policy.go / execution_slot_status.go / interrupt.go / subagent_idle_drain.go：slot 执行主链、runtime 选项、Room 工具权限策略、连接诊断、终态同步和中断。
//   - input_queue.go / input_queue_dispatch.go / guidance_input.go：持久化输入队列（受理/上下文/存储）、队列派发和运行中引导。
//   - directed_message.go / public_message.go / public_mentions.go / public_handoff.go / public_context.go：Room 协作消息（含唤醒调度与 timer 注册表）、公开消息因果、mention 唤醒、handoff 标注/回收和 slot 可见上下文。
//   - goal_runtime.go / goal_continuation.go：Room 与 Goal runtime 的适配（用量/取消/完成度门槛）与 Goal 接力派发。
//
// 测试按 package 边界和行为聚合：realtime 白盒测试归入 state_test.go（状态/广播/派发锁）、
// collaboration_test.go（协作与路由）、goal_runtime_test.go / goal_continuation_test.go、
// runtime_policy_tools_test.go（Room 工具权限策略）、
// guidance_input_test.go；realtime_test 黑盒测试归入 chat_delivery_test.go（交付与 mention 唤醒）、
// chat_runtime_test.go、lifecycle_test.go、runtime_policy_test.go（含 Goal 派发竞态）和共享夹具
// test_helpers_test.go。queue、guidance、session、directed message 等大场景保持独立，
// 避免把互不相关的夹具和断言堆成测试泥团。不为其它包已覆盖的函数写跨包重复测试。
//
// queue、public wake、Goal continuation 与 execution 共享 conversation 状态；
// 派发顺序锁属于对应 conversation，禁止回退为 Service 级总锁。slot 的可变
// 数据统一进入 roomSlotMutableState，再由 runtime、goal、cursor、delivery
// 等子状态独立同步。没有独立调用边界的纯策略就近归入所属业务，不为导出函数或测试
// 便利单独创建子包。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级 room/doc.go 与上级 AGENTS.md
package realtime
