# Team 页面

- `team-page.tsx` 只允许已登录 Control 远程账户按 `room_id` 适配 Relay 真人消息模型；本地免登录用户返回本地聊天首页。Header、消息阅读轨道、本人消息和 Composer 外观复用 Room 的共享 UI 原语，加载、同步和发送仍交给 `features/team/use-team-room.ts`。
- M1 只显示真人最终消息，不渲染远程 Agent 流。
