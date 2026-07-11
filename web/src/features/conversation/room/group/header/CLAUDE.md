# Room Header

- `group-conversation-header.tsx` 只装配会话标签、Room 导航动作和成员管理弹窗。
- `group-member-avatar-stack.tsx` 只投影成员头像与溢出计数，不解释成员命令。
- 共用 Tab 与指南菜单归 `surface/header/`，Group 私域不得复制导航定义。
- Header 只提交一个 `RoomDialogSubmission`；成员差异、写入顺序、作用域和刷新归页面命令层。
- 弹窗打开状态必须绑定 `roomId`，异步准备完成后不得跨 Room 显示。
