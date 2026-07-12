# Shared Dialog

- `dialog.tsx` 只提供 Portal、Backdrop、Shell、Header、Body 与 Footer 结构原语。
- `dialog-behavior.ts` 只装配 React 生命周期，不保存键盘规则或模态全局状态。
- `dialog-modal-runtime.ts` 独占模态栈与页面滚动锁；叠层关闭顺序由栈顶令牌决定。
- `dialog-focus.ts` 独占可聚焦元素发现、可见性过滤与无滚动聚焦。
- `dialog-keyboard.ts` 用有序规则投影 Escape 与 Tab 动作，不直接读写 DOM。
- Dialog 遇到带共享打开态契约的子浮层时不消费 Escape，由最内层浮层先关闭。
- 遮罩关闭是显式策略；迁移旧弹窗时不得借共享骨架改变原有关闭语义。
- 业务弹窗不得自行注册全局 Escape、焦点循环或页面滚动锁。
