# Room Header

- `group-conversation-header.tsx` 通过导航域 `RoomConversationTabs` 装配会话标签，并组合 Room 导航动作和成员管理弹窗；不直接绑定共享标签视图的领域状态。
- `group-member-avatar-stack.tsx` 只在共享 `UiButton` 中投影成员头像与溢出计数，不解释成员命令或复制 Header 按钮状态。
- 成员与辅助菜单作为共享 Header 协作区的直接子项装配，不再自建间距容器；其桌面触发器与历史、视图动作保持同一 36px 高度基线。
- 共用 Tab 与指南菜单归 `surface/header/`，Group 私域不得复制导航定义。
- Header 只提交一个 `RoomDialogSubmission`；成员差异、写入顺序、作用域和刷新归页面命令层。
- 弹窗打开状态必须绑定 `roomId`，异步准备完成后不得跨 Room 显示。

- `../../members/use-room-member-manager.ts` 与窄窗 Surface 共用，持有成员入口的单飞目录准备与临时打开态。Room/owner 改变或卸载使迟到打开失效，返回旧 Room 不恢复原弹窗；标题和目录刷新保留当前弹窗。目录是辅助读取，失败后继续沿既有 AgentStore 行为使用当前成员/目录，不改变成员写事务。
- Header 主头像直接选择公共 `UiRoomAvatar size="md"`，不覆盖尺寸、圆角或阴影。成员按钮保留 36px 命中与最多四枚 xs 头像，额外人数使用公共 sm Badge；全部成员图像是装饰，按钮唯一名称含真实人数，加载时以 busy/disabled 防止重复打开。

Header 主群头像与侧栏使用同一默认九人上限及共享稳定身份排序，不单独裁成四人；成员管理按钮的四枚堆叠仅是入口预览，不影响主群头像。
