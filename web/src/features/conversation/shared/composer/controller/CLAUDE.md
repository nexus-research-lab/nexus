# controller/

L5 | 父级: web/src/features/conversation/shared/composer

## 职责

- `use-composer-controller.ts`: 组合草稿、附件、提及、历史和各动作协议
- `use-composer-draft.ts`: 管理输入模式与弹层状态转换
- `use-composer-message-submit.ts`: 解析投递目标并提交消息
- `use-composer-goal-actions.ts`: 管理 Goal 与 Loop 动作
- `use-composer-keyboard.ts`: 管理输入法和键盘命令
- `composer-controller-model.ts`: 纯函数派生视图状态

控制器只编排窄接口，不自行复制子领域状态。视图可见状态必须由模型纯函数派生，异步动作不得塞回视图组件。
