# Room Round

- `round-agent-model.ts` 负责一轮内按 `agent_round_id` 对齐 Agent 消息、结果和占位槽；同 Agent 的多次执行不得坍缩。
- Agent entry 按不可变展示槽优先排序；live slot 与落盘消息都使用 `slot 创建时间 * 1000 + batch index`，既保持同批并发顺序，也让同一 root 的后续 wake 只追加不插队；完成时间只用于 header 语义。
- `round-thread-model.ts` 负责从根轮次投影精确 Agent 执行轮的 Thread 消息。
- Room 主 Feed 与 Thread 必须消费同一 Agent 聚合模型，不各自推导执行状态。
- 结果状态映射与消息状态优先级由数据表定义；合成 result 只在 canonical assistant 缺席时保留。
- 本目录只放纯模型，不读取 Store、不调用 API、不持有 React 状态。
