# User 消息视图

- `message-user-section.tsx`: 只选择阅读态或编辑态并装配子视图。
- `user-message-model.ts`: 投影密度、引导标记、时间和可用动作。
- `user-message-header.tsx`: 分离复制/编辑动作与用户身份。
- `user-message-content.tsx`: 组合正文和附件，不解释编辑状态。
- `use-user-message-editor.ts`: 管理编辑草稿、聚焦、高度和提交状态。
- `user-message-editor.tsx`: 编辑表单纯视图。
- `message-user-attachments.tsx`: 先投影附件名称、作用域动作和样式，再按附件类型表渲染工作区附件。

附件是否可打开只由工作区 Agent 作用域决定；编辑视图不持有消息标识或调用上层会话命令。
User 入口在消费侧声明消息、正文、附件和复制动作的最小结构，不依赖 Assistant 状态或控制器返回类型；上游按角色筛选后必须保留 `UserMessage` 具体类型，不得要求视图重复判别角色。
