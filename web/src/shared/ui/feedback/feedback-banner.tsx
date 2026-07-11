/**
 * =====================================================
 * @File   : feedback-banner.tsx
 * @Date   : 2026-04-16 13:35
 * @Author : leemysw
 * 2026-04-16 13:35   Create
 * =====================================================
 */

import { useEffect } from "react";
import { AlertCircle, CheckCircle2 } from "lucide-react";

import { cn } from "@/shared/ui/class-name";

function splitFeedbackItems(message: string): string[] {
  return message
    .split(/[；\n]+/)
    .map((item) => item.trim())
    .filter(Boolean);
}

interface FeedbackBannerProps {
  tone: "success" | "warning" | "error";
  title: string;
  message: string;
  onDismiss?: () => void;
}

export function FeedbackBanner({ tone, title, message, onDismiss: onDismiss }: FeedbackBannerProps) {
  const items = splitFeedbackItems(message);
  const isSuccess = tone === "success";
  const isWarning = tone === "warning";
  const Icon = isSuccess ? CheckCircle2 : AlertCircle;
  const autoDismissMs = isSuccess ? 2200 : isWarning ? 2800 : 3600;
  const shellClassName = cn(
    "pointer-events-auto flex min-w-[280px] max-w-[420px] items-start gap-3 rounded-[12px] border bg-[color:color-mix(in_srgb,var(--background)_94%,white)] px-4 py-3 shadow-[0_10px_28px_rgba(0,0,0,0.10)]",
    isSuccess
      ? "border-[color:color-mix(in_srgb,var(--success)_22%,transparent)]"
      : isWarning
        ? "border-[color:color-mix(in_srgb,var(--warning)_22%,transparent)]"
        : "border-[color:color-mix(in_srgb,var(--destructive)_22%,transparent)]",
  );
  const iconClassName = cn(
    "mt-0.5 flex h-6 w-6 shrink-0 items-center justify-center rounded-full",
    isSuccess
      ? "bg-[color:color-mix(in_srgb,var(--success)_12%,transparent)] text-(--success)"
      : isWarning
        ? "bg-[color:color-mix(in_srgb,var(--warning)_12%,transparent)] text-(--warning)"
        : "bg-[color:color-mix(in_srgb,var(--destructive)_12%,transparent)] text-(--destructive)",
  );
  const titleClassName = cn(
    "text-[12px] font-bold",
    isSuccess ? "text-(--success)" : isWarning ? "text-(--warning)" : "text-(--destructive)",
  );
  const itemClassName = cn(
    "inline-flex rounded-[6px] border bg-transparent px-2 py-0.5 text-[10px] font-medium",
    isSuccess ? "text-(--success)" : isWarning ? "text-(--warning)" : "text-(--destructive)",
    isSuccess
      ? "border-[color:color-mix(in_srgb,var(--success)_18%,transparent)]"
      : isWarning
        ? "border-[color:color-mix(in_srgb,var(--warning)_18%,transparent)]"
        : "border-[color:color-mix(in_srgb,var(--destructive)_18%,transparent)]",
  );

  useEffect(() => {
    if (!onDismiss) {
      return;
    }
    const timer = window.setTimeout(() => {
      onDismiss();
    }, autoDismissMs);
    return () => {
      window.clearTimeout(timer);
    };
  }, [autoDismissMs, onDismiss]);

  return (
    <div className={shellClassName}>
      <div className={iconClassName}>
        <Icon className="h-3.5 w-3.5" />
      </div>
      <div className="min-w-0 flex-1">
        <p className={titleClassName}>
          {title}
        </p>
        {items.length > 1 ? (
          <div className="mt-1.5 flex flex-wrap gap-1.5">
            {items.map((item) => (
              <span
                key={item}
                className={itemClassName}
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
          className="shrink-0 text-[11px] text-(--text-muted) transition-colors hover:text-(--text-default)"
          onClick={onDismiss}
          type="button"
        >
          ✕
        </button>
      ) : null}
    </div>
  );
}
