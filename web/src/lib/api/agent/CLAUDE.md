# Agent API

- `agent-api.ts` 负责 Agent CRUD、双向好友通讯录、创建行为模板读取与 workspace 文件操作；`agent-communication-api.ts` 负责 owner 以选中 Agent 视角打开直聊通道和发送消息；`profile_template` 只属于创建协议，不得复用为 Agent 摘要。
- `agent-transform.ts` 保存 Agent 协议到前端模型的单一转换规则。
- `private-domain-api.ts` 和 `memory-api.ts` 分别负责可游标翻页的私域记录与 workspace 记忆投影。
- 会话与 Room 请求不属于该目录，统一归 `conversation/`。
