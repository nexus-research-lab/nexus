// Package dm 编排 DM（单 Agent 私聊）会话的写入、运行时轮次与队列/中断/续跑。
//
// L2 | 父级: internal/service（L1 见 AGENTS.md）
//
// 成员清单：
//   - service.go / request.go / guidance_input.go / round*.go：写请求阶段状态、直接或 queue/guide 物化的首条 Room DM 用户消息消费 conversation draft，与运行时轮次编排。
//   - input_queue.go / running_input.go / guidance_input.go / interrupt.go：durable 幂等受理、先 ACK 后异步启动与下一轮队列、hook applied ACK 后消费引导、错过 hook 的接力与中断。
//   - goal_command.go：UI set_goal 与 `/goal` 共用的 Goal 写入、完成态控制 marker、会话 started/title 基础事实；不创建普通模型 round。
//   - goal_continuation.go / goal_context.go / goal_runtime.go / goal_completion_receipt.go：含旧显式 Goal 确定性 reservation 恢复的 exact Goal/revision/Execution 续跑启动 claim、上下文、消费后 revision adoption、live scope create guard、parent terminal ledger、child lifecycle evidence、fenced 结算与最终回复完成收据。
//   - history.go / rewrite.go / title.go / recovery_context.go / transport_context.go / execution_context.go / execution_evidence.go / context_usage.go：历史、SDK session/fingerprint 同步、重写、标题、上一轮失败恢复、active-paired IM 的无标识只读 transport 上下文、每轮权威 WorkGraph 上下文与 compact 持久证据，以及每轮终态上下文占用快照持久化。
//   - attachments.go / broadcast.go / external_reply.go / command_input.go：附件、广播、外部回复，以及把 DM 查看权限与仅 WebSocket 可用的 host Slash 权限分离的无副作用校验。
//   - quota.go / subagent_task.go / runtime_client.go：账号额度门禁与 Goal 限制投影、子任务、带 Execution-aware Agent hook 装配，
//     并按 main/self 身份只披露一个配置角色 Skill；active-paired 外部私聊复用同 Agent Skill，受控 Automation 执行可覆盖创建时工具快照。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package dm
