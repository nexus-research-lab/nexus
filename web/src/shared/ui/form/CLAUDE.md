# 表单原语

- 本目录拥有选择项、复选行、表单控件和分段控制器。
- 这里只处理通用输入语义，不维护业务草稿或提交事务。
- `UiSearchInput` 自己持有可本地化的清除动作，并用 `searchbox` 语义替代宿主语言生成的原生 search shadow 控件；消费者不得另造清除按钮。
- `SidebarSearchField` 只统一侧栏搜索壳层和可选动作，不持有业务状态；搜索框使用暖色内嵌 control field，尾部按钮使用 `SidebarSearchAction` 的稍高一层暖色轻抬升基座，消费者只传业务图标与命令。
- `choice-styles.ts` 先规范化公共状态，再由 variant resolver 独立组合样式；不同变体不得共享条件分支。
