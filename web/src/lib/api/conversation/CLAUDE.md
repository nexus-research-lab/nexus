# Conversation API

- `session-api.ts` 负责 Session 列表、消息历史、轮次索引、历史外部 IM Session 删除、当前 Session 运行时覆盖与本机目录请求；响应转换统一归 `session-api-model.ts`。
- `message-page-model.ts` 统一 Room 与 Session 消息分页的查询序列化和响应缺省值，不允许各 API 重复解释同一分页协议。
- Room 变更投影、读取和写命令分别归 `room-api-model.ts`、`room-resource-api.ts` 与 `room-command-api.ts`。
- Room 目录与 Session 运行时设置失效通知分别归 `lib/conversation/room-directory-events.ts`、`lib/conversation/session-runtime-settings-events.ts`，API 文件不得持有浏览器订阅。
- Goal 与子智能体任务按会话作用域独立维护协议文件；`execution-api.ts` 只承载最新/历史 managed WorkGraph 与 owner Workflow 目录读取、删除，不提供 Workflow 创建写入口。
- Agent CRUD 和 workspace 操作统一归 `agent/`。
- API 客户端不得读取 Store；缺失 Agent 的恢复由 Navigation Feature 负责。
