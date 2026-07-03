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
import { getDialogActionClassName } from "@/shared/ui/dialog/dialog-styles";
import { UiField, UiInput, UiTextarea } from "@/shared/ui/form-control";

interface GoalDraftFormProps {
  budget: string;
  disabled: boolean;
  error: string | null;
  isLoading: boolean;
  loadingLabel?: string | null;
  objective: string;
  onBudgetChange: (value: string) => void;
  onCancel: () => void;
  onObjectiveChange: (value: string) => void;
  onSubmit: (event: FormEvent) => void;
}

export function GoalDraftForm({
  budget,
  disabled,
  error,
  isLoading: isLoading,
  loadingLabel: loadingLabel = null,
  objective,
  onBudgetChange: onBudgetChange,
  onCancel: onCancel,
  onObjectiveChange: onObjectiveChange,
  onSubmit: onSubmit,
}: GoalDraftFormProps) {
  const objectiveRef = useRef<HTMLTextAreaElement | null>(null);
  const canClose = !disabled && !isLoading;
  const submitLabel = isLoading
    ? (loadingLabel ?? "保存中")
    : "保存";

  return (
    <UiDialogPortal>
      <UiDialogBackdrop
        className="z-[9998]"
        initialFocusRef={objectiveRef}
        labelledBy="goal-edit-dialog-title"
        onClose={canClose ? onCancel : undefined}
      >
        <UiDialogFormShell
          className="pointer-events-auto"
          size="md"
          onSubmit={onSubmit}
        >
          <UiDialogHeader
            icon={<Target className="h-4 w-4" />}
            iconClassName="text-(--primary)"
            title="编辑 Goal"
            titleId="goal-edit-dialog-title"
            onClose={canClose ? onCancel : undefined}
          />

          <UiDialogBody className="flex flex-col gap-4">
            <UiField
              error={error}
              htmlFor="goal-objective-input"
              label="目标"
            >
              <UiTextarea
                ref={objectiveRef}
                className="min-h-[128px]"
                data-autofocus="true"
                disabled={disabled || isLoading}
                id="goal-objective-input"
                placeholder="输入长期目标"
                value={objective}
                variant="dialog"
                onChange={(event) => onObjectiveChange(event.target.value)}
              />
            </UiField>

            <UiField
              htmlFor="goal-budget-input"
              label="Token 预算"
            >
              <UiInput
                className="max-w-[180px]"
                disabled={disabled || isLoading}
                id="goal-budget-input"
                inputMode="numeric"
                placeholder="不限制"
                value={budget}
                variant="dialog"
                onChange={(event) => onBudgetChange(event.target.value)}
              />
            </UiField>
          </UiDialogBody>

          <UiDialogFooter className="justify-end gap-3">
            <button
              className={getDialogActionClassName("default")}
              disabled={disabled || isLoading}
              type="button"
              onClick={onCancel}
            >
              取消
            </button>
            <button
              className={getDialogActionClassName(objective.trim() ? "primary" : "default")}
              disabled={disabled || isLoading || !objective.trim()}
              type="submit"
            >
              {isLoading ? (
                <span className="inline-flex items-center gap-2">
                  <Loader2 className="h-4 w-4 animate-spin" />
                  {submitLabel}
                </span>
              ) : (
                submitLabel
              )}
            </button>
          </UiDialogFooter>
        </UiDialogFormShell>
      </UiDialogBackdrop>
    </UiDialogPortal>
  );
}
