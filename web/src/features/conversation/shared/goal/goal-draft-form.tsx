/**
 * INPUT: Goal 草稿、预算、可靠性事实、修改门禁与提交/只读核对命令。
 * OUTPUT: 保留草稿、阻止未知结果重复提交并展示完整恢复信息的 plain 编辑表单。
 * POS: Conversation Goal 编辑边界；不解释 mutation 结果或自动重发修改。
 */
"use client";

import { type FormEvent, useRef } from "react";
import { Loader2 } from "lucide-react";

import {
  UiDialogBackdrop,
  UiDialogBody,
  UiDialogFooter,
  UiDialogFormShell,
  UiDialogHeader,
  UiDialogPortal,
} from "@/shared/ui/dialog/dialog";
import { getDialogActionClassName } from "@/shared/ui/dialog/dialog-styles";
import { getUiSpinnerClassName } from "@/shared/ui/display/spinner-styles";
import { UiField, UiInput, UiTextarea } from "@/shared/ui/form/form-control";

import type { GoalReliabilityState } from "./goal-lifecycle-recovery";
import { buildGoalDraftFormModel } from "./goal-model";
import { GoalReliabilityNotice } from "./goal-reliability-notice";

interface GoalDraftFormProps {
  budget: string;
  disabled: boolean;
  isLoading: boolean;
  loadingLabel?: string | null;
  mutationBlocked: boolean;
  objective: string;
  onBudgetChange: (value: string) => void;
  onCancel: () => void;
  onObjectiveChange: (value: string) => void;
  onRefresh: () => void;
  onSubmit: (event: FormEvent) => void;
  reliability: GoalReliabilityState | null;
}

export function GoalDraftForm({
  budget,
  disabled,
  isLoading,
  loadingLabel = null,
  mutationBlocked,
  objective,
  onBudgetChange,
  onCancel,
  onObjectiveChange,
  onRefresh,
  onSubmit,
  reliability,
}: GoalDraftFormProps) {
  const objectiveRef = useRef<HTMLTextAreaElement | null>(null);
  const model = buildGoalDraftFormModel({
    disabled,
    isLoading,
    loadingLabel,
    mutationBlocked,
    objective,
  });

  return (
    <UiDialogPortal>
      <UiDialogBackdrop
        layer="dialogUnderlay"
        initialFocusRef={objectiveRef}
        labelledBy="goal-edit-dialog-title"
        onClose={model.canClose ? onCancel : undefined}
      >
        <UiDialogFormShell
          className="pointer-events-auto"
          size="md"
          onSubmit={onSubmit}
        >
          <UiDialogHeader
            appearance="plain"
            title="编辑 Goal"
            titleId="goal-edit-dialog-title"
            onClose={model.canClose ? onCancel : undefined}
          />

          <UiDialogBody className="flex flex-col gap-4">
            {reliability ? (
              <GoalReliabilityNotice
                isRefreshing={isLoading}
                mutationBlocked={mutationBlocked}
                state={reliability}
                onRefresh={onRefresh}
              />
            ) : null}
            <UiField
              htmlFor="goal-objective-input"
              label="目标"
            >
              <UiTextarea
                ref={objectiveRef}
                className="min-h-[128px]"
                data-autofocus="true"
                disabled={model.fieldsDisabled}
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
                disabled={model.fieldsDisabled}
                id="goal-budget-input"
                inputMode="numeric"
                placeholder="不限制"
                value={budget}
                variant="dialog"
                onChange={(event) => onBudgetChange(event.target.value)}
              />
            </UiField>
          </UiDialogBody>

          <UiDialogFooter appearance="plain" className="justify-end gap-3">
            <button
              className={getDialogActionClassName("default")}
              disabled={!model.canClose}
              type="button"
              onClick={onCancel}
            >
              取消
            </button>
            <button
              className={getDialogActionClassName(model.submitTone)}
              disabled={model.submitDisabled}
              type="submit"
            >
              {model.isLoading ? (
                <span className="inline-flex items-center gap-2">
                  <Loader2 className={getUiSpinnerClassName({ size: "md" })} />
                  {model.submitLabel}
                </span>
              ) : (
                model.submitLabel
              )}
            </button>
          </UiDialogFooter>
        </UiDialogFormShell>
      </UiDialogBackdrop>
    </UiDialogPortal>
  );
}
