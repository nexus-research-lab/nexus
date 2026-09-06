# Shared Mention

本目录拥有跨 Feature 的 Mention 文本匹配、插入和目标选择视图，不解释 Agent、Room 或消息协议。

- `mention-target-model.ts` 保存触发符、匹配、插入、筛选和键盘命令纯规则，不再持有定位、尺寸或私有方向枚举。
- `mention-target-popover.tsx` 接收真实 input/textarea ref 与打开态，候选组合公共 SelectMenuPanel/SelectMenuOptionRow，行布局和估算高度复用 Menu recipe；只渲染消费者提供的标签、说明和装饰标记。
- 定位和滚动/窗口变化使用 reference-list preset，方向随空间自动选择；Portal、外部指针与 Escape 由公共 anchored layer 管理。保留编辑器焦点，输入父级会阻止冒泡时使用该层的 captureEscape，组件不自行关闭 Dialog。
- 打开时为当前真实编辑器绑定 listbox 与 active descendant；关闭、卸载或替换编辑器后恢复原属性，不接管文本、输入事件或 role。候选按钮不进入顺序 Tab 导航，鼠标按下保留编辑器焦点，click 与捕获键盘分别只选中一次。
- 捕获键盘前必须通过公共 `isImeKeyboardEvent` 排除组合输入和兼容 229；输入法确认不是选择、关闭或移动候选的指令。Launcher 的真实输入/浮层回归覆盖 @Agent 和 #Room 的组合输入后选择。
- 方向键/Enter/Tab 只由当前模态范围的最上层候选、且事件来自自身锚点或列表时消费；关闭/隐藏后立即释放。共置测试覆盖生命周期、真实 Portal 模态隔离、位置刷新、IME、属性恢复和外部指针焦点。

目标分类与标记由消费者投影。共享视图不得根据业务类型决定图标、字符或筛选范围。
