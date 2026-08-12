# Group Chat Panel 控制器

- `use-group-chat-panel-model.ts` 只编排业务阶段；布局和 Provider 状态来自共享面板环境。
- `use-group-chat-session-controller.ts` 独占会话身份、Room 事件和外部快照观察器。
- `use-group-chat-composer-model.ts` 独占附件准备、初始草稿和输入区动作装配。
- `use-room-goal-composer.ts` 独占负责人草稿与 Goal 刷新信号；`use-group-chat-composer-model.ts` 把提交装配成独立 `set_goal` host control。未提交的负责人选择进入当前 Session 的 Room Composer 草稿胶囊，切回该 Session 时恢复。
- `group-chat-panel-projection.ts` 只把已完成的领域状态投影为视图模型，把 Room 内部 Agent 引导排除在用户待发送队列之外，并把全局 pending interaction 队列交给 Composer replacement surface；不持有状态或副作用。
- `use-group-conversation-unread.ts` 只按稳定 Agent 节点维护 Room 当前 Session 的未读队列，认领通知锚点、测量首节点方向并复用 round scroll 精确跳转；DM、侧栏数字和 Feed 排序不在该 Hook 内重新推导。
- `room-handoff-status-model.ts` 以 `handoff_id` 合并 realtime final message、public mention queue、pending slot 与 execution 证据；阶段只能按 `preparing → queued → active` 提升，历史静态消息不得单独复活交接状态。
- Frame、导航、视口和滚动控件统一复用 `shared/conversation-panel-model.ts`，不得在 DM / Room 内各自复制。

接口由消费阶段定义，只传实际读取的数据；不得通过 Hook `ReturnType` 反向依赖完整 Session 控制器，也不得重新引入恒定权限标记。
Thread 数据只通过 `group/thread/live/use-room-thread-source.ts` 发布，不在 Chat 域保存桥接状态。
