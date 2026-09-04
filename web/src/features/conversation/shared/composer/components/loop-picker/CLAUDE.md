# loop-picker/

L6 | 父级: web/src/features/conversation/shared/composer/components

## 职责

- `loop-picker-dialog.tsx`: 控制开放作用域并装配无图标、无副标题的目录型 Dialog
- `use-loop-picker-controller.ts`: 加载目录、维护筛选和串行选择
- `loop-picker-model.ts`: 生成分类、筛选结果和内容状态
- `loop-picker-content.tsx`: 展示加载、错误、空态或列表
- `loop-picker-item.tsx`: 展示单个 Loop 并提交选择

Dialog 关闭时卸载开放作用域，状态通过 React 生命周期自然清空，不维护额外 reset key。资源响应和选择结果必须停留在当前开放作用域。
Loop 使用 `UiListRow` 的统一 hover、focus、键盘和禁用语义展示标题、摘要和简短元数据，不嵌套选项卡片、手写原生按钮或重复选择动作。
Loop 启动失败只向 `UiInlineNotice` 提供错误正文与 alert tone；Dialog 不再拥有私有错误卡片样式。
