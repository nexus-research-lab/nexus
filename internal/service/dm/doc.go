// Package dm 编排 DM（单 Agent 私聊）会话的写入、运行时轮次与队列/中断/续跑。
//
// L2 | 父级: internal/service（L1 见 AGENTS.md）
//
// 成员清单：
//   - service.go / request.go / guidance_input.go / round*.go：写请求阶段状态、直接或 queue/guide 物化的首条 Room DM 用户消息消费 conversation draft，与运行时轮次编排。
//   - input_queue.go / running_input.go / guidance_input.go / interrupt.go：durable 幂等受理、admission 暂时失败时保留并允许原请求重试恢复、先 ACK 后异步启动与下一轮队列、hook applied ACK 后消费引导、错过 hook 的接力与中断。
//   - goal_command.go：UI set_goal 与 `/goal` 共用的 Goal 写入、完成态控制 marker、会话 started/title 基础事实；不创建普通模型 round。
//   - goal_continuation.go / goal_runtime.go / goal_completion_receipt.go：含旧显式 Goal 确定性 reservation 恢复的 exact Goal/revision/Execution 续跑启动 claim、上下文、消费后 revision adoption、live scope create guard、parent terminal ledger、child lifecycle evidence、fenced 结算与最终回复完成收据。
//   - history.go / fork.go / transient_fork.go / transient_session.go / rewrite.go / title.go / transport_context.go / execution_context.go / execution_evidence.go / context_usage.go：历史、同 Agent 延迟物化的 conversation fork、仍需继承上下文的嵌入场景所用且跳过 overlay-only/局部投影的隐藏 fork、不继承源 transcript 且供 WorkGraph 编辑/保存使用的目录隐藏专用 Session、provider-specific 边界兼容、先提交 Room SQL 再投影 workspace 的 SDK session/fingerprint 同步、区分 transcript rewrite 与启动失败 overlay-only rerun 的重写、标题、Goal/Automation/上一轮失败恢复上下文、active-paired IM 的无标识只读 transport 上下文、每轮权威 WorkGraph 上下文与 compact 持久证据，以及每轮终态上下文占用快照持久化。
//   - attachments.go / broadcast.go / external_reply.go / command_input.go / connector_context.go：附件、广播、外部回复、Connector 配置与当前 Session 选择的可信动态上下文，以及把 DM 查看权限与仅 WebSocket 可用的 host Slash 权限分离的无副作用校验。
//   - quota.go / subagent_task.go / runtime_client.go / runtime_settings_preparation.go：账号额度门禁与 Goal 限制投影、子任务、带 Execution-aware Agent hook 装配，以及 exact 临时 Session 的受限 system prompt / tool policy，
//     Connector 选择提交后按 Session latest-wins 预备工具面 fork，真实输入仍同步兜底；工具面变化时从旧 transcript 幂等 fork 新物理 Session，并签发 nexuscfg 与 Agent-facing nexus command 的 physical-round capability；active-paired
//     外部私聊复用同 Agent Skill，provider init/fork 后的 SDK Session identity 动态写回同轮 command context，受控 Automation 执行可覆盖创建时工具快照且 CLI 只读绑定当前 job/run。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package dm
