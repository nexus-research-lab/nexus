# Shared Overlay

- `anchored-overlay-model.ts` 只计算锚点浮层在视口内的位置与尺寸。
- `anchored-overlay-layer.ts` 统一 Portal 容器、外部点击、Escape、滚动和窗口变化生命周期。
- `overlay-contract.ts` 定义打开态 DOM 契约，供嵌套 Dialog 判断 Escape 的唯一消费层。

本目录不解释菜单、选择器或业务内容。消费者提供定位参数和关闭命令，浮层语义仍归消费者所有。
