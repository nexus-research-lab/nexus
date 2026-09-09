# Team 页面

- `team-page.tsx` 只渲染默认共享 Room，并把加载、同步和发送交给 `features/team/use-team-room.ts`。
- M1 只显示真人最终消息，不渲染远程 Agent 流。

- Enter 发送先排除输入法组合事件（含 keyCode 229）；Shift+Enter 保留换行。空内容、加载前或发送中不得受理提交；失败保留草稿。加载与失败具备 status/alert 语义，加载失败不声明空会话。
