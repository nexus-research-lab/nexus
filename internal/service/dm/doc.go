// Package dm 编排 DM（单 Agent 私聊）会话的写入、运行时轮次与队列/中断/续跑。
//
// L2 | 父级: internal/service（L1 见 AGENTS.md）
//
// 成员清单：
//   - service.go / request.go / guidance_input.go / round*.go：写请求阶段状态、直接或 queue/guide 物化的首条 Room DM 用户消息消费 conversation draft，与运行时轮次编排。
//   - input_queue.go / running_input.go / guidance_input.go / interrupt.go：
//     durable 幂等入队与下一轮队列、hook applied ACK 后消费引导、错过 hook 的接力与中断。
//   - goal_continuation.go / goal_context.go / goal_runtime.go：Goal 续跑启动 claim、上下文、消费后 revision adoption、live scope create guard、parent terminal ledger、child lifecycle evidence 与 fenced 结算。
//   - history.go / rewrite.go / title.go / recovery_context.go：历史、SDK session/fingerprint 同步、重写、标题与上一轮失败恢复上下文。
//   - attachments.go / broadcast.go / external_reply.go / context_usage.go：
//     附件、广播、外部回复与每轮终态上下文占用快照持久化。
//   - quota.go / subagent_task.go / runtime_client.go：账号额度门禁与 Goal 限制投影、子任务、运行时客户端。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package dm
