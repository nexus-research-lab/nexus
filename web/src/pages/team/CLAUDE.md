# Team 页面

- `team-page.tsx` 只允许已登录 Control 远程账户按 `room_id` 适配 Relay 真人消息模型；本地免登录用户返回本地聊天首页。Header、消息阅读轨道、本人消息和 Composer 外观复用 Room 的共享 UI 原语，加载、同步和发送仍交给 `features/team/use-team-room.ts`。
- M1 只显示真人最终消息，不渲染远程 Agent 流。

- Enter 发送先排除输入法组合事件（含 keyCode 229）；Shift+Enter 保留换行。空内容、加载前或发送中不得受理提交；失败保留草稿。加载与失败具备 status/alert 语义，加载失败不声明空会话。

- 标题、作者、时间与反馈使用公共 Typography，作者行允许换行；时间遵循当前界面语言。真人首字圆形标记保留领域身份区别，但字符提取复用 getInitials，不按 UTF-16 截断；标记为装饰，姓名正文拥有可访问身份。

- 滚动复用 conversation 的 useFollowScroll 与 ScrollToLatestButton：底部跟随，上滚后保持阅读，显式回到底部恢复跟随。Team 不维护独立滚动阈值、动画或强制 scrollIntoView；会话 identity 为实际 conversation.id。可聚焦 region 支持键盘滚动，局部 lint 例外仅用于该滚动区域。

- 首次加载失败提供公共“重试”按钮，仅调用 retryLoad；加载中禁用且标记 busy。不得用重试入口重发聊天消息。

- 已有消息在读取刷新期间继续可见，列表标记 busy；只有没有消息的首次加载才显示整块加载提示。

- 页面实例按 owner generation、用户与路由 room_id 隔离，草稿和未完成发送不能越过会话切换。
