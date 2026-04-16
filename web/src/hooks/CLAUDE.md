# hooks/

L2 | 父级: web/CLAUDE.md

## 成员清单

- `agent/use-agent-conversation.ts`: useAgentConversation Hook 实现，负责装配 Agent 对话状态
- `agent/index.ts`: hooks/agent 目录导出入口
- `agent/message-helpers.ts`: 消息归并与排序辅助
- `agent/websocket-event-handler.ts`: WebSocket 事件分发
- `agent/conversation-actions.ts`: 对话动作层
- `agent/conversation-lifecycle.ts`: 对话生命周期处理
- `use-extract-todos.ts`: 从消息中提取 TodoItem 的 Hook
- `use-initialize-conversations.ts`: 初始化对话列表的 Hook（hydration 控制）
- `use-conversation-loader.ts`: 响应式对话加载 Hook
- `use-follow-scroll.ts`: 聊天面板自动跟随底部的滚动管理 Hook（跟随/暂停/触摸手势/resize）
- `use-assistant-content-merge.ts`: 合并并去重一轮对话中多条 assistant 消息的内容块，追踪流式输出索引

[PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
