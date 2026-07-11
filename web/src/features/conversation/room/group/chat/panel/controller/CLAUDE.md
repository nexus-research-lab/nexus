# Group Chat Panel 控制器

- `use-group-chat-panel-model.ts` 只编排业务阶段，不直接展开 Feed、导航或输入区字段。
- `use-group-chat-session-controller.ts` 独占会话身份、Room 事件和外部快照观察器。
- `use-group-chat-composer-model.ts` 独占附件准备、初始草稿和输入区动作装配。
- `use-room-goal-composer.ts` 独占负责人选择与 Goal 创建事务。
- `group-chat-panel-projection.ts` 只把已完成的领域状态投影为视图模型，不持有状态或副作用。
- 导航、视口和滚动控件统一复用 `shared/conversation-panel-model.ts`，不得在 DM / Room 内各自复制。

接口由消费阶段定义，只传实际读取的数据；不得重新引入恒定权限标记。
Thread 数据只通过 `group/thread/live/use-room-thread-source.ts` 发布，不在 Chat 域保存桥接状态。
