// Package dm 编排 DM（单 Agent 私聊）会话的写入、运行时轮次与队列/中断/续跑。
//
// L2 | 父级: internal/service（L1 见 AGENTS.md）
//
// 成员清单：
//   - service.go / service_request.go / service_round*.go：写请求入口与运行时轮次编排。
//   - service_input_queue.go / service_running_input.go / service_interrupt.go：队列、运行中输入、中断。
//   - service_goal_continuation.go / goal_context.go / goal_runtime.go：Goal 续跑与上下文。
//   - service_history.go / service_rewrite.go / service_title.go：历史、重写、标题。
//   - service_attachments.go / service_guidance_input.go / service_broadcast.go / service_external_reply.go：附件、引导输入、广播、外部回复。
//   - service_quota.go / service_subagent_task.go / service_runtime_client.go：额度、子任务、运行时客户端。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package dm
