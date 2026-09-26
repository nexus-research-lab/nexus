# Conversation API

- `team-node-api.ts` 访问本机 `/team-node` 授权状态/命令；不使用被 Desktop 代理到远程的 `/team`，Browser 不接触设备凭据。
- Node DTO 仅投影执行开关、任务状态和本地会话身份；`enable_execution` 默认 false，只由用户开启动作置 true。

- `session-api.ts` 负责 Session 列表、消息历史、轮次索引、历史外部 IM Session 删除、当前 Session 运行时覆盖与本机目录请求；响应转换统一归 `session-api-model.ts`。
- `team-api.ts` 负责 Organization 隔离的在线 Room、真人消息、快照、difference 与同源 WSS URL；Browser 不接触 Relay token，也不能提交 Organization ID。
- `message-page-model.ts` 统一 Room 与 Session 消息分页的查询序列化和响应缺省值，不允许各 API 重复解释同一分页协议。
- Room 变更投影、读取和写命令分别归 `room-api-model.ts`、`room-resource-api.ts` 与 `room-command-api.ts`。
- Room 目录与 Session 运行时设置失效通知分别归 `lib/conversation/room-directory-events.ts`、`lib/conversation/session-runtime-settings-events.ts`，API 文件不得持有浏览器订阅。
- Goal 与子智能体任务按会话作用域独立维护协议文件；创建 Goal 统一进入 Composer 的 `set_goal` 控制命令，REST 客户端只保留已使用的读取与生命周期操作。
- `execution-api.ts` 承载最新/历史 managed WorkGraph、owner Workflow 目录与 durable Draft 编辑/版本选择；命名图确认保存直接提交后端事务；成功响应返回已持久化工作图，关闭编辑 UI 不删除隐藏 Session。
- 编辑器启动的元信息字段只表达显式表单修改；省略字段时恢复服务端当前值，不复制客户端旧草图快照。
- Agent CRUD 和 workspace 操作统一归 `agent/`。
- API 客户端不得读取 Store；缺失 Agent 的恢复由 Navigation Feature 负责。

- 保存确认携带用户可见 head/selected revision；save-state 是只读核对入口，返回当前草稿和实际生效命令。Apply 同时传入 head/selected revision，不能只校验 head。

- team-api.ts 的 PUT read-state 仅携带 message_seq 与 stream_epoch，身份由 Gateway 派生；不创建客户端读取真相或幂等发件箱。

- team-api 的成员读取使用 next_member_cursor 和原版本/世代续页；team-api.test.ts 验证续页栅栏与投递查询 100 条分批。历史投递按 message_id 请求，空列表不发请求。
