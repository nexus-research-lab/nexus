# Team 页面

- `team-page.tsx` 只渲染默认共享 Room，并把加载、同步和发送交给 `features/team/use-team-room.ts`。
- M1 只显示真人最终消息，不渲染远程 Agent 流。

- Enter 发送先排除输入法组合事件（含 keyCode 229）；Shift+Enter 保留换行。空内容、加载前或发送中不得受理提交；失败保留草稿。加载与失败具备 status/alert 语义，加载失败不声明空会话。

- 标题、作者、时间与反馈使用公共 Typography，作者行允许换行；时间遵循当前界面语言。真人首字圆形标记保留领域身份区别，但字符提取复用 getInitials，不按 UTF-16 截断；标记为装饰，姓名正文拥有可访问身份。
