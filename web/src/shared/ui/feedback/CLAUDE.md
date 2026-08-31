# 反馈展示原语

- `feedback-banner-model.ts` 用 tone 定义表统一图标、视觉类名和自动关闭规则；error、warning 与带动作的消息保持显示，只有无动作的 success、info 可以限时收起。
- `feedback-banner-contract.ts` 定义 error、warning 必须同时提供当前影响和下一步的类型合同；`completeFeedbackBanner` 只用于把既有业务反馈模型接入该合同，调用方必须显式提供与当前业务相符的影响和下一步兜底，不得由组件猜测。
- `feedback-banner.tsx` 自然排版正文，不渲染审计标签或分段标签。紧急程度独立于 tone，默认礼貌播报，只有业务明确要求立即打断时才使用 assertive。
- `feedback-banner-viewport.tsx` 提供固定定位边界和窄屏重排；当前产品只有单条反馈，不维护伪堆叠协议。
- 业务域负责生成标题、消息、当前影响、下一步和关闭命令；共享层不解释业务反馈类型。
- 恢复动作默认使用产品主色；只有会删除、覆盖或撤销数据的动作才能显式声明 danger。
- 消息内容变化必须重置自动消失计时，不能沿用上一条反馈的生命周期。
