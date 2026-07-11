# conversation-tabs/ - Workspace 会话标签

- `conversation-tabs-model.ts` 定义排序、打开集合、活动标签、关闭邻居和宽度分配等纯模型。
- `use-conversation-tabs-controller.ts` 只维护浏览器式标签事务和容器测量，不渲染样式。
- `workspace-conversation-tab.tsx` 只渲染单个标签；不得自行推导会话集合状态。
- 当前活动标签必须属于打开集合，宽度模型不得依赖 Effect 的执行时序修正非法状态。
