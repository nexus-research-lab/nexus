import {
  AlertCircle,
  CheckCircle2,
  type LucideIcon,
} from "lucide-react";

export type FeedbackBannerTone = "success" | "warning" | "error";

interface FeedbackTonePresentation {
  autoDismissMs: number;
  icon: LucideIcon;
  iconClassName: string;
  itemClassName: string;
  shellClassName: string;
  titleClassName: string;
}

export interface FeedbackBannerPresentation extends FeedbackTonePresentation {
  items: string[];
}

const FEEDBACK_TONE_PRESENTATION: Record<
  FeedbackBannerTone,
  FeedbackTonePresentation
> = {
  success: {
    autoDismissMs: 2200,
    icon: CheckCircle2,
    iconClassName: "bg-[color:color-mix(in_srgb,var(--success)_12%,transparent)] text-(--success)",
    itemClassName: "border-[color:color-mix(in_srgb,var(--success)_18%,transparent)] text-(--success)",
    shellClassName: "border-[color:color-mix(in_srgb,var(--success)_22%,transparent)]",
    titleClassName: "text-(--success)",
  },
  warning: {
    autoDismissMs: 2800,
    icon: AlertCircle,
    iconClassName: "bg-[color:color-mix(in_srgb,var(--warning)_12%,transparent)] text-(--warning)",
    itemClassName: "border-[color:color-mix(in_srgb,var(--warning)_18%,transparent)] text-(--warning)",
    shellClassName: "border-[color:color-mix(in_srgb,var(--warning)_22%,transparent)]",
    titleClassName: "text-(--warning)",
  },
  error: {
    autoDismissMs: 3600,
    icon: AlertCircle,
    iconClassName: "bg-[color:color-mix(in_srgb,var(--destructive)_12%,transparent)] text-(--destructive)",
    itemClassName: "border-[color:color-mix(in_srgb,var(--destructive)_18%,transparent)] text-(--destructive)",
    shellClassName: "border-[color:color-mix(in_srgb,var(--destructive)_22%,transparent)]",
    titleClassName: "text-(--destructive)",
  },
};

export function projectFeedbackBanner(
  tone: FeedbackBannerTone,
  message: string,
): FeedbackBannerPresentation {
  return {
    ...FEEDBACK_TONE_PRESENTATION[tone],
    items: message
      .split(/[；\n]+/)
      .map((item) => item.trim())
      .filter(Boolean),
  };
}
