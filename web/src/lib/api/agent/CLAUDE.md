# Agent API

- `agent-api.ts` 负责 Agent CRUD、双向好友通讯录、创建行为模板读取与 workspace 文件操作；带 `creation_request_id` 的创建只能通过 exact owner-scoped 回执对账，不得复用 HTTP `X-Request-ID`、Agent 名称或目录时间。文件读取携带内容 revision，并可选以 `expected_revision` 条件写入。`agent-communication-api.ts` 负责 owner 以选中 Agent 视角打开直聊通道和发送消息；`profile_template` 只属于创建协议，不得复用为 Agent 摘要。
- `agent-transform.ts` 保存 Agent 协议到前端模型的单一转换规则；`business_tags` 是管理目录元数据，`vibe_tags` 是运行时风格画像，两者不得互换。
- `private-domain-api.ts` 和 `memory-api.ts` 分别负责可游标翻页的私域记录与 workspace 记忆投影。
- 会话与 Room 请求不属于该目录，统一归 `conversation/`。
