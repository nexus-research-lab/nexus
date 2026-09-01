// INPUT: 已由业务方确认的结果、单句说明和可选直接动作。
// OUTPUT: 标题、一句说明和至多一个动作的全局反馈条。
// POS: 反馈展示边界；不推测请求结果，也不发起恢复请求。
import { useEffect, useRef } from "react";
import { X } from "lucide-react";

import { cn } from "@/shared/ui/class-name";
import { UiButton } from "@/shared/ui/button/button";
import { useI18n } from "@/shared/i18n/i18n-context";

import {
  projectFeedbackBanner,
} from "./feedback-banner-model";
import type { FeedbackBannerProps } from "./feedback-banner-contract";
import { RecoverySummary } from "./recovery-summary";

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
  const presentation = projectFeedbackBanner(tone, Boolean(action));
  const Icon = presentation.icon;
  const onDismissRef = useRef(onDismiss);
  const canAutoDismiss = Boolean(onDismiss)
    && presentation.autoDismissMs !== null
    && !impact
    && !nextStep;

  useEffect(() => {
    onDismissRef.current = onDismiss;
  }, [onDismiss]);

  useEffect(() => {
    if (!canAutoDismiss || presentation.autoDismissMs === null) {
      return;
    }
    const timer = window.setTimeout(() => {
      onDismissRef.current?.();
    }, presentation.autoDismissMs);
    return () => {
      window.clearTimeout(timer);
    };
  }, [canAutoDismiss, impact, nextStep, noticeMessage, presentation.autoDismissMs, title]);

  return (
    <div
      aria-atomic="true"
      aria-live={urgency}
      className={cn(
        "pointer-events-auto flex max-h-[calc(100dvh-6rem)] w-full min-w-0 max-w-[420px] items-start gap-2.5 overflow-y-auto rounded-[10px] border bg-[color:color-mix(in_srgb,var(--surface-panel-background)_97%,white)] px-3.5 py-3 shadow-[0_6px_24px_color-mix(in_srgb,var(--shadow-color)_9%,transparent)] sm:max-h-[calc(100dvh-7.5rem)] sm:min-w-[320px]",
        presentation.shellClassName,
      )}
      role={urgency === "assertive" ? "alert" : "status"}
    >
      <Icon className={cn("mt-0.5 h-4 w-4 shrink-0", presentation.iconClassName)} />
      <div className="min-w-0 flex-1">
        <p className={cn("break-words text-[13px] font-medium leading-5 [overflow-wrap:anywhere]", presentation.titleClassName)}>
          {title}
        </p>
        {impact ? (
          <RecoverySummary
            className="mt-0.5"
            impact={impact}
            nextStep={action ? undefined : nextStep}
          />
        ) : (
          <p className="mt-0.5 break-words text-xs leading-5 text-(--text-muted) [overflow-wrap:anywhere]">
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
        <button
          aria-label={t("common.close")}
          className="-mr-2 -mt-2 flex h-9 w-9 shrink-0 items-center justify-center rounded-[7px] text-(--icon-muted) transition-colors hover:bg-(--surface-interactive-hover-background) hover:text-(--icon-default) motion-reduce:transition-none"
          onClick={onDismiss}
          type="button"
        >
          <X className="h-3.5 w-3.5" />
        </button>
      ) : null}
    </div>
  );
}
