# User 消息视图

- `message-user-section.tsx`: 只选择阅读态或编辑态并装配子视图。
- `user-message-model.ts`: 投影密度、引导标记、时间和可用动作。
- `user-message-header.tsx`: 渲染消息下方的时间与共享微型悬浮动作；复制成功只使用 Button 的短暂 `success` tone，不重复显示用户昵称或头像。
- `user-message-content.tsx`: 组合正文和附件，为消息开头的通用 Slash 指令开启共享命令标签；超高正文按真实排版高度折叠，并复用时间线滚动锚点完成展开与收起。
- `use-user-message-editor.ts`: 管理编辑草稿、聚焦、高度和提交状态。
- `user-message-editor.tsx`: 编辑表单纯视图。
- `message-user-attachments.tsx`: 先投影附件名称、作用域动作和样式，再按附件类型表渲染工作区附件。

附件是否可打开只由工作区 Agent 作用域决定；编辑视图不持有消息标识或调用上层会话命令。
User 入口在消费侧声明消息、正文、附件和复制动作的最小结构，不依赖 Assistant 状态或控制器返回类型；上游按角色筛选后必须保留 `UserMessage` 具体类型，不得要求视图重复判别角色。
无气泡 User 正文必须与 Assistant 正文复用 `nexus-chat-message-body-rhythm`，桌面与窄栏的头部到正文、正文到消息尾部间距不得各自覆写。
