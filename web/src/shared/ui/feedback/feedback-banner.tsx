import { useEffect } from "react";
import { X } from "lucide-react";

import { cn } from "@/shared/ui/class-name";

import {
  type FeedbackBannerTone,
  projectFeedbackBanner,
} from "./feedback-banner-model";

export interface FeedbackBannerProps {
  message: string;
  onDismiss?: () => void;
  title: string;
  tone: FeedbackBannerTone;
}

export function FeedbackBanner({
  message,
  onDismiss,
  title,
  tone,
}: FeedbackBannerProps) {
  const presentation = projectFeedbackBanner(tone, message);
  const Icon = presentation.icon;

  useEffect(() => {
    if (!onDismiss) {
      return;
    }
    const timer = window.setTimeout(onDismiss, presentation.autoDismissMs);
    return () => {
      window.clearTimeout(timer);
    };
  }, [message, onDismiss, presentation.autoDismissMs, title]);

  return (
    <div
      className={cn(
        "pointer-events-auto flex min-w-[280px] max-w-[420px] items-start gap-3 rounded-[12px] border bg-[color:color-mix(in_srgb,var(--background)_94%,white)] px-4 py-3 shadow-[0_10px_28px_rgba(0,0,0,0.10)]",
        presentation.shellClassName,
      )}
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
        <p className={cn("text-[12px] font-bold", presentation.titleClassName)}>
          {title}
        </p>
        {presentation.items.length > 1 ? (
          <div className="mt-1.5 flex flex-wrap gap-1.5">
            {presentation.items.map((item) => (
              <span
                className={cn(
                  "inline-flex rounded-[6px] border bg-transparent px-2 py-0.5 text-[10px] font-medium",
                  presentation.itemClassName,
                )}
                key={item}
              >
                {item}
              </span>
            ))}
          </div>
        ) : (
          <p className="mt-0.5 text-[11px] text-(--text-soft)">
            {message}
          </p>
        )}
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
