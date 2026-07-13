// Package room 编排房间、对话和房间内实时运行。
//
// L2 | 父级: internal/service（L1 见 AGENTS.md）
//
// 文件按职责前缀分组：
//   - service.go / crud.go / conversation_crud.go / member.go / query.go：服务装配与房间数据操作。
//   - chat.go / chat_*：Room 输入受理、Agent 目录、标题生成、投递策略与 round 构造。
//   - directed_message_* / public_*：定向消息、公区消息和唤醒逻辑。
//   - execution.go：slot 生命周期、round mapper、runtime 消息、事件投递与 usage 写入。
//   - execution_runtime_*：runtime prompt、选项、连接、session 恢复与诊断。
//   - execution_slot_status.go / interrupt / slot_* / round_*：完成结算、状态、中断与 round 状态机。
//   - input_queue.go / input_queue_* / guidance_input.go：输入队列、round 终态确定性接力与分阶段运行时补充上下文。
//   - attachments.go：Room 公共附件上传、归一化与运行时路径解析。
//   - goal_*：Room 实时运行里的 goal 集成。
//   - privateview/：Agent 私域 thread/event 投影。
//   - runtimepolicy/：Room MCP 工具白名单和权限策略。
//
// 这个包共享 Service、RealtimeService 和 active round 状态；拆子包前先确认不会只换来更多导出类型。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package room
