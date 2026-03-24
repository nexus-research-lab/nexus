# types/

L2 | 父级: web/CLAUDE.md

## 成员清单

- `index.ts`: 类型统一导出 barrel
- `api.ts`: API 通用响应类型 `ApiResponse<T>`
- `sdk.ts`: SDK 基础类型（UUID/SessionId/ToolInput/ToolOutput）
- `message.ts`: 消息类型（Message/EventMessage 及内容块）
- `conversation.ts`: 对话类型（Conversation/ApiConversation/CRUD 参数）
- `agent-conversation.ts`: useAgentConversation 相关类型
- `agent.ts`: Agent 类型（Agent/AgentOptions/Workspace 文件操作类型）
- `cost.ts`: 成本统计类型（SessionCostSummary/AgentCostSummary）
- `ask-user-question.ts`: AskUserQuestion 工具交互类型
- `websocket.ts`: WebSocket 连接与消息类型

[PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
