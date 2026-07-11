# Shared Menu

- `select-menu-model.ts` 以静态配置表保存 Select/MultiSelect 共用的尺寸、表面和选中态样式。
- `use-select-menu-overlay.ts` 统一 Select 家族的内部开关、锚点定位和触发键盘协议。
- `select-menu-primitives.tsx` 只提供 Select 家族共用的触发器内容和 listbox 框架。
- `select-menu.tsx` 与 `multi-select-menu.tsx` 各自拥有单选、方向移动、多选、搜索和异步内容语义。
- `action-menu.tsx` 保持外部受控，不复用 Select 家族的内部开关状态。

菜单组件直接导入，不通过另一个菜单文件隐式转出。锚点定位与浏览器生命周期统一复用 `shared/ui/overlay/`。
