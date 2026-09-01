# 反馈展示原语

- `feedback-banner-model.ts` 用 tone 定义表统一图标、视觉类名和自动关闭规则；error、warning 与带动作的消息保持显示，只有无动作的 success、info 可以限时收起。
- `feedback-banner-contract.ts` 定义 error、warning 只提供一句正文和可选直接动作；没有动作和没有额外恢复说明都是合法状态。info、success 在单句 `message` 与引导式正文之间二选一，不得同时提交两份正文。`completeFeedbackBanner` 只为旧业务反馈补齐正文，不生成恢复建议。
- `feedback-banner.tsx` 自然排版正文，不渲染审计标签或分段标签。紧急程度独立于 tone，默认礼貌播报，只有业务明确要求立即打断时才使用 assertive。
- `feedback-banner-viewport.tsx` 提供固定定位边界和窄屏重排；当前产品只有单条反馈，不维护伪堆叠协议。
- 业务域负责生成标题、唯一正文、可选动作和关闭命令；有动作时展示层不再重复同义恢复说明，共享层不解释业务反馈类型。
- 恢复动作默认使用产品主色；只有会删除、覆盖或撤销数据的动作才能显式声明 danger。
- 消息内容变化必须重置自动消失计时，不能沿用上一条反馈的生命周期。
