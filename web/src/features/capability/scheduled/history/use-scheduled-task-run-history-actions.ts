/**
 * INPUT: 当前 owner scope、任务、运行记录动作与历史刷新命令。
 * OUTPUT: 绑定 owner+Job 进入代次的动作状态、确认目标、当前语言反馈与诊断复制。
 * POS: Scheduled 历史动作控制器；旧进入代次不写状态/刷新，新操作独占反馈，已发命令不取消或重放。
 */
"use client";

import { useCallback, useLayoutEffect, useMemo, useRef } from "react";

import { writeTextToClipboard } from "@/shared/lib/browser/clipboard";
import { useResettableState } from "@/shared/lib/react/use-resettable-state";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { ScheduledTaskRunItem } from "@/types/capability/scheduled-task/run";
import type { ScheduledTaskItem } from "@/types/capability/scheduled-task/task";

import {
  createPendingCommandState,
  type PendingCommandState,
  setPendingCommand,
} from "../controller/pending-command-model";
import { projectMutationFailure } from "@/lib/error-message";
import { projectRunHistoryFeedback, type RunHistoryAction, type RunHistoryFeedback } from "./scheduled-task-run-feedback-model";
import { buildRunDiagnostic } from "./scheduled-task-run-diagnostic-model";

const RUN_HISTORY_ACTIONS = ["recover", "retry", "retryDelivery"] as const;
type RunHistoryPendingActions = PendingCommandState<RunHistoryAction>;

interface RunHistoryActionState {
  copiedRunId: string | null;
  deliveryVerificationTarget: ScheduledTaskRunItem | null;
  feedback: RunHistoryFeedback | null;
  pending: RunHistoryPendingActions;
  recoveryTarget: ScheduledTaskRunItem | null;
}

export type ScheduledTaskRunHistoryActionResult =
  | { status: "completed" }
  | { message: string; status: "blocked" };

interface RunHistoryActionCommands {
  onRecoverTaskRun: (
    task: ScheduledTaskItem,
    run: ScheduledTaskRunItem,
  ) => ScheduledTaskRunHistoryActionResult | Promise<ScheduledTaskRunHistoryActionResult>;
  onRetryDelivery: (
    task: ScheduledTaskItem,
    run: ScheduledTaskRunItem,
    reconcileHistory: () => Promise<void>,
    options?: { confirmUnverifiedAttempt?: boolean },
  ) => ScheduledTaskRunHistoryActionResult | Promise<ScheduledTaskRunHistoryActionResult>;
  onRetryTask: (
    task: ScheduledTaskItem,
    reconcileHistory: () => Promise<void>,
  ) => ScheduledTaskRunHistoryActionResult | Promise<ScheduledTaskRunHistoryActionResult>;
}

interface RunHistoryActionsOptions extends RunHistoryActionCommands {
  reconcileHistory: () => Promise<void>;
  refresh: () => Promise<unknown>;
  scopeKey: string | null;
  task: ScheduledTaskItem | null;
}

function createInitialActionState(): RunHistoryActionState {
  return {
    copiedRunId: null,
    deliveryVerificationTarget: null,
    feedback: null,
    pending: createPendingCommandState(RUN_HISTORY_ACTIONS),
    recoveryTarget: null,
  };
}

function runHistoryTaskKey(scopeKey: string | null, jobId: string | null): string | null {
  return scopeKey && jobId ? `${scopeKey}\u0000${jobId}` : null;
}

