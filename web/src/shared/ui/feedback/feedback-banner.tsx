import { useEffect } from "react";
import { ArrowRight, X } from "lucide-react";

import { cn } from "@/shared/ui/class-name";
import { UiButton } from "@/shared/ui/button/button";
import { useI18n } from "@/shared/i18n/i18n-context";

import {
  type FeedbackBannerTone,
  projectFeedbackBanner,
} from "./feedback-banner-model";

export interface FeedbackBannerProps {
  action?: {
    label: string;
    onClick: () => void;
  };
  impact?: string;
  message: string;
  nextStep?: string;
  onDismiss?: () => void;
  title: string;
  tone: FeedbackBannerTone;
}

export function FeedbackBanner({
  action,
  impact,
  message,
  nextStep,
  onDismiss,
  title,
  tone,
}: FeedbackBannerProps) {
  const { t } = useI18n();
  const presentation = projectFeedbackBanner(tone, message);
  const Icon = presentation.icon;

  useEffect(() => {
    if (!onDismiss || action) {
      return;
    }
    const timer = window.setTimeout(onDismiss, presentation.autoDismissMs);
    return () => {
      window.clearTimeout(timer);
    };
  }, [action, message, onDismiss, presentation.autoDismissMs, title]);

  return (
    <div
      className={cn(
        "pointer-events-auto flex min-w-[280px] max-w-[420px] items-start gap-3 rounded-[12px] border bg-[color:color-mix(in_srgb,var(--background)_94%,white)] px-4 py-3 shadow-(--surface-popover-shadow)",
        presentation.shellClassName,
      )}
      role={tone === "error" ? "alert" : "status"}
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
        <p className={cn("text-compact font-semibold", presentation.titleClassName)}>
          {title}
        </p>
        {presentation.items.length > 1 ? (
          <div className="mt-1.5 flex flex-wrap gap-1.5">
            {presentation.items.map((item) => (
              <span
                className={cn(
                  "inline-flex rounded-[6px] border bg-transparent px-2 py-0.5 text-2xs font-medium",
                  presentation.itemClassName,
                )}
                key={item}
              >
                {item}
              </span>
            ))}
          </div>
        ) : (
          <p className="mt-0.5 text-xs text-(--text-soft)">
            {message}
          </p>
        )}
        {impact ? (
          <p className="mt-1.5 text-2xs leading-4 text-(--text-muted)">
            <span className="font-semibold text-(--text-default)">{t("state.existing_data")}：</span>
            {impact}
          </p>
        ) : null}
        {nextStep ? (
          <p className="mt-0.5 text-2xs leading-4 text-(--text-muted)">
            <span className="font-semibold text-(--text-default)">{t("state.next_step")}：</span>
            {nextStep}
          </p>
        ) : null}
        {action ? (
          <UiButton
            className="mt-2"
            onClick={action.onClick}
            size="xs"
            tone={tone === "error" ? "danger" : "primary"}
            variant="text"
          >
            {action.label}
            <ArrowRight className="h-3 w-3" />
          </UiButton>
        ) : null}
      </div>
      {onDismiss ? (
        <button
          aria-label="关闭反馈"
          className="shrink-0 rounded-[6px] p-0.5 text-(--icon-muted) transition-colors hover:bg-(--surface-interactive-hover-background) hover:text-(--icon-default)"
          onClick={onDismiss}
          type="button"
        >
          <X className="h-3.5 w-3.5" />
        </button>
      ) : null}
    </div>
  );
}
