// INPUT: 业务反馈事实，以及为旧反馈模型显式提供的恢复文案。
// OUTPUT: error、warning 必含当前影响和下一步的严格反馈展示合同。
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
