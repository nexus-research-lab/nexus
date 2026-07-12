# Agent Event Handlers

- `handler-scope.ts` 提供当前 Session 事件守卫，事件族不得重复作用域判断。
- `agent-message-event-handlers.ts` 处理消息快照与流式载荷。
- `resync-event-handlers.ts` 统一推进 Session/Room 游标，并在缺口重拉完成且连接有效时重新订阅。
- `permission/` 分离权限事件的未知载荷解码与当前 Session 状态增删。
- `session-event-handlers.ts` 处理错误、运行状态、队列、Goal、轮次和消息状态。
- `session-event-data.ts` 解码 Session、队列、轮次和 ACK 载荷，不承载副作用。
- `scope-event-handlers.ts` 处理 Agent runtime、Workspace 与 Room 级事件。
- 每个文件导出事件类型到处理器的纯映射；路由器显式注册并拒绝重复事件所有权。
- Handler 不得直接断言生成信封的 `data`；复杂载荷先通过所属解码函数，字段回退和协议默认值不得进入副作用处理器。
- 解码器以协议字段集合批量校验共同身份，枚举字段通过集合读取原语收窄，不维护逐字段布尔链或散落断言。
