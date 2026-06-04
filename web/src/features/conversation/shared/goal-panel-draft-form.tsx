"use client";

import { FormEvent } from "react";
import { Loader2, Save, Target, X } from "lucide-react";

import { cn } from "@/lib/utils";
import { UiIconButton } from "@/shared/ui/button";
import { UiInput } from "@/shared/ui/form-control";
import {
  GOAL_PANEL_BADGE_CLASS_NAME,
  GOAL_PANEL_COMPACT_CLASS_NAME,
  GOAL_PANEL_HEADER_CLASS_NAME,
  GOAL_PANEL_HEADER_LABEL_CLASS_NAME,
  GOAL_PANEL_LEADING_ICON_CLASS_NAME,
  GOAL_PANEL_ROW_CLASS_NAME,
  GOAL_PANEL_STRIP_CLASS_NAME,
  GOAL_PANEL_SURFACE_CLASS_NAME,
} from "./goal-panel-styles";

interface GoalDraftFormProps {
  budget: string;
  compact: boolean;
  disabled: boolean;
  error: string | null;
  is_loading: boolean;
  loading_label?: string | null;
  objective: string;
  scope_label: string;
  on_budget_change: (value: string) => void;
  on_cancel: () => void;
  on_objective_change: (value: string) => void;
  on_submit: (event: FormEvent) => void;
}

export function GoalDraftForm({
  budget,
  compact,
  disabled,
  error,
  is_loading,
  loading_label = null,
  objective,
  scope_label,
  on_budget_change,
  on_cancel,
  on_objective_change,
  on_submit,
}: GoalDraftFormProps) {
  const submit_label = is_loading
    ? (loading_label ?? "处理中")
    : "保存 Goal";

  return (
    <form
      className={cn(
        GOAL_PANEL_STRIP_CLASS_NAME,
        compact && GOAL_PANEL_COMPACT_CLASS_NAME,
      )}
      onSubmit={on_submit}
    >
      <div className={GOAL_PANEL_SURFACE_CLASS_NAME}>
        <div className={GOAL_PANEL_HEADER_CLASS_NAME}>
          <span className={GOAL_PANEL_HEADER_LABEL_CLASS_NAME}>
            <Target className="h-3 w-3 shrink-0 text-(--primary)" />
            {scope_label}
            <span className={GOAL_PANEL_BADGE_CLASS_NAME}>
              编辑中
            </span>
          </span>
          {is_loading && loading_label ? (
            <span
              aria-live="polite"
              className="inline-flex min-w-0 items-center gap-1.5 text-(--primary)"
            >
              <Loader2 className="h-3 w-3 shrink-0 animate-spin" />
              <span className="truncate">{loading_label}</span>
            </span>
          ) : null}
        </div>

        <div className={GOAL_PANEL_ROW_CLASS_NAME}>
          <span className={GOAL_PANEL_LEADING_ICON_CLASS_NAME}>
            <Target className="h-3.5 w-3.5" />
          </span>
          <input
            className="h-7 min-w-[180px] flex-1 appearance-none border-0 bg-transparent p-0 text-[13px] leading-5 text-(--text-strong) outline-none shadow-none ring-0 placeholder:text-(--text-soft) focus:border-0 focus:bg-transparent focus:outline-none focus:ring-0 focus-visible:outline-none focus-visible:ring-0 disabled:cursor-not-allowed disabled:opacity-(--disabled-opacity)"
            disabled={disabled || is_loading}
            placeholder="更新 Goal"
            value={objective}
            onChange={(event) => on_objective_change(event.target.value)}
          />

          <UiInput
            class_name="w-[104px] shrink-0"
            control_size="xs"
            disabled={disabled || is_loading}
            inputMode="numeric"
            placeholder="Token"
            title="Token 预算"
            value={budget}
            variant="surface"
            onChange={(event) => on_budget_change(event.target.value)}
          />

          <UiIconButton
            aria-label={submit_label}
            disabled={disabled || is_loading || !objective.trim()}
            size="md"
            title={submit_label}
            tone="primary"
            type="submit"
            variant="solid"
          >
            {is_loading ? (
              <Loader2 className="h-4 w-4 animate-spin" />
            ) : (
              <Save className="h-4 w-4" />
            )}
          </UiIconButton>
          <UiIconButton
            aria-label="取消"
            disabled={disabled || is_loading}
            size="md"
            title="取消"
            type="button"
            variant="surface"
            onClick={on_cancel}
          >
            <X className="h-4 w-4" />
          </UiIconButton>
        </div>

        {error ? (
          <div className="mt-1 pl-6 text-[11px] leading-4 text-(--destructive)">
            {error}
          </div>
        ) : null}
      </div>
    </form>
  );
}
