# hooks/

L2 | 父级: web/CLAUDE.md

## 成员清单

- `agent/use-agent-conversation.ts`: useAgentConversation Hook 实现，负责装配 Agent 对话状态
- `agent/index.ts`: hooks/agent 目录导出入口
- `agent/message-helpers.ts`: 消息归并与排序辅助
- `agent/websocket-event-handler.ts`: WebSocket 事件分发
- `agent/session-actions.ts`: 对话动作层
- `agent/session-lifecycle.ts`: 对话生命周期处理
- `use-extract-todos.ts`: 从消息中提取 TodoItem 的 Hook
- `use-initialize-conversations.ts`: 初始化对话列表的 Hook（hydration 控制）
- `use-conversation-loader.ts`: 响应式对话加载 Hook

[PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
