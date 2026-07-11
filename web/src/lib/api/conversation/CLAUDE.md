# Conversation API

- `session-api.ts` 负责私聊 Session 与轮次索引，不再使用含混的 `agent-api` 名称。
- Room 纯投影、查询和写命令分别归 `room-api-model.ts`、`room-resource-api.ts` 与 `room-command-api.ts`。
- 目录失效通知归 `lib/conversation/room-directory-events.ts`，API 文件不得持有浏览器订阅。
- Goal 与子智能体任务按会话作用域独立维护协议文件。
- Agent CRUD 和 workspace 操作统一归 `agent/`。
- API 客户端不得读取 Store；缺失 Agent 的恢复由 Navigation Feature 负责。
