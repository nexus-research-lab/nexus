# Shared Menu

- `select-menu-model.ts` 以静态配置表保存菜单共用的尺寸、表面和选中态样式。
- `use-select-menu-overlay.ts` 统一选择菜单的内部开关、锚点定位和触发键盘协议。
- `select-menu-primitives.tsx` 只提供选择菜单共用的触发器内容和 listbox 框架。
- `select-menu-view.tsx` 只渲染共享单选菜单，不读取业务状态或决定选值。
- `select-menu.tsx` 只编排共享单选语义和浮层生命周期；带搜索、异步状态或多选规则的菜单归真实业务所有者。
- `action-menu.tsx` 保持外部受控，不复用 Select 家族的内部开关状态。

菜单组件直接导入，不通过另一个菜单文件隐式转出。锚点定位与浏览器生命周期统一复用 `shared/ui/overlay/`。
