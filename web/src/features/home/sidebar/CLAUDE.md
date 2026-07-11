# sidebar/ - Home 侧栏

- `sidebar-directory.ts` 将共享 Home 目录接到 Agent 运行态订阅，不复制目录请求。
- `sidebar-conversation-model.ts` 只投影 Room/DM 目录项；未读状态由 `sidebar-unread-model.ts` 统一聚合。
- `use-chat-sidebar-controller.ts` 负责聊天列表导航、Room 创建和删除事务，视图不得直接调用 API 或 Store 命令。
- `chat-sidebar-panel.tsx` 与 `contacts-sidebar-panel.tsx` 是两个独立入口，不再通过聚合文件互相耦合。
- `sidebar-list-rows.tsx` 只渲染共享搜索框和目录行，不推导目录或通知状态。
