# Team 页面

- `team-page.tsx` 只渲染默认共享 Room，并把加载、同步和发送交给 `features/team/use-team-room.ts`。
- M1 只显示真人最终消息，不渲染远程 Agent 流。

- Enter 发送先排除输入法组合事件（含 keyCode 229）；Shift+Enter 保留换行。空内容、加载前或发送中不得受理提交；失败保留草稿。加载与失败具备 status/alert 语义，加载失败不声明空会话。

- 标题、作者、时间与反馈使用公共 Typography，作者行允许换行；时间遵循当前界面语言。真人首字圆形标记保留领域身份区别，但字符提取复用 getInitials，不按 UTF-16 截断；标记为装饰，姓名正文拥有可访问身份。

- 滚动复用 conversation 的 useFollowScroll 与 ScrollToLatestButton：底部跟随，上滚后保持阅读，显式回到底部恢复跟随。Team 不维护独立滚动阈值、动画或强制 scrollIntoView；会话 identity 为实际 conversation.id。可聚焦 region 支持键盘滚动，局部 lint 例外仅用于该滚动区域。
