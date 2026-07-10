# composer/

L4 | 父级: web/src/features/conversation/shared

## 职责

- `composer-panel.tsx`: 输入区、发送按钮和 Footer 的纯视图装配
- `use-composer-controller.ts`: 消息、Goal、Loop、IME 与输入状态编排
- `use-composer-history.ts`: 发送历史记录和上下键召回
- `composer-model.ts`: 输入策略、键盘规则和布局状态表
- `use-composer-mention.ts`: Room 成员提及状态
- `use-conversation-composer-handlers.ts`: DM/Room 对 Composer 的发送适配
- `attachments/`: 本地附件建模、准备和展示
- `components/`: Footer、待发送队列和 Loop 选择器

发送目标先投影为 `send/enqueue + delivery policy`，再调用对应消费者；不要在视图中复制忙碌态判断。
中文输入法的 composition 保护属于控制器边界，修改键盘流程时必须保留 Safari 的补发 Enter 防护。
