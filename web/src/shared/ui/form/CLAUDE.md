# 表单原语

- 本目录拥有选择项、复选行、表单控件和分段控制器。
- 这里只处理通用输入语义，不维护业务草稿或提交事务。
- `UiPasswordInput` 统一屏蔽宿主密码框指示器并提供尺寸稳定、可访问的 Caps Lock 状态，不允许业务表单自行复制实现。
- `SidebarSearchField` 只统一侧栏搜索壳层和可选动作，不持有业务状态。
- `choice-styles.ts` 先规范化公共状态，再由 variant resolver 独立组合样式；不同变体不得共享条件分支。
