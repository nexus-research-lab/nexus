// INPUT: 反馈语气，以及当前消息是否带有需要用户处理的动作。
// OUTPUT: 是否允许自动收起及其稳定时长。
// POS: 全局反馈条的纯生命周期策略；不返回图标、颜色、视觉类或业务恢复动作。

import type { FeedbackBannerTone } from "./feedback-banner-contract";

interface FeedbackBannerPolicy {
  autoDismissMs: number | null;
}

const FEEDBACK_AUTO_DISMISS_MS: Record<
  FeedbackBannerTone,
  number | null
> = {
  error: null,
  info: 5000,
  success: 4000,
  warning: null,
};

export function resolveFeedbackBannerPolicy(
  tone: FeedbackBannerTone,
  hasAction: boolean,
): FeedbackBannerPolicy {
  return {
    autoDismissMs: hasAction ? null : FEEDBACK_AUTO_DISMISS_MS[tone],
  };
}
