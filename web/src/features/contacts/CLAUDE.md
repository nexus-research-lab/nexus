# Contacts 视图

- 本目录只提供联系人目录、卡片和详情视图，不读取 URL、Store 或调用 Agent/Room API。
- Agent 管理目录仅在手机与窄窗使用紧凑单列摘要卡；`md` 起恢复 comfort 大卡片，并按桌面宽度使用两至四列。主体动作由本领域以独立按钮承载，底部动作不得嵌套在共享卡片交互语义中。
- 详情页复用 Agent Options 的可编辑字段投影、保存命令和名称校验。
- 手机详情把聊天与发起群聊收进 `contacts-agent-detail-actions-menu.tsx`，返回动作由应用级手机 Header 提供。
- 视图回调由页面消费者定义，保持具体且不暴露整页控制器。
