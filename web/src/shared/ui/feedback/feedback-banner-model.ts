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
    iconClassName: "bg-[color:color-mix(in_srgb,var(--brand-action)_10%,transparent)] text-(--brand-action)",
    shellClassName: "border-[color:color-mix(in_srgb,var(--brand-action)_18%,transparent)]",
    titleClassName: "text-(--text-strong)",
  },
  success: {
    autoDismissMs: 4000,
    icon: CheckCircle2,
    iconClassName: "bg-[color:color-mix(in_srgb,var(--success)_12%,transparent)] text-(--success)",
    shellClassName: "border-[color:color-mix(in_srgb,var(--success)_22%,transparent)]",
    titleClassName: "text-(--success)",
  },
  warning: {
    autoDismissMs: null,
    icon: AlertCircle,
    iconClassName: "bg-[color:color-mix(in_srgb,var(--warning)_12%,transparent)] text-(--warning)",
    shellClassName: "border-[color:color-mix(in_srgb,var(--warning)_22%,transparent)]",
    titleClassName: "text-(--text-strong)",
  },
  error: {
    autoDismissMs: null,
    icon: AlertCircle,
    iconClassName: "bg-[color:color-mix(in_srgb,var(--destructive)_12%,transparent)] text-(--destructive)",
    shellClassName: "border-[color:color-mix(in_srgb,var(--destructive)_22%,transparent)]",
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
