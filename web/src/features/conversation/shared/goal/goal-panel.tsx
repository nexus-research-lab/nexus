"use client";

import type { ReactNode } from "react";

import { ConfirmDialog } from "@/shared/ui/dialog/confirm-dialog";
import type { Goal } from "@/types/conversation/goal";

import type { GoalContinuationHold } from "./goal-continuation-hold";
import { GoalDraftForm } from "./goal-draft-form";
import { GoalStatusStrip } from "./goal-status-strip";
import { useGoalController } from "./use-goal-controller";

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
      <ConfirmDialog
        cancelText="取消"
        confirmText="清除"
        isOpen={dialog.kind === "clear"}
        message={`Goal：${goal.objective}`}
        title="清除当前 Goal?"
        variant="danger"
        onCancel={actions.cancelDialog}
        onConfirm={actions.confirmDialog}
      />
      <ConfirmDialog
        cancelText="暂不继续"
        confirmText="继续"
        isOpen={dialog.kind === "resume"}
        message={`Goal：${dialog.kind === "resume" ? dialog.goal.objective : ""}`}
        title="继续当前 Goal?"
        onCancel={actions.cancelDialog}
        onConfirm={actions.confirmDialog}
      />
    </>
  );
}
