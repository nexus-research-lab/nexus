# Agent Event Handlers

- `handler-scope.ts` 提供当前 Session 事件守卫，事件族不得重复作用域判断。
- `agent-message-event-handlers.ts` 处理消息快照与流式载荷；完整 snapshot 写入前同步 flush 已排队 stream patch，保持同一 WebSocket 的到达顺序，并在 snapshot 提交后通知 transport 于下一帧清理 live reveal 标记。只有 durable 消息可跨当前 Session 进入后台缓存，transient host 通知只保留在触发它的当前时间线。
- Room realtime user 消息通过消息集合模型消费 `client_message_id`，Handler 不得先追加 canonical 节点再等待 ACK 删除 optimistic 节点。
- client 请求对应的 `chat_ack` 负责 correlation，并可用 `user_message_delivery_mode` 明确保留 host 指令的 transient 用户投影；字段只接受生成协议定义的 delivery mode。server-initiated public wake 的 pending 事件没有 client/user correlation，但它是可按 Room 序号重放的 durable 状态，携带非空 slot 与稳定 root 时必须接收。
- 重连的 authoritative pending snapshot 即使为空且聚合 `round_id` 为空也必须接收并清理陈旧槽位；当前 conversation 的 `snapshot_room_seq` 必须先替换 transport 游标（允许服务端重启后回到新代次的小序号），使快照之前的重放事件无法复活 execution，其他 conversation 的迟到快照不得改写游标。并发多 root 快照由每个 slot 自己的 `round_id` 定位。
- `resync-event-handlers.ts` 统一推进 Session/Room 游标，并在缺口重拉完成且连接有效时重新订阅。
- `permission/` 分离权限事件的未知载荷解码与当前 Session 状态增删；重复 request 快照必须原位替换，禁止改变首次到达顺序导致 Composer 当前确认跳项。
- `session-event-handlers.ts` 处理错误、Session/runtime 状态、队列、Goal、轮次和消息状态；round status 只推进生命周期，错误文案由 durable result 或显式 error 事件单点投影。
- `session-event-data.ts` 解码 Session/runtime 状态、队列、轮次和 chat / input_queue ACK 载荷，不承载副作用。
- `scope-event-handlers.ts` 处理 Agent runtime、Workspace 与 Room 级事件。
- 每个文件导出事件类型到处理器的纯映射；路由器显式注册并拒绝重复事件所有权。
- Handler 不得直接断言生成信封的 `data`；复杂载荷先通过所属解码函数，字段回退和协议默认值不得进入副作用处理器。
- 解码器以协议字段集合批量校验共同身份，枚举字段通过集合读取原语收窄，不维护逐字段布尔链或散落断言。
