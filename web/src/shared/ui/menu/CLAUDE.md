# Shared Menu

- `select-menu-model.ts` 保存 Select/MultiSelect 共用的尺寸与表面样式。
- `action-menu.tsx`、`select-menu.tsx` 与 `multi-select-menu.tsx` 各自拥有交互语义和内容视图。

菜单组件直接导入，不通过另一个菜单文件隐式转出。锚点定位与浏览器生命周期统一复用 `shared/ui/overlay/`。
