# WebSocket

- `socket-policy.ts` 定义有效配置、共享通道身份、离线队列白名单、退避和僵尸连接判定。
- `socket-heartbeat.ts` 独占心跳 interval 与 timeout；连接客户端不得直接维护心跳计时器。
- `socket-client.ts` 只维护单条连接、重连生命周期和待发送控制消息。
- `shared-socket-channel.ts` 负责多订阅者广播、共享连接注册和延迟释放。
- `use-socket.ts` 只把 React 生命周期接到共享通道，不复制连接状态机。
- 业务消息禁止离线排队；只有策略表中明确列出的幂等控制消息可以等待重连。
- 共享通道身份必须包含规范化后的完整有效配置，禁止由首个订阅者静默决定连接策略。
