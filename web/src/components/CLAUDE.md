# components/

L2 | 父级: web/CLAUDE.md

## 成员清单

- `loading.tsx`: 共享加载动画原子组件（跨域消费）

### chat/ — 对话域
- `chat-interface.tsx`: 对话主界面，编排消息列表 + 输入框 + 空状态
- `chat-input.tsx`: 消息输入框组件（含发送/停止控制）
- `chat-header.tsx`: 对话头部栏
- `empty-state.tsx`: 无会话时的引导界面

### message/ — 消息渲染域
- `index.ts`: 消息组件统一导出
- `message-item.tsx`: 单轮消息渲染（用户+助手+结果）
- `content-renderer.tsx`: 内容块分发渲染器
- `markdown-renderer.tsx`: Markdown 渲染（含 KaTeX/GFM）
- `message-stats.tsx`: 消息轮次统计显示
- `block/tool-block.tsx`: 工具调用展示块
- `block/thinking-block.tsx`: 思考过程展示块
- `block/code-block.tsx`: 代码块渲染
- `block/ask-user-question-block.tsx`: 用户交互问答块

### workspace/ — 工作区域
- `console.tsx`: 首页 Spotlight 启动页
- `agent-inspector.tsx`: Agent 详情面板（成本/任务/会话）
- `agent-switcher.tsx`: Agent 快速切换器
- `workspace-editor-pane.tsx`: 工作区文件编辑面板
- `workspace-sidebar.tsx`: 工作区侧边栏（文件树+会话列表）
- `agent-task-widget.tsx`: Agent 任务列表组件

### dialog/ — 对话框域
- `agent-options.tsx`: Agent 创建/编辑对话框
- `permission-dialog.tsx`: 权限请求对话框
- `confirm-dialog.tsx`: 通用确认/输入对话框

[PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
