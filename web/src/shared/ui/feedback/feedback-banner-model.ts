// INPUT: 反馈语气，以及当前消息是否带有需要用户处理的动作。
// OUTPUT: 统一的图标、语义色和是否允许自动收起的展示规则。
// POS: 全局反馈条的纯展示模型；不解释业务结果或恢复动作。
import {
  AlertCircle,
  CheckCircle2,
  Info,
  type LucideIcon,
} from "lucide-react";

export type FeedbackBannerTone = "info" | "success" | "warning" | "error";

interface FeedbackTonePresentation {
  autoDismissMs: number | null;
  icon: LucideIcon;
  iconClassName: string;
  shellClassName: string;
  titleClassName: string;
}

export type FeedbackBannerPresentation = FeedbackTonePresentation;

const FEEDBACK_TONE_PRESENTATION: Record<
  FeedbackBannerTone,
  FeedbackTonePresentation
> = {
  info: {
    autoDismissMs: 5000,
    icon: Info,
    iconClassName: "text-(--brand-action)",
    shellClassName: "border-(--surface-panel-border)",
    titleClassName: "text-(--text-strong)",
  },
  success: {
    autoDismissMs: 4000,
    icon: CheckCircle2,
    iconClassName: "text-(--success)",
    shellClassName: "border-(--surface-panel-border)",
    titleClassName: "text-(--text-strong)",
  },
  warning: {
    autoDismissMs: null,
    icon: AlertCircle,
    iconClassName: "text-(--warning)",
    shellClassName: "border-(--surface-panel-border)",
    titleClassName: "text-(--text-strong)",
  },
  error: {
    autoDismissMs: null,
    icon: AlertCircle,
    iconClassName: "text-(--destructive)",
    shellClassName: "border-(--surface-panel-border)",
    titleClassName: "text-(--text-strong)",
  },
};

export function projectFeedbackBanner(
  tone: FeedbackBannerTone,
  hasAction: boolean,
): FeedbackBannerPresentation {
  return {
    ...FEEDBACK_TONE_PRESENTATION[tone],
    autoDismissMs: hasAction
      ? null
      : FEEDBACK_TONE_PRESENTATION[tone].autoDismissMs,
  };
}