export function useScheduledTaskRunHistoryActions({
  onRecoverTaskRun,
  onRetryDelivery,
  onRetryTask,
  reconcileHistory,
  refresh,
  scopeKey,
  task,
}: RunHistoryActionsOptions) {
  const { locale, t } = useI18n();
  const taskJobId = task?.job_id ?? null;
  const taskKey = runHistoryTaskKey(scopeKey, taskJobId);
  const scopeGeneration = useMemo(() => Symbol(taskKey ?? "closed"), [taskKey]);
  const taskDeletionState = task?.deletion_state?.trim() ?? "";
  const [state, setState] = useResettableState(
    createInitialActionState(),
    scopeGeneration,
  );
  const activeScopeRef = useRef<symbol | null>(null);
  const feedbackRequestRef = useRef(0);
  const pendingPromisesRef = useRef(new Map<string, Promise<void>>());

  useLayoutEffect(() => {
    activeScopeRef.current = scopeGeneration;
    feedbackRequestRef.current += 1;
    pendingPromisesRef.current.clear();
    return () => {
      if (activeScopeRef.current === scopeGeneration) {
        activeScopeRef.current = null;
      }
    };
  }, [scopeGeneration]);

  useLayoutEffect(() => {
    if (!taskJobId || !taskDeletionState) {
      return;
    }
    setState((current) => ({
      ...current,
      deliveryVerificationTarget: null,
      recoveryTarget: null,
    }));
  }, [setState, taskDeletionState, taskJobId]);

  const updateActiveState = useCallback((
    jobId: string,
    update: (current: RunHistoryActionState) => RunHistoryActionState,
  ): void => {
    if (jobId === taskJobId && activeScopeRef.current === scopeGeneration) {
      setState(update);
    }
  }, [scopeGeneration, setState, taskJobId]);

  const updateFeedback = useCallback((request: number, feedback: RunHistoryFeedback | null): void => {
    if (taskJobId && feedbackRequestRef.current === request) {
      updateActiveState(taskJobId, (current) => ({ ...current, feedback }));
    }
  }, [taskJobId, updateActiveState]);

  const isCurrentRun = useCallback((run: ScheduledTaskRunItem): boolean => (
    Boolean(taskKey && taskJobId === run.job_id && activeScopeRef.current === scopeGeneration)
  ), [scopeGeneration, taskJobId, taskKey]);

  const runAction = useCallback((
    action: RunHistoryAction,
    run: ScheduledTaskRunItem,
    execute: (
      activeTask: ScheduledTaskItem,
    ) => ScheduledTaskRunHistoryActionResult | Promise<ScheduledTaskRunHistoryActionResult>,
  ): Promise<void> => {
    if (!task || !isCurrentRun(run)) return Promise.resolve();
    const commandKey = `${action}:${run.run_id}`;
    const pendingPromise = pendingPromisesRef.current.get(commandKey);
    if (pendingPromise) return pendingPromise;
    const activeTask = task;
    const feedbackRequest = ++feedbackRequestRef.current;
    if (activeTask.deletion_state?.trim()) {
      updateFeedback(feedbackRequest, blockedActionFeedback(activeTask));
      return Promise.resolve();
    }
    updateActiveState(activeTask.job_id, (current) => ({
      ...current,
      feedback: null,
      pending: setPendingCommand(current.pending, action, run.run_id, true),
    }));
    // 注册 Promise 后再执行，同步抛错也能按同一身份清理，不遗留已结束的防重项。
    const nextPromise = Promise.resolve().then(async () => {
      try {
        const result = await execute(activeTask);
        if (activeScopeRef.current !== scopeGeneration) return;
        if (result.status === "blocked") {
          updateFeedback(feedbackRequest, blockedActionFeedback(activeTask));
          return;
        }
        updateFeedback(feedbackRequest, { action, kind: "completed" });
        try {
          await refresh();
        } catch {
          updateFeedback(feedbackRequest, { kind: "refresh_failed" });
        }
      } catch (error) {
        updateFeedback(feedbackRequest, {
          action,
          effect: projectMutationFailure(error, "").effect,
          kind: "failed",
        });
      } finally {
        if (pendingPromisesRef.current.get(commandKey) === nextPromise) {
          pendingPromisesRef.current.delete(commandKey);
        }
        updateActiveState(activeTask.job_id, (current) => ({
          ...current,
          pending: setPendingCommand(current.pending, action, run.run_id, false),
        }));
      }
    });
    pendingPromisesRef.current.set(commandKey, nextPromise);
    return nextPromise;
  }, [isCurrentRun, refresh, scopeGeneration, task, updateActiveState, updateFeedback]);

  const copyDiagnostic = useCallback(async (run: ScheduledTaskRunItem): Promise<void> => {
    if (!task || !isCurrentRun(run)) return;
    const feedbackRequest = ++feedbackRequestRef.current;
    updateFeedback(feedbackRequest, null);
    const copied = await writeTextToClipboard(buildRunDiagnostic(task, run, { locale, t }));
    if (feedbackRequestRef.current !== feedbackRequest) return;
    updateActiveState(task.job_id, (current) => ({
      ...current,
      copiedRunId: copied ? run.run_id : current.copiedRunId,
      feedback: { copied, kind: "clipboard" },
    }));
  }, [isCurrentRun, locale, t, task, updateActiveState, updateFeedback]);

  const retry = useCallback((run: ScheduledTaskRunItem): Promise<void> => (
    runAction("retry", run, (activeTask) => (
      onRetryTask(activeTask, reconcileHistory)
    ))
  ), [onRetryTask, reconcileHistory, runAction]);

  const executeRetryDelivery = useCallback((
    run: ScheduledTaskRunItem,
    confirmUnverifiedAttempt: boolean,
  ): Promise<void> => (
    runAction("retryDelivery", run, (activeTask) => (
      onRetryDelivery(activeTask, run, reconcileHistory, {
        confirmUnverifiedAttempt,
      })
    ))
  ), [onRetryDelivery, reconcileHistory, runAction]);

  const retryDelivery = useCallback((run: ScheduledTaskRunItem): Promise<void> => {
    if (!isCurrentRun(run)) return Promise.resolve();
    if (task?.deletion_state?.trim()) {
      feedbackRequestRef.current += 1;
      updateActiveState(task.job_id, (current) => ({
        ...current,
        feedback: blockedActionFeedback(task),
      }));
      return Promise.resolve();
    }
    if (run.delivery_status !== "retrying") {
      return executeRetryDelivery(run, false);
    }
    if (!task || typeof run.delivery_attempts !== "number") {
      return Promise.resolve();
    }
    updateActiveState(task.job_id, (current) => ({
      ...current,
      deliveryVerificationTarget: run,
    }));
    return Promise.resolve();
  }, [executeRetryDelivery, isCurrentRun, task, updateActiveState]);

  const cancelDeliveryVerification = useCallback(() => {
    if (!task) return;
    updateActiveState(task.job_id, (current) => ({
      ...current,
      deliveryVerificationTarget: null,
    }));
  }, [task, updateActiveState]);

  const confirmDeliveryVerification = useCallback((): Promise<void> => {
    if (!task || !state.deliveryVerificationTarget || !isCurrentRun(state.deliveryVerificationTarget)) return Promise.resolve();
    if (task.deletion_state?.trim()) {
      feedbackRequestRef.current += 1;
      updateActiveState(task.job_id, (current) => ({
        ...current,
        deliveryVerificationTarget: null,
        feedback: blockedActionFeedback(task),
      }));
      return Promise.resolve();
    }
    const run = state.deliveryVerificationTarget;
    updateActiveState(task.job_id, (current) => ({
      ...current,
      deliveryVerificationTarget: null,
    }));
    return executeRetryDelivery(run, true);
  }, [executeRetryDelivery, isCurrentRun, state.deliveryVerificationTarget, task, updateActiveState]);

  const recover = useCallback((run: ScheduledTaskRunItem): Promise<void> => {
    if (!task || !isCurrentRun(run)) return Promise.resolve();
    if (task.deletion_state?.trim()) {
      feedbackRequestRef.current += 1;
      updateActiveState(task.job_id, (current) => ({
        ...current,
        feedback: blockedActionFeedback(task),
      }));
      return Promise.resolve();
    }
    updateActiveState(task.job_id, (current) => ({
      ...current,
      recoveryTarget: run,
    }));
    return Promise.resolve();
  }, [isCurrentRun, task, updateActiveState]);

  const cancelRecovery = useCallback(() => {
    if (!task) return;
    updateActiveState(task.job_id, (current) => ({
      ...current,
      recoveryTarget: null,
    }));
  }, [task, updateActiveState]);

  const confirmRecovery = useCallback((): Promise<void> => {
    if (!task || !state.recoveryTarget || !isCurrentRun(state.recoveryTarget)) return Promise.resolve();
    if (task.deletion_state?.trim()) {
      feedbackRequestRef.current += 1;
      updateActiveState(task.job_id, (current) => ({
        ...current,
        feedback: blockedActionFeedback(task),
        recoveryTarget: null,
      }));
      return Promise.resolve();
    }
    const run = state.recoveryTarget;
    updateActiveState(task.job_id, (current) => ({
      ...current,
      recoveryTarget: null,
    }));
    return runAction("recover", run, (activeTask) => (
      onRecoverTaskRun(activeTask, run)
    ));
  }, [isCurrentRun, onRecoverTaskRun, runAction, state.recoveryTarget, task, updateActiveState]);

  return {
    ...state,
    feedback: projectRunHistoryFeedback(state.feedback, t),
    cancelDeliveryVerification,
    cancelRecovery,
    confirmDeliveryVerification,
    confirmRecovery,
    copyDiagnostic,
    recover,
    retry,
    retryDelivery,
  };
}

function blockedActionFeedback(task: ScheduledTaskItem): RunHistoryFeedback {
  const deletionState = task.deletion_state?.trim();
  return {
    deletion: deletionState === "review_required" ? "review_required" : deletionState ? "deleting" : null,
    kind: "blocked",
  };
}
