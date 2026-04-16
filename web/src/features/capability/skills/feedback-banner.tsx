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
    "pointer-events-auto flex min-w-[280px] max-w-[420px] items-start gap-3 rounded-[18px] border bg-(--surface-panel-background) px-4 py-3 shadow-[0_18px_40px_rgba(0,0,0,0.12)]",
    is_success ? "border-emerald-500/20" : is_warning ? "border-amber-500/20" : "border-rose-500/20",
  );
  const icon_class_name = cn(
    "mt-0.5 flex h-6 w-6 shrink-0 items-center justify-center rounded-full",
    is_success ? "bg-emerald-500/12 text-emerald-500" : is_warning ? "bg-amber-500/12 text-amber-600" : "bg-rose-500/12 text-rose-500",
  );
  const title_class_name = cn(
    "text-[12px] font-bold",
    is_success ? "text-emerald-500" : is_warning ? "text-amber-600" : "text-rose-500",
  );
  const item_class_name = cn(
    "inline-flex rounded-full bg-(--chip-default-background) border border-(--chip-default-border) px-2 py-0.5 text-[10px] font-medium",
    is_success ? "text-emerald-500" : is_warning ? "text-amber-600" : "text-rose-500",
  );

  useEffect(() => {
    if (!on_dismiss) return;
    const timer = window.setTimeout(() => {
      on_dismiss();
    }, auto_dismiss_ms);
    return () => {
      window.clearTimeout(timer);
    };
  }, [auto_dismiss_ms, on_dismiss]);

  return (
    <div
      className={shell_class_name}
    >
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
          <p className={cn("mt-0.5 text-[11px] text-(--text-soft)")}>
            {message}
          </p>
        )}
      </div>
      {on_dismiss && (
        <button
          className="shrink-0 text-[11px] text-(--text-muted) transition-colors hover:text-(--text-default)"
          onClick={on_dismiss}
          type="button"
        >
          ✕
        </button>
      )}
    </div>
  );
}
