# hooks/agent/reliability/

L4 | 父级: ../CLAUDE.md

负责 DM/Room 共用的 Conversation reliability 纯状态机及 React 适配。传输恢复、provider retry 和结构化会话故障是三条独立轨道；界面只投影稳定产品语义，不保存或展示 raw error、provider 详情、request ID 与 round ID。

故障必须以精确 Session 为边界；带 `client_request_id`、`round_id` 或 `agent_round_id` 的故障只能由同一关联身份的正向证据撤销。Room 成员级故障只影响该 Agent round，不得提升为全 Room 故障；DM durable reconcile 可恢复最后一个助手轮次的结构化终态。

[PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
