// Package dm 编排 DM（单 Agent 私聊）会话的写入、运行时轮次与队列/中断/续跑。
//
// L2 | 父级: internal/service（L1 见 AGENTS.md）
//
// 成员清单：
//   - service.go / request.go / round*.go：写请求阶段状态与运行时轮次编排。
//   - input_queue.go / running_input.go / interrupt.go：队列、运行中输入、中断。
//   - goal_continuation.go / goal_context.go / goal_runtime.go：Goal 续跑、上下文与进展结算。
//   - history.go / rewrite.go / title.go：历史、SDK session/fingerprint 同步、重写、标题。
//   - attachments.go / guidance_input.go / broadcast.go / external_reply.go：附件、引导输入、广播、外部回复。
//   - quota.go / subagent_task.go / runtime_client.go：额度、子任务、运行时客户端。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package dm
