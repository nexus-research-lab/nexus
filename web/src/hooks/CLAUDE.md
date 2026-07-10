# hooks/

L2 | 父级: web/CLAUDE.md

## 成员清单

- `agent/`: Agent 对话控制器；公开入口只负责装配，动作、会话、运行态和 WebSocket 传输各自维护内部边界
- `use-extract-todos.ts`: 从消息中提取 TodoItem 的 Hook
- `use-initialize-conversations.ts`: 初始化对话列表的 Hook（hydration 控制）
- `use-conversation-loader.ts`: 响应式对话加载 Hook
- `use-follow-scroll.ts`: 聊天面板自动跟随底部的滚动管理 Hook（跟随/暂停/触摸手势/resize）
- `use-assistant-content-merge.ts`: 合并并去重一轮对话中多条 assistant 消息的内容块，追踪流式输出索引

[PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
