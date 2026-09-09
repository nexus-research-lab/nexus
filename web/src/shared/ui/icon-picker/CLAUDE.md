# 图标选择器

- `icon-picker-model.ts` 只生成资源路径、稳定条目、选中语义和清除动作可见性，不得返回 className、尺寸、颜色、阴影或布局；`icon-picker-layout.ts` 只维护网格列数和横向滚动几何，五列网格用于窄浮层中的大图标选择。
- `icon-picker.tsx` 通过共享 `UiChoiceButton variant="icon"` 渲染图片选择，通过 `UiButton` 渲染清除动作；图片只作为装饰，选择项按钮拥有唯一可访问名称与 pressed 状态，不得恢复原生按钮或私有选中阴影。
- `icon-picker-popover.tsx` 统一 Agent、Personal 与 Room 的锚定头像选择浮层、触发器焦点/禁用状态、`IconPickerTriggerLabel`、关闭事务和焦点归还；业务侧只负责头像内容、对齐和命令，不得各自复制触发按钮、标签箭头、定位与 Portal 逻辑。浮层宽度必须受当前视口边距约束，覆盖桌面壳允许的最小窗口宽度。
- `use-icon-picker-row-scroll.ts` 独占横排图标的尺寸测量、滚动位置与分页命令；可见滑轨必须与原生滚动位置同步。
- 图标族由 `lib/avatar` 定义，本目录不得维护业务侧 Agent 或 Room 图标范围。

- 浮层定位后聚焦当前选项或首项；Tab 在选项边界按触发器所在表单继续，Escape 与外部点击由公共 anchored overlay 所有者处理。禁用、目录或值变化关闭旧浮层，业务对象切换由对象作用域重建入口。浮层高度不能以固定最小高度突破可用视口。
- 头像复用公共档位：Agent/Room 编辑入口 lg，个人资料 xl；不在消费者叠加私有尺寸、边框悬停色或圆角。网格选择保持正方形并受单元格宽度约束；选项名称由视图本地化，纯模型不生成用户文案。
