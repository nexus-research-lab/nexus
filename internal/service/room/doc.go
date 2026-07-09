// Package room 编排房间、对话和房间内实时运行。
//
// L2 | 父级: internal/service（L1 见 AGENTS.md）
//
// 文件按职责前缀分组：
//   - service_*：房间、成员、会话查询和服务装配。
//   - chat_* / directed_message_* / public_*：公区消息、定向消息和唤醒逻辑。
//   - execution_* / interrupt / slot_* / round_*：实时 round、slot 和中断状态机。
//   - input_queue_* / guidance_*：输入队列与运行时补充上下文。
//   - goal_*：Room 实时运行里的 goal 集成。
//   - privateview/：Agent 私域 thread/event 投影。
//   - runtimepolicy/：Room MCP 工具白名单和权限策略。
//
// 这个包共享 Service、RealtimeService 和 active round 状态；拆子包前先确认不会只换来更多导出类型。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package room
