# Shared Dialog

- `dialog.tsx` 只提供 Portal、Backdrop、Shell、Header、Body 与 Footer 结构原语。
- Backdrop 的高层叠放只通过 `layer` 选择共享语义层，窄窗口留白只通过 `inset` 选择；业务不得在 `className` 中写高位 z-index 或复制小窗口 padding。
- `dialog-layout.ts` 独占 `content / compact / compactMax / adaptive / adaptiveMax / workbench` 视口模式；选择器和短向导使用固定 620px 上限的 `compact`，内容量不固定但应保持紧凑的目录使用 `compactMax`，长表单使用 `adaptive` 或只限高的 `adaptiveMax`，业务 Shell 不复制桌面/窄窗口高度公式；大型图形编辑与对照界面同时选择 `size="workbench"`。
- 关闭按钮默认可访问名称使用当前语言的 `common.close`；业务只有在语义更具体时才覆盖。
- `dialog-behavior.ts` 只装配 React 生命周期，不保存键盘规则或模态全局状态。
- `dialog-modal-runtime.ts` 独占模态栈与页面滚动锁；叠层关闭顺序由栈顶令牌决定。
- `dialog-focus.ts` 独占可聚焦元素发现、可见性过滤与无滚动聚焦。
- `dialog-keyboard.ts` 用有序规则投影 Escape 与 Tab 动作，不直接读写 DOM。
- `decision/` 组合确认与输入弹窗，复用共享模态协议，不自行注册焦点或键盘生命周期。
- 单行 `PromptDialog` 固定使用 `xs` 决策宽度，多行 Prompt 使用 `sm`；标题、输入控件和主次动作必须分别复用 `UiDialogHeader`、`UiInput/UiTextarea` 与共享 Button recipe，业务只提供文案和提交命令。
- 异步确认执行中必须禁用关闭、取消和重复确认；高后果操作的失败留在原弹窗内，用自然文案完整说明结果、已有数据影响和安全下一步。
- Dialog 与锚点浮层使用同一高不透明主题表面、16px 外轮廓与细边界；Dialog 只在尺寸层级上使用更深一档同源阴影。
- Dialog 遇到带共享打开态契约的子浮层时不消费 Escape，由最内层浮层先关闭。
- 遮罩关闭是显式策略；迁移旧弹窗时不得借共享骨架改变原有关闭语义。
- 业务弹窗不得自行注册全局 Escape、焦点循环或页面滚动锁。
- `dialog.test.tsx` 是 Portal 模态的可执行行为合同：必须覆盖初始焦点、Tab 循环、嵌套关闭顺序、子浮层 Escape、遮罩策略、滚动锁计数和焦点归还。
