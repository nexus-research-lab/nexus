# WebSocket

- `socket-policy.ts` 定义有效配置、共享通道身份、离线队列白名单、退避和僵尸连接判定。
- `socket-heartbeat.ts` 独占心跳 interval 与 timeout；连接客户端不得直接维护心跳计时器。
- `socket-client.ts` 只维护单条连接、重连生命周期和待发送控制消息。
- `session-binding-leases.ts` 负责共享物理连接上的 Session 逻辑租约、最后消费者解绑与重连重放。
- `request-transport-leases.ts` 按 exact `client_request_id` 持有已发送请求的原 Session binding，在 React subscriber 全部卸载后继续收取 raw ACK/error；重复 acquire 不获得 release 权限，终态、硬超时或显式取消精确释放。
- `shared-socket-channel.ts` 负责多订阅者广播、Session/请求租约接入、共享连接注册和延迟释放；请求租约仍存在时不得关闭零 UI subscriber 的物理连接。
- `use-socket.ts` 只把 React 生命周期接到共享通道，不复制连接状态机。
- `protocol/event-message.ts` 只校验生成协议的通用信封，业务 `data` 留给领域解码器。
- 业务消息禁止离线排队；只有策略表中明确列出的幂等控制消息可以等待重连。
- 共享通道身份必须包含规范化后的完整有效配置，禁止由首个订阅者静默决定连接策略。
- Session 消费者必须通过共享通道租约 bind；cleanup 只释放自己的租约，禁止直接发送可能解绑其他消费者的 `unbind_session`。
- 请求 ACK 所有权只取本地 mint 的 exact `client_request_id`，与当前路由无关；raw 请求收口先于页面广播，foreign ACK 和重复 cleanup 都必须 no-op。
