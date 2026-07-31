# Room Surface

- `room-chat-surface.tsx` 是 DM/Group 与 desktop/mobile 共用的聊天参数装配边界；布局和 Room Host 身份沿唯一上游保持显式。
- `room-chat-error-boundary.tsx` 按会话身份隔离渲染错误，`room-chat-error-view.tsx` 只负责 i18n 回退视图。
- `header/` 保存 DM/Group 共用导航，`mobile/` 按头部、会话 Sheet 和全屏 Overlay 分离移动端职责。
- Surface Tab 是 Header 导航契约，不得在全局 `types/` 重复定义 UI 状态。

- 根目录保留桌面/移动端入口和可独立展示的业务 Surface。
- `room-conversation-header-edge.css` 只定义桌面与专注模式 Header 自身向正文延伸的非交互下缘羽化，不改变布局高度、拖窗热区或消息滚动几何。
- `room-surface-model.ts` 放置桌面与移动端共享的纯派生，不读取 UI 状态。
- `room-agent-switcher.tsx` 只投影成员与业务触发器，菜单生命周期复用 `shared/ui/menu/`；Workspace、Subagent 与 Room 多 Agent 进程切换共享这个入口，不得各自复制头像菜单。Task 只使用该入口的紧凑无下划线变体，名称必须与进程状态共享同一 `text-compact` 字号和字重节奏。
- `room-subagent-task-surface.tsx` 复用成员切换器，把当前 Session 的全部 Room 子智能体按实际调用者 `host_agent_id` 投影到共享只读任务表面；轮次不得成为隐藏条件。
- Room Agent 简介按身份、联络、记忆、工具、技能组织；配置复用 Agent Options 的 Edit 来源工厂，记忆复用 `AgentMemoryView`，不在 Surface 复制可编辑字段表或文件式记忆状态。
- 桌面分栏、右侧面板与 Thread 编排统一位于 `layout/`。
- 会话历史排序、能力投影、标题编辑和条目视图统一位于 `history/`。
