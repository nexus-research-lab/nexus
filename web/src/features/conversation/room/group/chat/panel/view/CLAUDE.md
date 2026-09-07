# Group Chat Panel 视图

- `group-chat-panel-view.tsx` 只选择空状态或活动会话布局，并在 pending interaction 存在时把共享确认组件作为 Composer 输入壳的互斥内容传入。
- `room-goal-lead-control.tsx` 独占负责人选择控件及其展示文案。
- `room-workspace-task-panel.tsx` 把有任务的有效成员交给共用成员切换器，摘要和菜单都以完整 Room 目录稳定重名序号，头像仍取原始展示姓名。`room-workspace-task-model.ts` 只从仍在成员目录中的进程选择，优先有效手动选择，否则按最新任务回退；手动选择失效时视图提交该回退，恢复旧成员或旧进程不重新抢占。精确会话 scope 同时重置手动选择与共享任务浮层，默认自动模式继续跟随最近进程。

视图不得读取会话 Hook、拼装领域事件或自行推导 Room 权限。

协作活动属于紧凑状态提示，使用共享 `xs` muted Spinner；视图不得自行维护尺寸、颜色、旋转或 reduced-motion class。
