// INPUT: 业务反馈事实，以及旧调用方尚未提供时的兜底恢复文案。
// OUTPUT: 标题、一句说明和至多一个直接动作的反馈合同。
// POS: 业务反馈到共享展示的类型适配边界；不推测请求结果。

export type FeedbackBannerTone = "info" | "success" | "warning" | "error";

export type FeedbackBannerUrgency = "assertive" | "polite";

export interface FeedbackBannerAction {
  label: string;
  onClick: () => void;
  tone?: "danger" | "primary";
}

interface FeedbackBannerBaseProps {
  onDismiss?: () => void;
  title: string;
  urgency?: FeedbackBannerUrgency;
}

interface FeedbackBannerRecoveryProps extends FeedbackBannerBaseProps {
  action?: FeedbackBannerAction;
  impact: string;
  message?: never;
  nextStep?: string;
  tone: "error" | "warning";
}

interface FeedbackBannerMessageNoticeProps extends FeedbackBannerBaseProps {
  action?: FeedbackBannerAction;
  impact?: never;
  message: string;
  nextStep?: never;
  tone: "info" | "success";
}

interface FeedbackBannerGuidedNoticeProps extends FeedbackBannerBaseProps {
  action?: never;
  impact: string;
  message?: never;
  nextStep: string;
  tone: "info" | "success";
}

export type FeedbackBannerProps =
  | FeedbackBannerRecoveryProps
  | FeedbackBannerMessageNoticeProps
  | FeedbackBannerGuidedNoticeProps;

interface FeedbackBannerRecoveryInput extends FeedbackBannerBaseProps {
  action?: FeedbackBannerAction;
  impact?: string;
  nextStep?: string;
  message?: never;
  tone: Extract<FeedbackBannerTone, "error" | "warning">;
}

interface FeedbackBannerNoticeInput extends FeedbackBannerBaseProps {
  action?: FeedbackBannerAction;
  impact?: string;
  message?: string;
  nextStep?: string;
  tone: Extract<FeedbackBannerTone, "info" | "success">;
}

export type FeedbackBannerInput =
  | FeedbackBannerRecoveryInput
  | FeedbackBannerNoticeInput;

export interface FeedbackBannerRecoveryCopy {
  impact: string;
}

export function completeFeedbackBanner(
  input: FeedbackBannerInput,
  recovery: FeedbackBannerRecoveryCopy,
): FeedbackBannerProps {
  if (input.tone === "error" || input.tone === "warning") {
    const { action, nextStep, ...recoveryInput } = input;
    const impact = input.impact?.trim() || recovery.impact;
    if (action) {
      return {
        ...recoveryInput,
        action,
        impact,
        tone: input.tone,
      };
    }
    const guidance = nextStep?.trim();
    return {
      ...recoveryInput,
      impact,
      tone: input.tone,
      ...(guidance
        ? { nextStep: guidance }
        : {}),
    };
  }
  if (!input.action && input.impact?.trim() && input.nextStep?.trim()) {
    const { action: _action, message: _message, ...guidedInput } = input;
    return {
      ...guidedInput,
      impact: input.impact.trim(),
      nextStep: input.nextStep.trim(),
      tone: input.tone,
    };
  }
  const { impact: _impact, nextStep: _nextStep, ...messageInput } = input;
  return {
    ...messageInput,
    message: input.message?.trim() || input.title,
    tone: input.tone,
  };
}
