# Agent API

- `agent-api.ts` 负责 Agent CRUD 与 workspace 文件操作。
- `agent-transform.ts` 保存 Agent 协议到前端模型的单一转换规则。
- `private-domain-api.ts` 和 `memory-api.ts` 分别负责私域记录与 workspace 记忆投影。
- 会话与 Room 请求不属于该目录，统一归 `conversation/`。
