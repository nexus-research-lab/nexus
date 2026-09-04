"use client";

/**
 * INPUT: session-scoped Goal controller/reliability state and panel presentation props.
 * OUTPUT: status, Problem/Impact/Recovery notices, edit and clear-confirmation UI.
 * POS: Goal panel composition layer; it does not infer Execution binding or call APIs.
 */

import type { ReactNode } from "react";

import { ConfirmDialog } from "@/shared/ui/dialog/decision/decision-dialog";
import type { Goal } from "@/types/conversation/goal";

import type { GoalContinuationHold } from "./goal-continuation-hold";
import { GoalDraftForm } from "./goal-draft-form";
import type { GoalReliabilityState } from "./goal-lifecycle-recovery";
import {
  GOAL_PANEL_COMPACT_CLASS_NAME,
  GOAL_PANEL_STRIP_CLASS_NAME,
} from "./goal-panel-layout";
import type { GoalDialog } from "./goal-model";
import { GoalReliabilityNotice } from "./goal-reliability-notice";
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
  if (!sessionKey) {
    return null;
  }
  const resourceReliability = controller.reliability;
  const runtimeReliability = runtimeReliabilityState(
    goal,
    sessionKey,
  );
  const draftReliability = resourceReliability ?? runtimeReliability;
  if (!goal) {
    return resourceReliability ? (
      <GoalReliabilityLane
        compact={compact}
        isRefreshing={controller.isLoading}
        mutationBlocked={controller.mutationsBlocked}
        reliability={resourceReliability}
        onRefresh={actions.refresh}
      />
    ) : null;
  }

  return (
    <>
      <GoalStatusStrip
        canResume={controller.canResume}
        clearDisabledReason={controller.clearDisabledReason}
        compact={compact}
        continuationHold={continuationHold}
        disabled={disabled}
        executionBinding={controller.executionBinding}
        goal={goal}
        isGenerating={isGenerating}
        isLoading={controller.isLoading}
        mutationBlockReason={controller.mutationBlockReason}
        mutationBlocked={controller.mutationsBlocked}
        scopeLabel={scopeLabel}
        statusExtra={statusExtra}
        onClearRequest={actions.startClearing}
        onEdit={actions.startEditing}
        onPause={actions.pause}
        onRefresh={actions.refresh}
        onResume={actions.resume}
      />
      {resourceReliability ? (
        <GoalReliabilityLane
          compact={compact}
          isRefreshing={controller.isLoading}
          mutationBlocked={controller.mutationsBlocked}
          reliability={resourceReliability}
          onRefresh={actions.refresh}
        />
      ) : null}
      {runtimeReliability ? (
        <GoalReliabilityLane
          compact={compact}
          isRefreshing={controller.isLoading}
          mutationBlocked={controller.mutationsBlocked}
          reliability={runtimeReliability}
          onRefresh={actions.refresh}
        />
      ) : null}
      {draft ? (
        <GoalDraftForm
          budget={draft.budget}
          disabled={disabled}
          isLoading={controller.isLoading}
          loadingLabel={controller.loadingLabel}
          mutationBlocked={controller.mutationsBlocked}
          objective={draft.objective}
          onBudgetChange={actions.setBudget}
          onCancel={actions.cancelEditing}
          onObjectiveChange={actions.setObjective}
          onRefresh={actions.refresh}
          onSubmit={actions.submit}
          reliability={draftReliability}
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

function GoalReliabilityLane({
  compact,
  isRefreshing,
  mutationBlocked,
  onRefresh,
  reliability,
}: {
  compact: boolean;
  isRefreshing: boolean;
  mutationBlocked: boolean;
  onRefresh: () => void;
  reliability: GoalReliabilityState;
}) {
  return (
    <div className={compact
      ? GOAL_PANEL_COMPACT_CLASS_NAME
      : GOAL_PANEL_STRIP_CLASS_NAME}
    >
      <GoalReliabilityNotice
        isRefreshing={isRefreshing}
        mutationBlocked={mutationBlocked}
        state={reliability}
        onRefresh={onRefresh}
      />
    </div>
  );
}

function runtimeReliabilityState(
  goal: Goal | null,
  sessionKey: string,
): GoalReliabilityState | null {
  if (!goal?.last_error) {
    return null;
  }
  return {
    access: null,
    detail: goal.last_error,
    kind: goal.status === "budget_limited"
      ? "runtime_budget_limited"
      : goal.status === "usage_limited"
        ? "runtime_usage_limited"
        : "runtime_failed",
    operation: null,
    sessionKey,
    stale: false,
  };
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
