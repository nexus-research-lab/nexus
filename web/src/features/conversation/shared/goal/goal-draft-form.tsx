/**
 * INPUT: Goal 草稿、预算、可靠性事实、修改门禁与提交/只读核对命令。
 * OUTPUT: 具名且本地化的 plain 编辑表单；完整预算校验、未知结果禁用与恢复反馈。
 * POS: Conversation Goal 编辑边界；不解释 mutation 结果或自动重发修改。
 */
"use client";

import { type FormEvent, useId, useRef } from "react";
import { Loader2 } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton } from "@/shared/ui/button/button";
import {
  UiDialogBackdrop,
  UiDialogBody,
  UiDialogFooter,
  UiDialogFormShell,
  UiDialogHeader,
  UiDialogPortal,
} from "@/shared/ui/dialog/dialog";
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
  const { t } = useI18n();
  const fieldId = useId();
  const objectiveRef = useRef<HTMLTextAreaElement | null>(null);
  const model = buildGoalDraftFormModel({
    budget,
    t,
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
        onClose={model.canClose ? onCancel : undefined}
      >
        <UiDialogFormShell
          className="pointer-events-auto"
          size="md"
          onSubmit={(event) => {
            if (model.submitDisabled) {
              event.preventDefault();
              return;
            }
            onSubmit(event);
          }}
        >
          <UiDialogHeader
            appearance="plain"
            title={t("goal.edit_title")}
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
              htmlFor={`${fieldId}-objective`}
              label={t("goal.objective_label")}
            >
              <UiTextarea
                ref={objectiveRef}
                className="min-h-[128px]"
                data-autofocus="true"
                disabled={model.fieldsDisabled}
                id={`${fieldId}-objective`}
                placeholder={t("goal.objective_placeholder")}
                value={objective}
                variant="dialog"
                onChange={(event) => onObjectiveChange(event.target.value)}
              />
            </UiField>

            <UiField
              htmlFor={`${fieldId}-budget`}
              label={t("goal.budget_label")}
              description={t("goal.budget_hint")}
              error={model.budgetInvalid ? t("goal.budget_invalid") : undefined}
            >
              <UiInput
                className="max-w-[180px]"
                disabled={model.fieldsDisabled}
                id={`${fieldId}-budget`}
                inputMode="numeric"
                placeholder={t("goal.budget_placeholder")}
                value={budget}
                variant="dialog"
                onChange={(event) => onBudgetChange(event.target.value)}
              />
            </UiField>
          </UiDialogBody>

          <UiDialogFooter appearance="plain" className="justify-end gap-3">
            <UiButton
              disabled={!model.canClose}
              onClick={onCancel}
              size="md"
              variant="surface"
            >
              {t("common.cancel")}
            </UiButton>
            <UiButton
              aria-busy={model.isLoading || undefined}
              disabled={model.submitDisabled}
              size="md"
              tone={model.submitTone}
              type="submit"
              variant={model.submitTone === "default" ? "surface" : "solid"}
            >
              {model.isLoading ? (
                <span className="inline-flex items-center gap-2">
                  <Loader2 aria-hidden="true" className={getUiSpinnerClassName({ size: "md" })} />
                  {model.submitLabel}
                </span>
              ) : (
                model.submitLabel
              )}
            </UiButton>
          </UiDialogFooter>
        </UiDialogFormShell>
      </UiDialogBackdrop>
    </UiDialogPortal>
  );
}
