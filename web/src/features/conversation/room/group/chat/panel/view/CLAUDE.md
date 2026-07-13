# Group Chat Panel 视图

- `group-chat-panel-view.tsx` 只选择空状态或活动会话布局，并消费完整视图模型。
- `room-goal-lead-control.tsx` 独占负责人选择控件及其展示文案。

视图不得读取会话 Hook、拼装领域事件或自行推导 Room 权限。
