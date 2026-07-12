# hooks/agent/actions/

L4 | 父级: ../CLAUDE.md

负责把用户意图转换为协议消息，并统一发送、ACK、超时和失败收口。这里不维护会话历史或 WebSocket 订阅状态。

- `use-chat-ack-failure.ts` 统一 ACK 超时重连、发送中请求清理和 optimistic 用户消息回滚。
- `conversation-control-actions.ts` 先生成权限响应计划，再由动作边界执行发送与状态变更。
- 公共动作保持稳定，执行时读取最新会话上下文，不因消息流更新重建整组命令。

[PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
