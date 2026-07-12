# Room Round

- `round-agent-model.ts` 负责一轮内 Agent 消息、结果和占位槽的纯聚合。
- `round-thread-model.ts` 负责从根轮次投影单 Agent Thread 消息。
- Room 主 Feed 与 Thread 必须消费同一 Agent 聚合模型，不各自推导执行状态。
- 结果状态映射与消息状态优先级由数据表定义；新增状态时更新统一规则，不扩散条件分支。
- 本目录只放纯模型，不读取 Store、不调用 API、不持有 React 状态。
