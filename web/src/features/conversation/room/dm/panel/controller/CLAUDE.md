# DM Chat Panel 控制器

- `use-dm-chat-panel-model.ts` 只按业务阶段组合控制器和纯投影；布局和 Provider 状态来自共享面板环境。
- `use-dm-chat-session-controller.ts` 独占会话事件、Todo 与外部快照观察。
- `use-dm-chat-composer-model.ts` 独占附件准备、初始草稿和输入动作。
- `use-dm-goal-controller.ts` 独占 Goal 展示刷新和续跑约束；创建由 Composer 装配为独立 `set_goal` host control。
- `dm-chat-panel-projection.ts` 只把领域状态投影为视图模型，并把完整 pending interaction 队列交给 Composer replacement surface；Feed 仍消费同一真相以展示不可操作的等待证据。
- 嵌入模式沿用同一 Session/Feed/Composer 控制器，只隐藏不适用于短期编辑 Session 的 Goal、能力菜单、Session 设置和本地目录入口，并向外报告运行忙碌态。

Frame、导航、视口和滚动控件统一复用 `shared/conversation-panel-model.ts`。
接口由消费阶段定义，只传实际读取的数据；不得通过 Hook `ReturnType` 反向依赖完整 Session 控制器。
