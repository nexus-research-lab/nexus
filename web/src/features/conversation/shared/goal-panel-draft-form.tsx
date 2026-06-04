"use client";

import { FormEvent, useRef } from "react";
import { Loader2, Target } from "lucide-react";

import {
  UiDialogBackdrop,
  UiDialogBody,
  UiDialogFooter,
  UiDialogFormShell,
  UiDialogHeader,
  UiDialogPortal,
} from "@/shared/ui/dialog/dialog";
import { get_dialog_action_class_name } from "@/shared/ui/dialog/dialog-styles";
import { UiField, UiInput, UiTextarea } from "@/shared/ui/form-control";

interface GoalDraftFormProps {
  budget: string;
  disabled: boolean;
  error: string | null;
  is_loading: boolean;
  loading_label?: string | null;
  objective: string;
  on_budget_change: (value: string) => void;
  on_cancel: () => void;
  on_objective_change: (value: string) => void;
  on_submit: (event: FormEvent) => void;
}

export function GoalDraftForm({
  budget,
  disabled,
  error,
  is_loading,
  loading_label = null,
  objective,
  on_budget_change,
  on_cancel,
  on_objective_change,
  on_submit,
}: GoalDraftFormProps) {
  const objective_ref = useRef<HTMLTextAreaElement | null>(null);
  const can_close = !disabled && !is_loading;
  const submit_label = is_loading
    ? (loading_label ?? "保存中")
    : "保存";

  return (
    <UiDialogPortal>
      <UiDialogBackdrop
        class_name="z-[9998]"
        initial_focus_ref={objective_ref}
        labelled_by="goal-edit-dialog-title"
        on_close={can_close ? on_cancel : undefined}
      >
        <UiDialogFormShell
          class_name="pointer-events-auto"
          size="md"
          onSubmit={on_submit}
        >
          <UiDialogHeader
            icon={<Target className="h-4 w-4" />}
            icon_class_name="text-(--primary)"
            title="编辑 Goal"
            title_id="goal-edit-dialog-title"
            on_close={can_close ? on_cancel : undefined}
          />

          <UiDialogBody class_name="flex flex-col gap-4">
            <UiField
              error={error}
              html_for="goal-objective-input"
              label="目标"
            >
              <UiTextarea
                ref={objective_ref}
                class_name="min-h-[128px]"
                data-autofocus="true"
                disabled={disabled || is_loading}
                id="goal-objective-input"
                placeholder="输入长期目标"
                value={objective}
                variant="dialog"
                onChange={(event) => on_objective_change(event.target.value)}
              />
            </UiField>

            <UiField
              html_for="goal-budget-input"
              label="Token 预算"
            >
              <UiInput
                class_name="max-w-[180px]"
                disabled={disabled || is_loading}
                id="goal-budget-input"
                inputMode="numeric"
                placeholder="不限制"
                value={budget}
                variant="dialog"
                onChange={(event) => on_budget_change(event.target.value)}
              />
            </UiField>
          </UiDialogBody>

          <UiDialogFooter class_name="justify-end gap-3">
            <button
              className={get_dialog_action_class_name("default")}
              disabled={disabled || is_loading}
              type="button"
              onClick={on_cancel}
            >
              取消
            </button>
            <button
              className={get_dialog_action_class_name(objective.trim() ? "primary" : "default")}
              disabled={disabled || is_loading || !objective.trim()}
              type="submit"
            >
              {is_loading ? (
                <span className="inline-flex items-center gap-2">
                  <Loader2 className="h-4 w-4 animate-spin" />
                  {submit_label}
                </span>
              ) : (
                submit_label
              )}
            </button>
          </UiDialogFooter>
        </UiDialogFormShell>
      </UiDialogBackdrop>
    </UiDialogPortal>
  );
}
