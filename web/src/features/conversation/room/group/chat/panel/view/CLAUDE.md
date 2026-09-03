# Group Chat Panel 视图

- `group-chat-panel-view.tsx` 只选择空状态或活动会话布局，并在 pending interaction 存在时把共享确认组件作为 Composer 输入壳的互斥内容传入。
- `room-goal-lead-control.tsx` 独占负责人选择控件及其展示文案。

视图不得读取会话 Hook、拼装领域事件或自行推导 Room 权限。

协作活动属于紧凑状态提示，使用共享 `xs` muted Spinner；视图不得自行维护尺寸、颜色、旋转或 reduced-motion class。
