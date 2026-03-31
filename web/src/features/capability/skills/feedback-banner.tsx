import { useEffect } from "react";
import { AlertCircle, CheckCircle2 } from "lucide-react";

import { cn } from "@/lib/utils";

function splitFeedbackItems(message: string): string[] {
  return message
    .split(/[；\n]+/)
    .map((item) => item.trim())
    .filter(Boolean);
}

interface FeedbackBannerProps {
  tone: "success" | "error";
  title: string;
  message: string;
  on_dismiss?: () => void;
}

export function FeedbackBanner({ tone, title, message, on_dismiss }: FeedbackBannerProps) {
  const items = splitFeedbackItems(message);
  const is_success = tone === "success";
  const Icon = is_success ? CheckCircle2 : AlertCircle;
  const auto_dismiss_ms = is_success ? 2200 : 3600;

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
      className={cn(
        "pointer-events-auto flex min-w-[280px] max-w-[420px] items-start gap-3 rounded-[18px] border px-4 py-3 shadow-[0_18px_40px_rgba(90,110,140,0.18)] backdrop-blur-xl",
        is_success
          ? "border-emerald-200/70 bg-white/88"
          : "border-rose-200/70 bg-white/92",
      )}
    >
      <div
        className={cn(
          "mt-0.5 flex h-6 w-6 shrink-0 items-center justify-center rounded-full",
          is_success ? "bg-emerald-100 text-emerald-600" : "bg-rose-100 text-rose-600",
        )}
      >
        <Icon className="h-3.5 w-3.5" />
      </div>
      <div className="min-w-0 flex-1">
        <p className={cn("text-[12px] font-bold", is_success ? "text-emerald-700" : "text-rose-700")}>
          {title}
        </p>
        {items.length > 1 ? (
          <div className="mt-1.5 flex flex-wrap gap-1.5">
            {items.map((item) => (
              <span
                key={item}
                className={cn(
                  "inline-flex rounded-full px-2 py-0.5 text-[10px] font-medium",
                  is_success ? "bg-white/70 text-emerald-700" : "bg-white/70 text-rose-700",
                )}
              >
                {item}
              </span>
            ))}
          </div>
        ) : (
          <p className={cn("mt-0.5 text-[11px]", is_success ? "text-emerald-600/80" : "text-rose-600/80")}>
            {message}
          </p>
        )}
      </div>
      {on_dismiss && (
        <button
          className="shrink-0 text-[11px] text-slate-400 transition-colors hover:text-slate-700"
          onClick={on_dismiss}
          type="button"
        >
          ✕
        </button>
      )}
    </div>
  );
}
