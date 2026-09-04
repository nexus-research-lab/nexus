// INPUT: 已由业务方确认的结果、单句说明和可选直接动作。
// OUTPUT: 标题、一句说明和至多一个动作的全局反馈条。
// POS: 反馈展示边界；不推测请求结果，也不发起恢复请求。
import { useEffect, useRef } from "react";
import {
  AlertCircle,
  CheckCircle2,
  Info,
  X,
  type LucideIcon,
} from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton, UiIconButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import {
  getUiToneClassName,
  getUiTypographyClassName,
  type UiTypographyTone,
} from "@/shared/ui/typography/typography-styles";

import {
  resolveFeedbackBannerPolicy,
} from "./feedback-banner-model";
import type {
  FeedbackBannerProps,
  FeedbackBannerTone,
} from "./feedback-banner-contract";
import { RecoverySummary } from "./recovery-summary";

const FEEDBACK_ICON_BY_TONE: Record<FeedbackBannerTone, LucideIcon> = {
  error: AlertCircle,
  info: Info,
  success: CheckCircle2,
  warning: AlertCircle,
};

const FEEDBACK_ICON_TONE: Record<FeedbackBannerTone, UiTypographyTone> = {
  error: "danger",
  info: "brand",
  success: "success",
  warning: "warning",
};

export function FeedbackBanner({
  ...props
}: FeedbackBannerProps) {
  const {
    action,
    impact,
    nextStep,
    onDismiss,
    title,
    tone,
    urgency = "polite",
  } = props;
  const noticeMessage = "message" in props ? props.message : null;
  const { t } = useI18n();
  const policy = resolveFeedbackBannerPolicy(tone, Boolean(action));
  const Icon = FEEDBACK_ICON_BY_TONE[tone];
  const onDismissRef = useRef(onDismiss);
  const canAutoDismiss = Boolean(onDismiss)
    && policy.autoDismissMs !== null
    && !impact
    && !nextStep;

  useEffect(() => {
    onDismissRef.current = onDismiss;
  }, [onDismiss]);

  useEffect(() => {
    if (!canAutoDismiss || policy.autoDismissMs === null) {
      return;
    }
    const timer = window.setTimeout(() => {
      onDismissRef.current?.();
    }, policy.autoDismissMs);
    return () => {
      window.clearTimeout(timer);
    };
  }, [canAutoDismiss, impact, nextStep, noticeMessage, policy.autoDismissMs, title]);

  return (
    <div
      aria-atomic="true"
      aria-live={urgency}
      className="surface-popover surface-radius-md pointer-events-auto flex max-h-[calc(100dvh-6rem)] w-full min-w-0 max-w-[420px] items-start gap-2.5 overflow-y-auto px-3.5 py-3 sm:max-h-[calc(100dvh-7.5rem)] sm:min-w-[320px]"
      role={urgency === "assertive" ? "alert" : "status"}
    >
      <Icon className={cn(
        "mt-0.5 h-4 w-4 shrink-0",
        getUiToneClassName(FEEDBACK_ICON_TONE[tone]),
      )} />
      <div className="min-w-0 flex-1">
        <p className={cn(
          "break-words [overflow-wrap:anywhere]",
          getUiTypographyClassName({
            role: "supporting",
            tone: "strong",
            weight: "medium",
          }),
        )}>
          {title}
        </p>
        {impact ? (
          <RecoverySummary
            className="mt-0.5"
            impact={impact}
            nextStep={action ? undefined : nextStep}
          />
        ) : (
          <p className={cn(
            "mt-0.5 break-words [overflow-wrap:anywhere]",
            getUiTypographyClassName({ role: "metadata", tone: "muted" }),
          )}>
            {noticeMessage}
          </p>
        )}
        {action ? (
          <UiButton
            className="mt-1.5 max-w-full whitespace-normal text-left"
            onClick={action.onClick}
            size="xs"
            tone={action.tone === "danger" ? "danger" : "primary"}
            variant="text"
          >
            {action.label}
          </UiButton>
        ) : null}
      </div>
      {onDismiss ? (
        <UiIconButton
          aria-label={t("common.close")}
          className="-mr-2 -mt-2 shrink-0 motion-reduce:transition-none"
          onClick={onDismiss}
          size="lg"
          variant="ghost"
        >
          <X className="h-3.5 w-3.5" />
        </UiIconButton>
      ) : null}
    </div>
  );
}
