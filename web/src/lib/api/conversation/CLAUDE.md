# Conversation API

- `session-api.ts` 只负责私聊 Session、消息历史与轮次索引请求；响应转换统一归 `session-api-model.ts`。
- `message-page-model.ts` 统一 Room 与 Session 消息分页的查询序列化和响应缺省值，不允许各 API 重复解释同一分页协议。
- Room 变更投影、读取和写命令分别归 `room-api-model.ts`、`room-resource-api.ts` 与 `room-command-api.ts`。
- 目录失效通知归 `lib/conversation/room-directory-events.ts`，API 文件不得持有浏览器订阅。
- Goal 与子智能体任务按会话作用域独立维护协议文件。
- Agent CRUD 和 workspace 操作统一归 `agent/`。
- API 客户端不得读取 Store；缺失 Agent 的恢复由 Navigation Feature 负责。
