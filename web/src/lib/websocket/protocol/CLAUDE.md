# WebSocket 协议边界

- `event-message.ts` 只校验生成协议的通用信封，不解释业务事件数据。
- `resource-change.ts` 只判断 `resource_changed` 的 `data.resource` 与可选 `data.agent_ids` 是否命中当前消费者；事件不携带资源内容。
- 具体 `data` 由对应领域消费者继续解码，禁止在连接层恢复 `any`。
