// Package room 编排房间、对话和房间内实时运行。
//
// L2 | 父级: internal/service（L1 见 AGENTS.md）
//
// 文件按职责前缀分组：
//   - service.go / crud.go / conversation_crud.go / member.go / query.go：服务装配与房间数据操作。
//   - chat.go / chat_*：Room 输入受理、最近活跃 root slot 默认目标、Agent 目录、标题生成、投递策略与 round 构造。
//   - directed_message_* / message_causality.go / wake_timer_registry.go / public_*：定向消息、因果链、busy 公区 @ 的 guide-first 持久化唤醒、计时调度和公区消息。
//   - execution.go：slot 生命周期、round mapper、runtime 消息、事件投递与 usage 写入。
//   - execution_runtime_*：runtime prompt、选项、连接、session 恢复与诊断。
//   - execution_slot_status.go / interrupt / slot_* / round_*：完成结算、状态、中断与 round 状态机。
//   - conversation_rounds.go：按 conversation 分片保存 active round、public wake 与 pending guidance；不再由 service 级大锁包住所有 Room。
//   - dispatch_coordinator.go：按 conversation 串行化 queue/wake/continuation/round finish 的交接，不阻塞其他 conversation 的派发。
//   - round_state.go / slot_state.go：active slot 的稳定身份与 runtime、Goal、cursor、delivery 四类可变状态分离。
//   - input_queue.go / input_queue_* / guidance_input.go：跨成员 durable 幂等受理、按 Agent 串行队列、round 终态确定性接力、逐批 applied ACK、transport/错过 hook 回退、运行时补充上下文与已消费引导归组。
//   - public_context.go：按 runtime resume 状态装配预算化 anchor/delta，并提交真实消费 cursor。
//   - attachments.go：Room 公共附件上传、归一化与运行时路径解析。
//   - goal_*：Room 实时运行里的 Goal lead/成员目录对齐、逐 slot objective steering、消费后 revision adoption/fencing、续跑启动 claim 与 active/queued/wake completion readiness。
//   - privateview/：Agent 私域 thread/event 投影。
//   - runtimepolicy/：Room MCP 工具白名单和权限策略。
//
// 这个包共享 Service、RealtimeService 和 active round 状态；拆子包前先确认不会只换来更多导出类型。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package room
