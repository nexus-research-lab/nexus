# Shared Menu

- `select-menu-model.ts` 只计算当前选项、键盘遍历、高度估算和锚点几何，不得返回视觉类；`select-menu-styles.ts` 独占菜单共用的尺寸、表面、标签换行和选中态视觉 recipe。业务需要组合多选或特殊 listbox 时可以复用 style recipe，但不得从 model 导入样式。
- `use-select-menu-overlay.ts` 统一选择菜单的内部开关、锚点定位和触发键盘协议。
- Select 进入 disabled 状态必须立即收起 listbox、清除 expanded/controls 关系并丢弃旧打开态；恢复可用后只能由用户重新打开。Select/Action Menu 的 Escape 和焦点归还交给共享 Overlay 仲裁，不自行抢先关闭父菜单。
- `select-menu-primitives.tsx` 提供选择菜单共用的 `SelectMenuTrigger`、触发器内容、listbox 框架和 `SelectMenuOptionRow`；SelectMenuTrigger 统一单选与领域多选的原生 button、listbox ARIA、ref 和原生事件透传，直接消费既有样式投影，不改变调用者的开关、键盘或布局。所有单选、多选、Slash/Mention 类 listbox 条目由 OptionRow 持有原生 button、`role=option`、选中语义与基础交互底面，业务只组合行内容、密度和选择命令。
- `select-menu-view.tsx` 只渲染共享单选菜单，不读取业务状态或决定选值。
- `select-menu.tsx` 只编排共享单选语义和浮层生命周期；带搜索、异步状态或多选规则的菜单归真实业务所有者。
- `action-menu.tsx` 保持外部受控，不复用 Select 家族的内部开关状态；业务可显式选择与锚点起点或终点对齐。级联浮层复用 `UiActionMenuContent` 的条目和底部动作，不复制 Action Menu 行结构。
- Action Menu 首次焦点必须等定位完成、浮层实际可见后进入首个可用条目；后续滚动/窗口变化只更新几何，不把用户当前条目焦点重置到第一项。
- `menu-action-row.tsx` 是 Action Menu 与业务上下文菜单的唯一行级 DOM 所有者；它使用原生 button、`role=menuitem`、`aria-disabled`、有限密度和共享状态。业务只组合图标、标签、尾部内容与命令，不得重新导入菜单样式拼装按钮。
- Action Menu 的可选数组默认值必须引用模块级稳定空值；禁止在参数默认值中写 `[]`，否则锚定层的定位状态更新会让回调引用反复失效并形成 render loop。
- Action Menu 的重置等次级动作通过 `footerItems` 进入带分隔线的底部区域，不能混入主要选项伪装成普通值。
- Action Menu 默认普通行保持 36px，带一行短说明时使用 44px；纯单行选择可显式使用 `compact` 密度收至 32px，但带说明的权限菜单必须保留默认密度，不允许业务层用大块卡片高度破坏菜单节奏。
- Action、Select、Mention、Slash、级联模型与 Workspace 上下文菜单统一使用 4px 内容内边距和 2px 条目间距；间距由 `menu-styles.ts` 的共享列表合同提供，业务层不得按页面覆盖。浮层必须把间距计入内容高度，在自身上限内随条目增长，达到上限后才滚动。
- `menu-styles.ts` 统一 Action、Select 与上下文菜单的行级圆角、焦点和状态层级；业务菜单不得复制整套条目样式。
- 16px 外框配合 4px 内容内边距时，菜单行固定使用 12px 圆角，保证选中底面与外框同心。
- Action Menu 的活动项使用中性活动底面；`primary` 只控制文字或小型状态提示，不再给整行铺品牌色。
- Action Menu 标签统一使用正常字重；选择与危险状态由底面和色调表达，不靠加粗制造层级。
- 活动项悬浮时继续保持活动底面，避免鼠标经过反而降低当前位置的辨识度；非活动项 hover 才使用更轻一档的中性底。
- `menu.test.tsx` 使用真实 Portal、点击和键盘覆盖 Select 的 listbox 选择、disabled 跳过、Escape，以及 Action Menu 的初始焦点、方向键、Home/End、选择后焦点归还；源码断言不能替代这组合同。
- `select-menu-trigger.test.tsx` 单独验证共用触发器的原生按钮/表单、ref、受控 ARIA 和禁用/键盘委派；Room 多选测试继续证明 Chip 移除与菜单开关互不触发。

菜单组件直接导入，不通过另一个菜单文件隐式转出。锚点定位、材质与浏览器生命周期统一复用 `shared/ui/overlay/`；业务需要扩大弹层时传 `menuMinWidth`，不得开放或使用菜单表面样式类覆盖定位、圆角、内边距与条目节奏。Action Menu 的高度估算必须包含带说明行、共享条目间距与完整 surface 内边距，正常内容不得因估算偏小产生内部滚动，只有真实可用视口不足时才允许滚动。单行触发器保留完整活动标签的原生 `title` 兜底，并独立声明正常行高，避免继承紧凑按钮的 `leading-none` 后裁掉英文下行字形。
