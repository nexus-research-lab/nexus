# Team 页面

- `team-page.tsx` 只允许已登录 Control 远程账户按 `room_id` 适配 Relay 真人消息模型；本地免登录用户返回本地聊天首页。Header、消息阅读轨道、本人消息和 Composer 外观复用 Room 的共享 UI 原语，加载、同步和发送仍交给 `features/team/use-team-room.ts`。
- 显示真人消息与 Agent 完整 assistant/final 回复，不渲染远程 Agent 流；Header 本机授权入口区分设备登记与尚未接入的执行器。
- Header 的成员入口读取 Relay Room 管理快照；真人群主和管理员可以邀请、移除，真人群主还可以改角色和移交治理权；Agent 与真人分区显示，每名 active 真人可添加自己的 Agent。

- Enter 发送先排除输入法组合事件（含 keyCode 229）；Shift+Enter 保留换行。空内容、加载前或发送中不得受理提交；失败保留草稿。加载与失败具备 status/alert 语义，加载失败不声明空会话。

- 标题、作者、时间与反馈使用公共 Typography，作者行允许换行；时间遵循当前界面语言。真人首字圆形标记保留领域身份区别，但字符提取复用 getInitials，不按 UTF-16 截断；标记为装饰，姓名正文拥有可访问身份。

- 滚动复用 conversation 的 useFollowScroll 与 ScrollToLatestButton：底部跟随，上滚后保持阅读，显式回到底部恢复跟随。Team 不维护独立滚动阈值、动画或强制 scrollIntoView；会话 identity 为实际 conversation.id。可聚焦 region 支持键盘滚动，局部 lint 例外仅用于该滚动区域。

- 首次加载失败提供公共“重试”按钮，仅调用 retryLoad；加载中禁用且标记 busy。不得用重试入口重发聊天消息。

- 已有消息在读取刷新期间继续可见，列表标记 busy；只有没有消息的首次加载才显示整块加载提示。

- 页面实例按 owner generation、用户与路由 room_id 隔离，草稿和未完成发送不能越过会话切换。
- 成员弹窗的新快照回传 `useTeamRoom.updateDetails`，Agent 目录随成员版本重读；选中的目标失效后仍保留可移除 chip，不降级成普通消息。未确认发送冻结输入与目标选择，仅保留原请求重试。
- Composer 恢复发件箱的原正文，不能用当前空草稿覆盖未知命令；失去群访问权后禁用输入。Header 使用远端 Room 当前名称和头像，解散/退出完成后重新核对目录。

- 顶栏复用 Room 的 WorkspaceConversationTabs 与 GroupMemberAvatarStack：在线唯一会话不提供关闭、新建、固定或本地工作区命令；成员摘要仅计 active 真人和 Agent，目录只补名称与头像。本机授权作为同规格轻量动作，窄屏保留具名图标。

- 真人 DM 通过 `room.direct_user_id` 解析对方姓名头像；消息沿用同一 Feed/Composer。`room_invitation` 是 Relay 生成的持久卡片，操作必须匹配当前待处理邀请和邀请时间；旧邀请没有消息卡片时在对应私聊补显示待办，不能根据卡片直接推断授权。
