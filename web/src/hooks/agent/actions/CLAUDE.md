# hooks/agent/actions/

L4 | 父级: ../CLAUDE.md

负责把用户意图转换为协议消息，并统一发送、ACK、超时和失败收口。这里不维护会话历史或 WebSocket 订阅状态。

- `use-pending-request-acks.ts` 统一 chat / set_goal / input_queue / interrupt 的请求 ACK 注册、所有权、乱序到达和取消语义；普通 Session 切换不取消已发送请求，只有本 Hook 预先追踪的 `client_request_id` 才能被事件收口，避免共享 Socket 的其他订阅者吞入外来 ACK。
- `use-request-ack-failure.ts` 统一 ACK 超时后的正向证据恢复与重连；超时和快照缺失只能得到“状态未知”，不得据此回滚 chat optimistic 用户消息，只有后端显式拒绝才可清理；精确 interrupt 超时则恢复可重试停止状态。
- `input-queue-actions.ts` 为 enqueue 发送稳定 `client_message_id` 与逐次 `client_request_id`，成功 ACK 前不算提交完成。
- `conversation-goal-actions.ts` 把 Composer Goal 模式发送为独立 `set_goal`，并生成 canonical `/goal …` optimistic 控制记录；文本 `/goal` 的普通发送动作只做同 subtype 即时投影，是否命中仍由后端 host registry 决定。
- `conversation-action-context.ts` 通过有序守卫表固定缺失 Session、非法 Session 和断连的失败优先级，成功身份单独投影。
- `conversation-control-actions.ts` 先生成权限响应计划，再由动作边界执行发送与状态变更。
- 权限响应成功发送后必须先确认精确 Room execution tombstone，再移除 pending permission，保证 shell 连续且表单不能重复提交。
- 普通聊天发送不清理待确认权限；权限只由明确决策、中断或对应 round 终态收口。精确 Agent 中断携带独立 `client_request_id`，发送后进入 `stopping`，只由对应 terminal event 或 `interrupt_ack` 幂等收口；发送/ACK 失败必须恢复可重试状态，且不能影响其他 Agent 的权限与执行。
- 公共动作保持稳定，执行时读取最新会话上下文，不因消息流更新重建整组命令。

[PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
