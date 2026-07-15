// Package dm 编排 DM（单 Agent 私聊）会话的写入、运行时轮次与队列/中断/续跑。
//
// L2 | 父级: internal/service（L1 见 AGENTS.md）
//
// 成员清单：
//   - service.go / request.go / round*.go：写请求阶段状态与运行时轮次编排。
//   - input_queue.go / running_input.go / guidance_input.go / interrupt.go：durable 下一轮队列、hook applied ACK 后消费引导、错过 hook 的接力与中断。
//   - goal_continuation.go / goal_context.go / goal_runtime.go：Goal 续跑启动 claim、上下文、消费后 revision adoption 与 fenced 结算。
//   - history.go / rewrite.go / title.go：历史、SDK session/fingerprint 同步、重写、标题。
//   - attachments.go / broadcast.go / external_reply.go：附件、广播、外部回复。
//   - quota.go / subagent_task.go / runtime_client.go：额度、子任务、运行时客户端。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package dm
