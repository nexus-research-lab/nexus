# Room Round

- `round-agent-model.ts` 负责一轮内按 `agent_round_id` 对齐 Agent 消息、结果和占位槽；同 Agent 的多次执行不得坍缩。
- `round-thread-model.ts` 负责从根轮次投影精确 Agent 执行轮的 Thread 消息。
- Room 主 Feed 与 Thread 必须消费同一 Agent 聚合模型，不各自推导执行状态。
- 结果状态映射与消息状态优先级由数据表定义；合成 result 只在 canonical assistant 缺席时保留。
- 本目录只放纯模型，不读取 Store、不调用 API、不持有 React 状态。
