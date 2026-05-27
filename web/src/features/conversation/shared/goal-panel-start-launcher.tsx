"use client";

import { Plus } from "lucide-react";

import { cn } from "@/lib/utils";
import { UiButton } from "@/shared/ui/button";

interface GoalStartLauncherProps {
  compact: boolean;
  disabled: boolean;
  is_loading: boolean;
  scope_label: string;
  on_create: () => void;
}

function goal_start_label(scope_label: string) {
  return scope_label.includes("房间") ? "启动房间 Goal" : "启动 Goal";
}

export function GoalStartLauncher({
  compact,
  disabled,
  is_loading,
  scope_label,
  on_create,
}: GoalStartLauncherProps) {
  return (
    <div
      className={cn(
        "mx-auto mb-2 flex w-full max-w-[980px] justify-end px-1",
        compact && "mx-2 max-w-none px-0",
      )}
    >
      <UiButton
        aria-label={`启动${scope_label}`}
        class_name="h-8 shrink-0 rounded-[10px] text-[12px] font-semibold"
        disabled={disabled || is_loading}
        size="sm"
        title={`启动${scope_label}`}
        variant="surface"
        onClick={on_create}
      >
        <Plus className={cn("h-3.5 w-3.5", is_loading && "animate-pulse")} />
        <span>{goal_start_label(scope_label)}</span>
      </UiButton>
    </div>
  );
}
