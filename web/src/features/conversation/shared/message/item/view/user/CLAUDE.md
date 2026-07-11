# User 消息视图

- `message-user-section.tsx`: 组合用户身份、正文、编辑器和附件。
- `use-user-message-editor.ts`: 管理编辑草稿、聚焦、高度和提交状态。
- `user-message-editor.tsx`: 编辑表单纯视图。
- `message-user-attachments.tsx`: 按附件类型表渲染工作区附件。

附件是否可打开只由工作区 Agent 作用域决定；编辑视图不持有消息标识或调用上层会话命令。
User 入口在消费侧声明消息、正文、附件和复制动作的最小结构，不依赖 Assistant 状态或控制器返回类型。
