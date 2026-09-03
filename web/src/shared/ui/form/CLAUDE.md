# 表单原语

- 本目录拥有选择项、原生复选框、复选行、表单控件和分段控制器。
- 这里只处理通用输入语义，不维护业务草稿或提交事务。
- `UiInput / UiTextarea / UiNativeSelect` 分别拥有普通文本、多行文本和原生下拉字段；`form-control-styles.ts` 是这些 primitive 的内部投影，业务层不得导入后再手写 DOM。嵌入 Composer 等复合控件内部的无壳原生字段必须由该 pattern 明确拥有，不能假装成普通 Field。
- `UiSearchInput` 自己持有可本地化的共享 IconButton 清除动作，并用具名 `searchbox` 语义替代宿主语言生成的原生 search shadow 控件；搜索壳不是 `<label>`，不得把清除按钮嵌入另一个 labelable control；消费者不得另造清除按钮。
- `SidebarSearchField` 只统一侧栏搜索壳层和可选动作，不持有业务状态；搜索框使用暖色内嵌 control field，尾部按钮使用 `SidebarSearchAction` 的稍高一层暖色轻抬升基座，消费者只传业务图标与命令。
- `choice-styles.ts` 先规范化公共状态，再由 variant resolver 独立组合样式；不同变体不得共享条件分支。
- `UiCheckbox` 是所有普通原生 checkbox 的尺寸、品牌色、焦点与 disabled 入口；带说明的整行选择继续组合 `UiCheckboxRow`，生成式问答等自绘选择器不属于该原语。
- `UiSegmentedControl` 是有限互斥选项的唯一入口；选中态使用背景与文字对比，不加阴影，普通设置不使用胶囊圆角。业务页面只提供选项、当前值和尺寸密度，不得再定义私有分段选择器。
- `form-controls.test.tsx` 以真实 invalid/input、select、键盘和点击事件覆盖 Field、Input、NativeSelect、SearchInput、Checkbox、Choice 与 SegmentedControl 的 ARIA/状态合同。
