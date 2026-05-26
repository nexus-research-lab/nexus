"use client";

import type { ReactNode } from "react";
import { Plus, RefreshCw, Target } from "lucide-react";

import { cn } from "@/lib/utils";

interface GoalEmptyStripProps {
  compact: boolean;
  disabled: boolean;
  error: string | null;
  is_loading: boolean;
  scope_label: string;
  on_create: () => void;
  on_refresh: () => void;
}

interface GoalEmptyButtonProps {
  children: ReactNode;
  disabled?: boolean;
  title: string;
  on_click: () => void;
}

function GoalEmptyButton({
  children,
  disabled = false,
  title,
  on_click,
}: GoalEmptyButtonProps) {
  return (
    <button
      aria-label={title}
      className="grid h-8 w-8 place-items-center rounded-md border border-border/70 text-muted-foreground transition-colors hover:bg-muted/70 disabled:cursor-not-allowed disabled:opacity-50"
      disabled={disabled}
      title={title}
      type="button"
      onClick={on_click}
    >
      {children}
    </button>
  );
}

export function GoalEmptyStrip({
  compact,
  disabled,
  error,
  is_loading,
  scope_label,
  on_create,
  on_refresh,
}: GoalEmptyStripProps) {
  return (
    <div
      className={cn(
        "mx-auto mb-2 flex w-full max-w-[980px] items-center gap-2 rounded-lg border border-border/70 bg-background/95 px-3 py-2 shadow-sm backdrop-blur",
        compact && "mx-2 max-w-none",
      )}
    >
      <div className="grid h-8 w-8 shrink-0 place-items-center rounded-md border border-border/70 bg-muted/50 text-muted-foreground">
        <Target className="h-4 w-4" />
      </div>
      <div className="min-w-0 flex-1">
        <div className="flex min-w-0 items-center gap-1.5">
          <span className="truncate text-[11px] font-medium text-muted-foreground">
            {scope_label}
          </span>
          <span className="shrink-0 rounded border border-border/70 bg-muted/40 px-1.5 py-0.5 text-[11px] font-medium text-muted-foreground">
            未设置
          </span>
        </div>
        {error ? (
          <div className="truncate text-[11px] text-destructive">{error}</div>
        ) : null}
      </div>
      <GoalEmptyButton
        disabled={is_loading}
        title="刷新 Goal"
        on_click={on_refresh}
      >
        <RefreshCw className={cn("h-4 w-4", is_loading && "animate-spin")} />
      </GoalEmptyButton>
      <GoalEmptyButton
        disabled={disabled || is_loading}
        title="设置 Goal"
        on_click={on_create}
      >
        <Plus className="h-4 w-4" />
      </GoalEmptyButton>
    </div>
  );
}
