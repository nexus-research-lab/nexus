# hooks/agent/actions/

L4 | 父级: ../CLAUDE.md

负责把用户意图转换为协议消息，并统一发送、ACK、超时和失败收口。这里不维护会话历史或 WebSocket 订阅状态。

- `use-pending-request-acks.ts` 统一 chat / input_queue 的请求 ACK 注册、乱序到达和取消语义。
- `use-request-ack-failure.ts` 统一 ACK 超时重连和发送中请求清理；仅 chat 失败回滚 optimistic 用户消息。
- `input-queue-actions.ts` 为 enqueue 发送稳定 `client_message_id` 与逐次 `client_request_id`，成功 ACK 前不算提交完成。
- `conversation-action-context.ts` 通过有序守卫表固定缺失 Session、非法 Session 和断连的失败优先级，成功身份单独投影。
- `conversation-control-actions.ts` 先生成权限响应计划，再由动作边界执行发送与状态变更。
- 普通聊天发送不清理待确认权限；权限只由明确决策、中断或对应 round 终态收口。
- 公共动作保持稳定，执行时读取最新会话上下文，不因消息流更新重建整组命令。

[PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
