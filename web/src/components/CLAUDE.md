# components/

L2 | 父级: web/CLAUDE.md

## 成员清单

- `chat-interface.tsx`: 对话主界面，编排消息列表 + 输入框 + 空状态
- `empty-state.tsx`: 无会话时的引导界面
- `chat/chat-input.tsx`: 消息输入框组件（含发送/停止控制）
- `header/chat-header.tsx`: 对话头部栏
- `header/loading.tsx`: 加载动画
- `header/message-stats.tsx`: 消息轮次统计
- `message/index.ts`: 消息组件统一导出
- `message/message-item.tsx`: 单轮消息渲染（用户+助手+结果）
- `message/content-renderer.tsx`: 内容块分发渲染器
- `message/markdown-renderer.tsx`: Markdown 渲染（含 KaTeX/GFM）
- `message/block/tool-block.tsx`: 工具调用展示块
- `message/block/thinking-block.tsx`: 思考过程展示块
- `message/block/code-block.tsx`: 代码块渲染
- `message/block/ask-user-question-block.tsx`: 用户交互问答块
- `option/agent-options.tsx`: Agent 创建/编辑对话框
- `permission/permission-dialog.tsx`: 权限请求对话框
- `todo/agent-task-widget.tsx`: Agent 任务列表组件
- `ui/confirm-dialog.tsx`: 通用确认/输入对话框
- `workspace/agent-directory.tsx`: Agent 目录列表页
- `workspace/agent-inspector.tsx`: Agent 详情面板（成本/任务/会话）
- `workspace/agent-switcher.tsx`: Agent 快速切换器
- `workspace/workspace-editor-pane.tsx`: 工作区文件编辑面板
- `workspace/workspace-sidebar.tsx`: 工作区侧边栏（文件树+会话列表）

[PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
