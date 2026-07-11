# DM Chat Panel 控制器

- `use-dm-chat-panel-model.ts` 只按业务阶段组合控制器和纯投影。
- `use-dm-chat-session-controller.ts` 独占会话事件、Todo 与外部快照观察。
- `use-dm-chat-composer-model.ts` 独占附件准备、初始草稿和输入动作。
- `use-dm-goal-controller.ts` 独占 Goal 创建和续跑约束。
- `dm-chat-panel-projection.ts` 只把领域状态投影为视图模型。

导航、视口和滚动控件统一复用 `shared/conversation-panel-model.ts`。
接口由消费阶段定义，只传实际读取的数据；不得通过 Hook `ReturnType` 反向依赖完整 Session 控制器。
