"use client";

import { FormEvent, ReactNode } from "react";
import { Save, Target, X } from "lucide-react";

import { cn } from "@/lib/utils";

interface GoalDraftFormProps {
  budget: string;
  compact: boolean;
  disabled: boolean;
  error: string | null;
  can_cancel?: boolean;
  is_editing: boolean;
  is_loading: boolean;
  objective: string;
  scope_label: string;
  on_budget_change: (value: string) => void;
  on_cancel: () => void;
  on_objective_change: (value: string) => void;
  on_submit: (event: FormEvent) => void;
}

interface GoalDraftButtonProps {
  children: ReactNode;
  disabled?: boolean;
  title: string;
  on_click: () => void;
}

function GoalDraftButton({
  children,
  disabled = false,
  title,
  on_click,
}: GoalDraftButtonProps) {
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

export function GoalDraftForm({
  budget,
  compact,
  disabled,
  error,
  can_cancel = false,
  is_editing,
  is_loading,
  objective,
  scope_label,
  on_budget_change,
  on_cancel,
  on_objective_change,
  on_submit,
}: GoalDraftFormProps) {
  return (
    <form
      className={cn(
        "mx-auto mb-2 flex w-full max-w-[980px] flex-wrap items-end gap-2 rounded-lg border border-border/70 bg-background/95 px-3 py-2 shadow-sm backdrop-blur",
        compact && "mx-2 max-w-none",
      )}
      onSubmit={on_submit}
    >
      <div className="mb-1.5 grid h-8 w-8 shrink-0 place-items-center rounded-md border border-border/70 bg-muted/50 text-muted-foreground">
        <Target className="h-4 w-4" />
      </div>
      <div className="min-w-[180px] flex-1">
        <div className="mb-0.5 flex min-w-0 items-center gap-1.5">
          <span className="text-[11px] font-medium text-muted-foreground">
            {scope_label}
          </span>
          <span className="rounded border border-border/70 bg-muted/40 px-1.5 py-0.5 text-[11px] font-medium text-muted-foreground">
            {is_editing ? "编辑中" : "新建"}
          </span>
        </div>
        <input
          className="h-8 w-full bg-transparent text-sm outline-none placeholder:text-muted-foreground"
          disabled={disabled || is_loading}
          placeholder={is_editing ? "更新 Goal" : "设定长期目标"}
          value={objective}
          onChange={(event) => on_objective_change(event.target.value)}
        />
        {error ? <div className="text-[11px] text-destructive">{error}</div> : null}
      </div>
      <input
        className="h-8 w-24 rounded-md border border-border/70 bg-background px-2 text-xs outline-none placeholder:text-muted-foreground"
        disabled={disabled || is_loading}
        inputMode="numeric"
        placeholder="Token"
        title="Token 预算"
        value={budget}
        onChange={(event) => on_budget_change(event.target.value)}
      />
      <button
        aria-label={is_editing ? "保存 Goal" : "创建 Goal"}
        className="grid h-8 w-8 place-items-center rounded-md bg-primary text-primary-foreground transition-colors hover:bg-primary/90 disabled:cursor-not-allowed disabled:opacity-50"
        disabled={disabled || is_loading || !objective.trim()}
        title={is_editing ? "保存 Goal" : "创建 Goal"}
        type="submit"
      >
        {is_editing ? <Save className="h-4 w-4" /> : <Target className="h-4 w-4" />}
      </button>
      {is_editing || can_cancel ? (
        <GoalDraftButton
          disabled={disabled || is_loading}
          title="取消"
          on_click={on_cancel}
        >
          <X className="h-4 w-4" />
        </GoalDraftButton>
      ) : null}
    </form>
  );
}
