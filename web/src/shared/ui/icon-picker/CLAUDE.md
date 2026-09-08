# 图标选择器

- `icon-picker-model.ts` 只生成资源路径、稳定条目、选中语义和清除动作可见性，不得返回 className、尺寸、颜色、阴影或布局；`icon-picker-layout.ts` 只维护网格列数和横向滚动几何，五列网格用于窄浮层中的大图标选择。
- `icon-picker.tsx` 通过共享 `UiChoiceButton variant="icon"` 渲染图片选择，通过 `UiButton` 渲染清除动作；图片只作为装饰，选择项按钮拥有唯一可访问名称与 pressed 状态，不得恢复原生按钮或私有选中阴影。
- `icon-picker-popover.tsx` 统一 Agent、Personal 与 Room 的锚定头像选择浮层、触发器焦点/禁用状态、`IconPickerTriggerLabel`、关闭事务和焦点归还；业务侧只负责头像内容、对齐和命令，不得各自复制触发按钮、标签箭头、定位与 Portal 逻辑。浮层宽度必须受当前视口边距约束，覆盖桌面壳允许的最小窗口宽度。
- `use-icon-picker-row-scroll.ts` 独占横排图标的尺寸测量、滚动位置与分页命令；可见滑轨必须与原生滚动位置同步。
- 图标族由 `lib/avatar` 定义，本目录不得维护业务侧 Agent 或 Room 图标范围。
