# Room Header

- `group-conversation-header.tsx` 通过导航域 `RoomConversationTabs` 装配会话标签，并组合 Room 导航动作和成员管理弹窗；不直接绑定共享标签视图的领域状态。
- `group-member-avatar-stack.tsx` 只在共享 `UiButton` 中投影成员头像与溢出计数，不解释成员命令或复制 Header 按钮状态。
- 成员与辅助菜单作为共享 Header 协作区的直接子项装配，不再自建间距容器；其桌面触发器与历史、视图动作保持同一 36px 高度基线。
- 共用 Tab 与指南菜单归 `surface/header/`，Group 私域不得复制导航定义。
- Header 只提交一个 `RoomDialogSubmission`；成员差异、写入顺序、作用域和刷新归页面命令层。
- 弹窗打开状态必须绑定 `roomId`，异步准备完成后不得跨 Room 显示。
