# hooks/

L2 | 父级: web/CLAUDE.md

## 成员清单

- `agent/index.ts`: useAgentSession Hook 主入口，管理 WebSocket 连接和消息状态
- `agent/types.ts`: useAgentSession 的入参和返回值类型
- `use-extract-todos.ts`: 从消息中提取 TodoItem 的 Hook
- `use-initialize-sessions.ts`: 初始化会话列表的 Hook（hydration 控制）
- `use-session-loader.ts`: 响应式会话加载 Hook
- `use-typewriter.ts`: 打字机效果 Hook

[PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
