// INPUT: 已由业务方确认的结果、当前影响、下一步和可选动作。
// OUTPUT: 不丢正文、按紧急程度播报且遵守持久性规则的全局反馈条。
// POS: 反馈展示边界；不推测请求结果，也不发起恢复请求。
import { useEffect, useRef } from "react";
import { ArrowRight, X } from "lucide-react";

import { cn } from "@/shared/ui/class-name";
import { UiButton } from "@/shared/ui/button/button";
import { useI18n } from "@/shared/i18n/i18n-context";

import {
  projectFeedbackBanner,
} from "./feedback-banner-model";
import type { FeedbackBannerProps } from "./feedback-banner-contract";

export function FeedbackBanner({
  action,
  impact,
  message,
  nextStep,
  onDismiss,
  title,
  tone,
  urgency = "polite",
}: FeedbackBannerProps) {
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
  }, [canAutoDismiss, impact, message, nextStep, presentation.autoDismissMs, title]);

  return (
    <div
      aria-atomic="true"
      aria-live={urgency}
      className={cn(
        "pointer-events-auto flex max-h-[calc(100dvh-6rem)] w-full min-w-0 max-w-[460px] items-start gap-3 overflow-y-auto rounded-[12px] border bg-[color:color-mix(in_srgb,var(--background)_94%,white)] px-4 py-3 shadow-(--surface-popover-shadow) sm:max-h-[calc(100dvh-7.5rem)] sm:min-w-[320px]",
        presentation.shellClassName,
      )}
      role={urgency === "assertive" ? "alert" : "status"}
    >
      <div
        className={cn(
          "mt-0.5 flex h-6 w-6 shrink-0 items-center justify-center rounded-full",
          presentation.iconClassName,
        )}
      >
        <Icon className="h-3.5 w-3.5" />
      </div>
      <div className="min-w-0 flex-1">
        <p className={cn("break-words text-compact font-semibold [overflow-wrap:anywhere]", presentation.titleClassName)}>
          {title}
        </p>
        <div className="mt-1 space-y-1.5 break-words [overflow-wrap:anywhere]">
          <p className="text-xs leading-5 text-(--text-default)">
            {message}
          </p>
          {impact ? (
            <p className="text-xs leading-5 text-(--text-muted)">
              {impact}
            </p>
          ) : null}
          {nextStep ? (
            <p className="text-xs font-medium leading-5 text-(--text-default)">
              {nextStep}
            </p>
          ) : null}
        </div>
        {action ? (
          <UiButton
            className="mt-3 max-w-full whitespace-normal text-left"
            onClick={action.onClick}
            size="xs"
            tone={action.tone === "danger" ? "danger" : "primary"}
            variant="text"
          >
            {action.label}
            <ArrowRight className="h-3 w-3" />
          </UiButton>
        ) : null}
      </div>
      {onDismiss ? (
        <button
          aria-label={t("common.close")}
          className="-mr-1 -mt-1 flex h-8 w-8 shrink-0 items-center justify-center rounded-[6px] text-(--icon-muted) transition-colors hover:bg-(--surface-interactive-hover-background) hover:text-(--icon-default) motion-reduce:transition-none"
          onClick={onDismiss}
          type="button"
        >
          <X className="h-3.5 w-3.5" />
        </button>
      ) : null}
    </div>
  );
}
