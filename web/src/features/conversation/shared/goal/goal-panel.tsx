"use client";

import type { ReactNode } from "react";

import { ConfirmDialog } from "@/shared/ui/dialog/decision/decision-dialog";
import type { Goal } from "@/types/conversation/goal";

import type { GoalContinuationHold } from "./goal-continuation-hold";
import { GoalDraftForm } from "./goal-draft-form";
import type { GoalDialog } from "./goal-model";
import { GoalStatusStrip } from "./goal-status-strip";
import { useGoalController } from "./use-goal-controller";

interface GoalDialogPresentation {
  cancelText: string;
  confirmText: string;
  title: string;
  variant?: "danger";
}

const GOAL_DIALOG_PRESENTATION: GoalDialogPresentation = {
  cancelText: "取消",
  confirmText: "清除",
  title: "清除当前 Goal?",
  variant: "danger",
};

interface GoalPanelProps {
  activityKey?: number | string | null;
  compact?: boolean;
  continuationHold?: GoalContinuationHold | null;
  disabled?: boolean;
  isGenerating?: boolean;
  onGoalChange?: (goal: Goal | null) => void;
  scopeLabel?: string;
  sessionKey: string | null;
  statusExtra?: ReactNode;
}

function GoalConfirmationDialog({
  dialog,
  onCancel,
  onConfirm,
}: {
  dialog: GoalDialog;
  onCancel: () => void;
  onConfirm: () => void;
}) {
  if (dialog.kind === "none") {
    return null;
  }
  const presentation = GOAL_DIALOG_PRESENTATION;
  return (
    <ConfirmDialog
      cancelText={presentation.cancelText}
      confirmText={presentation.confirmText}
      isOpen
      message={`Goal：${dialog.goal.objective}`}
      title={presentation.title}
      variant={presentation.variant}
      onCancel={onCancel}
      onConfirm={onConfirm}
    />
  );
}

function GoalPanelContent({
  compact,
  continuationHold,
  controller,
  disabled,
  isGenerating,
  scopeLabel,
  sessionKey,
  statusExtra,
}: {
  compact: boolean;
  continuationHold: GoalContinuationHold | null;
  controller: ReturnType<typeof useGoalController>;
  disabled: boolean;
  isGenerating: boolean;
  scopeLabel: string;
  sessionKey: string | null;
  statusExtra: ReactNode;
}) {
  const { actions, dialog, draft, goal } = controller;
  if (!controller.isAvailable || !sessionKey || !goal) {
    return null;
  }

  return (
    <>
      <GoalStatusStrip
        canResume={controller.canResume}
        compact={compact}
        continuationHold={continuationHold}
        disabled={disabled}
        error={controller.error}
        goal={goal}
        isGenerating={isGenerating}
        isLoading={controller.isLoading}
        scopeLabel={scopeLabel}
        statusExtra={statusExtra}
        onClearRequest={actions.startClearing}
        onEdit={actions.startEditing}
        onPause={actions.pause}
        onRefresh={actions.refresh}
        onResume={actions.resume}
      />
      {draft ? (
        <GoalDraftForm
          budget={draft.budget}
          disabled={disabled}
          error={controller.error}
          isLoading={controller.isLoading}
          loadingLabel={controller.loadingLabel}
          objective={draft.objective}
          onBudgetChange={actions.setBudget}
          onCancel={actions.cancelEditing}
          onObjectiveChange={actions.setObjective}
          onSubmit={actions.submit}
        />
      ) : null}
      <GoalConfirmationDialog
        dialog={dialog}
        onCancel={actions.cancelDialog}
        onConfirm={actions.confirmDialog}
      />
    </>
  );
}

export function GoalPanel({
  activityKey = null,
  compact = false,
  continuationHold = null,
  disabled = false,
  isGenerating = false,
  onGoalChange,
  scopeLabel = "会话 Goal",
  sessionKey,
  statusExtra = null,
}: GoalPanelProps) {
  const controller = useGoalController({
    activityKey,
    disabled,
    onGoalChange,
    sessionKey,
  });

  return (
    <GoalPanelContent
      compact={compact}
      continuationHold={continuationHold}
      controller={controller}
      disabled={disabled}
      isGenerating={isGenerating}
      scopeLabel={scopeLabel}
      sessionKey={sessionKey}
      statusExtra={statusExtra}
    />
  );
}
