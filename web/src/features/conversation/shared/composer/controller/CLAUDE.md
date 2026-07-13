# controller/

L5 | 父级: web/src/features/conversation/shared/composer

## 职责

- `use-composer-controller.ts`: 组合草稿、附件、提及、历史和各动作协议
- `use-composer-draft.ts`: 管理输入模式与弹层状态转换
- `use-composer-message-submit.ts`: 按资格判断、附件准备、投递和收尾阶段提交消息
- `use-composer-goal-actions.ts`: 管理 Goal 与 Loop 动作
- `use-composer-keyboard.ts`: 依次执行输入法、Safari 和 Mention 守卫，再分派键盘命令
- `composer-view-projections.ts`: 分别投影输入、运行时、模式和动作状态
- `composer-controller-model.ts`: 组装各状态投影为视图消费契约

控制器只编排窄接口，不自行复制子领域状态。视图可见状态必须由模型纯函数派生，异步动作不得塞回视图组件；状态投影之间只传递明确结果，不读取彼此的实现条件。
运行状态投影把 `compacting` 作为独立活动传给 Footer，同时继续独立计算停止按钮资格。
