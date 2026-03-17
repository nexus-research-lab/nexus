# hooks/

L2 | 父级: web/CLAUDE.md

## 成员清单

- `agent/use-agent-session.ts`: useAgentSession Hook 实现，负责装配 Agent 会话状态
- `agent/index.ts`: hooks/agent 目录导出入口
- `agent/types.ts`: useAgentSession 的入参和返回值类型
- `agent/message-helpers.ts`: 消息归并与排序辅助
- `agent/websocket-event-handler.ts`: WebSocket 事件分发
- `agent/session-actions.ts`: 会话动作层
- `agent/session-lifecycle.ts`: 会话生命周期处理
- `agent/session-context.ts`: 共享上下文类型
- `use-extract-todos.ts`: 从消息中提取 TodoItem 的 Hook
- `use-initialize-sessions.ts`: 初始化会话列表的 Hook（hydration 控制）
- `use-session-loader.ts`: 响应式会话加载 Hook

[PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
