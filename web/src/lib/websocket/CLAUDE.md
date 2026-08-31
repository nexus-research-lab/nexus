# WebSocket

- `socket-policy.ts` 定义有效配置、共享通道身份、离线队列白名单、退避和僵尸连接判定。
- `socket-heartbeat.ts` 独占心跳 interval 与 timeout；连接客户端不得直接维护心跳计时器。
- `socket-client.ts` 只维护单条连接、重连生命周期和待发送控制消息。
- `session-binding-leases.ts` 负责共享物理连接上的 Session 逻辑租约、最后消费者解绑与重连重放。
- `request-transport-leases.ts` 按 exact `client_request_id` 持有已发送请求的原 Session binding，在 React subscriber 全部卸载后继续收取 raw ACK/error；重复 acquire 不获得 release 权限，终态、硬超时或显式取消精确释放。
- `shared-socket-channel.ts` 负责多订阅者广播、Session/请求租约接入、共享连接注册和延迟释放；请求租约仍存在时不得关闭零 UI subscriber 的物理连接。owner reset 必须同步摘除并断开全部旧通道，静默释放请求/Session/subscriber lease；相同配置的下一次 acquire 必须建立新握手，旧 hook cleanup 不能删除新通道。
- `use-socket.ts` 把 React 生命周期接到共享通道，并以 auth owner generation 栅栏事件、状态、发送和请求回调；清理完成的 generation 发布会使仍挂载的 Hook 重新 acquire 新通道，不依赖 React 路由必须卸载，也不复制连接状态机。
- `protocol/event-message.ts` 只校验生成协议的通用信封，业务 `data` 留给领域解码器。
- 业务消息禁止离线排队；只有策略表中明确列出的幂等控制消息可以等待重连。
- 共享通道身份必须包含规范化后的完整有效配置，禁止由首个订阅者静默决定连接策略。
- Session 消费者必须通过共享通道租约 bind；cleanup 只释放自己的租约，禁止直接发送可能解绑其他消费者的 `unbind_session`。
- 请求 ACK 所有权只取本地 mint 的 exact `client_request_id`，与当前路由无关；raw 请求收口先于页面广播，foreign ACK 和重复 cleanup 都必须 no-op。
- owner scope reset 后，旧 React subscriber 即使尚未 cleanup，也不得接收 raw event/state、继续发送或执行请求租约回调；generation 不进入连接 key、协议或业务 identity，也不得触发额外连接或读取。
