# Shared Overlay

- `anchored-overlay-model.ts` 只计算锚点浮层在视口内的位置、尺寸与起点/终点对齐；它接收已经解析的数字约束，不认识产品语义。
- `anchored-overlay-layout.ts` 是锚定浮层 geometry preset 的唯一 owner：`directory-list / reference-list / form-picker / status-summary / status-list / cascade-menu / command-list / command-picker` 固定既有 gap、视口内边距与宽高边界。消费者只选择语义 preset，并按需提供内容估算高度、方向和对齐，不得重新散落同组数字。
- `anchored-overlay-layer.ts` 统一 Portal 容器、外部点击、Escape、滚动和窗口变化生命周期；默认向锚点归还焦点，包装型 primitive 必须显式提供真实触发器的焦点归还策略。
- `overlay-contract.ts` 定义打开态 DOM 契约，供嵌套 Dialog 判断 Escape 的唯一消费层。
- `overlay-styles.ts` 只定义锚点浮层共用的材质与进出场；进场只动画 `opacity`/`transform`，定位层提交的 `left`/`top`/`bottom` 几何不得参与 transition；层级、尺寸和内容语义仍由消费者决定。
- `layer-styles.ts` 把 select/action menu、popover、dialog、嵌套交互、tooltip、tour 与系统弹窗映射到主题 layer token；消费者选择语义层，不挑选或递增 z-index 整数。
- `tooltip.tsx` 复用锚点定位与 Portal 生命周期，统一短延迟 hover、键盘 focus、Escape 焦点归还和深色轻量提示；业务按钮只提供可访问标签与可选快捷键。

本目录不解释菜单、选择器或业务内容。消费者提供定位语义和关闭命令，浮层内容仍归消费者所有；确有新几何时先与现有 preset 比较，只有存在稳定布局差异才新增 preset，并同步补齐合同测试。
定位层只在几何值真实变化时写 state；相同位置必须保持原对象，不能让不稳定的业务回调放大成 React render loop。
定位完成后必须把未使用的 `top`/`bottom` 轴显式重置为 `auto`，避免消费者的初始原点 class 与最终坐标同时生效。
