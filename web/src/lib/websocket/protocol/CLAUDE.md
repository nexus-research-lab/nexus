# WebSocket 协议边界

- `event-message.ts` 只校验生成协议的通用信封，不解释业务事件数据。
- 具体 `data` 由对应领域消费者继续解码，禁止在连接层恢复 `any`。
