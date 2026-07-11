# Agent Event Handlers

- `handler-scope.ts` 提供当前 Session 事件守卫，事件族不得重复作用域判断。
- `agent-message-event-handlers.ts` 处理消息快照与流式载荷。
- `resync-event-handlers.ts` 处理 Session/Room 缺口重拉与重新订阅。
- `permission-event-handlers.ts` 处理权限请求的新增与移除。
- `session-event-handlers.ts` 处理错误、运行状态、队列、Goal、轮次和消息状态。
- `session-event-data.ts` 解码 Session、队列、轮次和 ACK 载荷，不承载副作用。
- `scope-event-handlers.ts` 处理 Agent runtime、Workspace 与 Room 级事件。
- 每个文件导出事件类型到处理器的纯映射；路由器显式注册并拒绝重复事件所有权。
- Handler 不得直接断言生成信封的 `data`；复杂载荷先通过所属解码函数。
