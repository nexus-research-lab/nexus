// INPUT: 业务反馈事实，以及旧调用方尚未提供时的兜底恢复文案。
// OUTPUT: 标题、一句影响/下一步说明和至多一个动作的反馈合同。
// POS: 业务反馈到共享展示的类型适配边界；不推测请求结果。
import type { FeedbackBannerTone } from "./feedback-banner-model";

export type FeedbackBannerUrgency = "assertive" | "polite";

export interface FeedbackBannerAction {
  label: string;
  onClick: () => void;
  tone?: "danger" | "primary";
}

interface FeedbackBannerBaseProps {
  action?: FeedbackBannerAction;
  message: string;
  onDismiss?: () => void;
  title: string;
  urgency?: FeedbackBannerUrgency;
}

interface FeedbackBannerRecoveryProps extends FeedbackBannerBaseProps {
  impact: string;
  nextStep: string;
  tone: "error" | "warning";
}

interface FeedbackBannerNoticeProps extends FeedbackBannerBaseProps {
  impact?: string;
  nextStep?: string;
  tone: "info" | "success";
}

export type FeedbackBannerProps =
  | FeedbackBannerRecoveryProps
  | FeedbackBannerNoticeProps;

export interface FeedbackBannerInput extends FeedbackBannerBaseProps {
  impact?: string;
  nextStep?: string;
  tone: FeedbackBannerTone;
}

export interface FeedbackBannerRecoveryCopy {
  impact: string;
  nextStep: string;
}

export function completeFeedbackBanner(
  input: FeedbackBannerInput,
  recovery: FeedbackBannerRecoveryCopy,
): FeedbackBannerProps {
  if (input.tone === "error" || input.tone === "warning") {
    return {
      ...input,
      impact: input.impact?.trim() || recovery.impact,
      nextStep: input.nextStep?.trim() || recovery.nextStep,
      tone: input.tone,
    };
  }
  return {
    ...input,
    tone: input.tone,
  };
}
