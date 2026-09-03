# Shared Overlay

- `anchored-overlay-model.ts` 只计算锚点浮层在视口内的位置、尺寸与起点/终点对齐。
- `anchored-overlay-layer.ts` 统一 Portal 容器、外部点击、Escape、滚动和窗口变化生命周期。
- `overlay-contract.ts` 定义打开态 DOM 契约，供嵌套 Dialog 判断 Escape 的唯一消费层。
- `overlay-styles.ts` 只定义锚点浮层共用的材质与进出场；进场只动画 `opacity`/`transform`，定位层提交的 `left`/`top`/`bottom` 几何不得参与 transition；层级、尺寸和内容语义仍由消费者决定。
- `layer-styles.ts` 把 select/action menu、popover、dialog、嵌套交互、tooltip、tour 与系统弹窗映射到主题 layer token；消费者选择语义层，不挑选或递增 z-index 整数。
- `tooltip.tsx` 复用锚点定位与 Portal 生命周期，统一短延迟 hover、键盘 focus 和深色轻量提示；业务按钮只提供可访问标签与可选快捷键。

本目录不解释菜单、选择器或业务内容。消费者提供定位参数和关闭命令，浮层语义仍归消费者所有。
定位层只在几何值真实变化时写 state；相同位置必须保持原对象，不能让不稳定的业务回调放大成 React render loop。
定位完成后必须把未使用的 `top`/`bottom` 轴显式重置为 `auto`，避免消费者的初始原点 class 与最终坐标同时生效。
