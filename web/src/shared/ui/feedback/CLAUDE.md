# 反馈展示原语

- `feedback-banner-contract.ts` 拥有 tone 与业务输入边界；`feedback-banner-model.ts` 只计算自动关闭生命周期，不得返回图标、颜色、className 或业务恢复动作。error、warning 与带动作的消息保持显示，只有无动作的 success、info 可以限时收起。
- `feedback-banner-contract.ts` 定义 error、warning 只提供一句正文和可选直接动作；没有动作和没有额外恢复说明都是合法状态。info、success 在单句 `message` 与引导式正文之间二选一，不得同时提交两份正文。`completeFeedbackBanner` 只为旧业务反馈补齐正文，不生成恢复建议。
- `feedback-banner.tsx` 独占 tone 到图标及语义前景色的渲染映射，自然排版正文，不渲染审计标签或分段标签，并复用 `surface-popover + surface-radius-md`、App Typography 与共享 Icon Button，不自行定义阴影、材质、圆角或字号。紧急程度独立于 tone，默认礼貌播报，只有业务明确要求立即打断时才使用 assertive。
- `feedback-banner-viewport.tsx` 提供固定定位边界、窄屏重排和 `feedback` 语义 layer；当前产品只有单条反馈，不维护伪堆叠协议。
- `inline-notice.tsx` 是页面内容流和 Conversation 状态层的唯一紧凑提示骨架；`contained / edge` 只表达容器关系，tone 只表达图标与轻量背景语义，`full / compact` 统一拥有可用列宽与短状态阅读宽度，标题、影响说明和至多一个恢复动作统一复用 Typography 与 `UiButton`。业务页面不得再手写提示条圆角、背景、宽度或动作状态。
- 业务域负责生成标题、唯一正文、可选动作和关闭命令；有动作时展示层不再重复同义恢复说明，共享层不解释业务反馈类型。
- 恢复动作默认使用产品主色；只有会删除、覆盖或撤销数据的动作才能显式声明 danger。
- 消息内容变化必须重置自动消失计时，不能沿用上一条反馈的生命周期。
- `feedback.test.tsx` 固定播报语义、单一恢复动作、关闭动作、消息变化后的计时重置以及命名 layer，基础组件修改不得只依赖页面肉眼回归。
- `inline-notice.test.tsx` 固定 contained / edge、tone、full / compact 阅读宽度、长文本换行、单一动作和 pending 禁用合同；业务域仍需测试错误事实如何映射到这些有限视觉输入。
- `loading-orb.tsx` 以 `active / preparing` 两种语义 variant 统一字符帧、周期和 reduced-motion 静态首帧，外框固定为 `12×12px`，所有字符帧绝对定位，不得让字形度量参与行高。Composer 与消息活动行必须共用该 owner，并为其保留固定宽高。动效规则预置在主题 recipe，禁止消费者传私有帧数组、为状态文案添加流光 / pulse，或在渲染时向 `document.head` 注入样式。
