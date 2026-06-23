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

import { cn } from "@/lib/utils";

function split_feedback_items(message: string): string[] {
  return message
    .split(/[；\n]+/)
    .map((item) => item.trim())
    .filter(Boolean);
}

interface FeedbackBannerProps {
  tone: "success" | "warning" | "error";
  title: string;
  message: string;
  on_dismiss?: () => void;
}

export function FeedbackBanner({ tone, title, message, on_dismiss }: FeedbackBannerProps) {
  const items = split_feedback_items(message);
  const is_success = tone === "success";
  const is_warning = tone === "warning";
  const Icon = is_success ? CheckCircle2 : AlertCircle;
  const auto_dismiss_ms = is_success ? 2200 : is_warning ? 2800 : 3600;
  const shell_class_name = cn(
    "pointer-events-auto flex min-w-[280px] max-w-[420px] items-start gap-3 rounded-[12px] border bg-[color:color-mix(in_srgb,var(--background)_94%,white)] px-4 py-3 shadow-[0_10px_28px_rgba(0,0,0,0.10)]",
    is_success
      ? "border-[color:color-mix(in_srgb,var(--success)_22%,transparent)]"
      : is_warning
        ? "border-[color:color-mix(in_srgb,var(--warning)_22%,transparent)]"
        : "border-[color:color-mix(in_srgb,var(--destructive)_22%,transparent)]",
  );
  const icon_class_name = cn(
    "mt-0.5 flex h-6 w-6 shrink-0 items-center justify-center rounded-full",
    is_success
      ? "bg-[color:color-mix(in_srgb,var(--success)_12%,transparent)] text-(--success)"
      : is_warning
        ? "bg-[color:color-mix(in_srgb,var(--warning)_12%,transparent)] text-(--warning)"
        : "bg-[color:color-mix(in_srgb,var(--destructive)_12%,transparent)] text-(--destructive)",
  );
  const title_class_name = cn(
    "text-[12px] font-bold",
    is_success ? "text-(--success)" : is_warning ? "text-(--warning)" : "text-(--destructive)",
  );
  const item_class_name = cn(
    "inline-flex rounded-[6px] border bg-transparent px-2 py-0.5 text-[10px] font-medium",
    is_success ? "text-(--success)" : is_warning ? "text-(--warning)" : "text-(--destructive)",
    is_success
      ? "border-[color:color-mix(in_srgb,var(--success)_18%,transparent)]"
      : is_warning
        ? "border-[color:color-mix(in_srgb,var(--warning)_18%,transparent)]"
        : "border-[color:color-mix(in_srgb,var(--destructive)_18%,transparent)]",
  );

  useEffect(() => {
    if (!on_dismiss) {
      return;
    }
    const timer = window.setTimeout(() => {
      on_dismiss();
    }, auto_dismiss_ms);
    return () => {
      window.clearTimeout(timer);
    };
  }, [auto_dismiss_ms, on_dismiss]);

  return (
    <div className={shell_class_name}>
      <div className={icon_class_name}>
        <Icon className="h-3.5 w-3.5" />
      </div>
      <div className="min-w-0 flex-1">
        <p className={title_class_name}>
          {title}
        </p>
        {items.length > 1 ? (
          <div className="mt-1.5 flex flex-wrap gap-1.5">
            {items.map((item) => (
              <span
                key={item}
                className={item_class_name}
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
      {on_dismiss ? (
        <button
          className="shrink-0 text-[11px] text-(--text-muted) transition-colors hover:text-(--text-default)"
          onClick={on_dismiss}
          type="button"
        >
          ✕
        </button>
      ) : null}
    </div>
  );
}
