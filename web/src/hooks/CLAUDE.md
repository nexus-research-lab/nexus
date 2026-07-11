# hooks/

L2 | 父级: web/CLAUDE.md

## 成员清单

- `agent/`: Agent 对话控制器；公开入口只负责装配，动作、会话、运行态和 WebSocket 传输各自维护内部边界
- `room-page-controller/`: Room 页面编排；数据资源、纯投影、Room 命令、会话快照和现有 Agent 配置各自管理作用域
- `launcher/`: Launcher 目录资源和选择命令；不承载无入口的 Agent 编辑弹窗状态
- `use-initialize-conversations.ts`: 初始化对话列表的 Hook（hydration 控制）
- `use-conversation-loader.ts`: 响应式对话加载 Hook
- `use-assistant-content-merge.ts`: 合并并去重一轮对话中多条 assistant 消息的内容块，追踪流式输出索引

[PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
