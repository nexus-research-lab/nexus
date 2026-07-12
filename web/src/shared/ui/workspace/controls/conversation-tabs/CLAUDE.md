# conversation-tabs/ - Workspace 会话标签

- `conversation-tabs-model.ts` 定义排序、打开集合、活动标签、关闭邻居和宽度分配等纯模型；打开集合按保留存活项、追加外部选中项、保证非空三阶段归约。
- `use-conversation-tabs-controller.ts` 只维护浏览器式标签事务和容器测量，不渲染样式。
- `workspace-conversation-tab-model.ts` 统一推导单标签的活动态样式、宽度、标题和关闭态。
- `workspace-conversation-tab.tsx` 只渲染单个标签；不得自行推导会话集合状态或状态样式。
- 当前活动标签必须属于打开集合，宽度模型不得依赖 Effect 的执行时序修正非法状态。
- 外部 Session 判断和标签统一来自 `lib/conversation/external-session.ts`，标签域不得维护通道别名。
