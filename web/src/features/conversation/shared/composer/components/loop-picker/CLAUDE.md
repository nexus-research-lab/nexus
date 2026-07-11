# loop-picker/

L6 | 父级: web/src/features/conversation/shared/composer/components

## 职责

- `loop-picker-dialog.tsx`: 控制开放作用域并装配 Dialog
- `use-loop-picker-controller.ts`: 加载目录、维护筛选和串行选择
- `loop-picker-model.ts`: 生成分类、筛选结果和内容状态
- `loop-picker-content.tsx`: 展示加载、错误、空态或列表
- `loop-picker-item.tsx`: 展示单个 Loop 并提交选择

Dialog 关闭时卸载开放作用域，状态通过 React 生命周期自然清空，不维护额外 reset key。资源响应和选择结果必须停留在当前开放作用域。
