/**
 * INPUT: 当前 owner scope、任务、运行记录动作与历史刷新命令。
 * OUTPUT: 绑定精确 owner+Job 的动作状态、恢复/投递核对目标与反馈。
 * POS: Scheduled 运行历史动作控制器；决策由产品内确认框承载。
 */
"use client";

import { useCallback, useLayoutEffect, useRef } from "react";

import { writeTextToClipboard } from "@/hooks/ui/clipboard";
import { useResettableState } from "@/hooks/ui/use-resettable-state";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { ScheduledTaskRunItem } from "@/types/capability/scheduled-task/run";
import type { ScheduledTaskItem } from "@/types/capability/scheduled-task/task";

import {
  createPendingCommandState,
  type PendingCommandState,
  setPendingCommand,
} from "../controller/pending-command-model";
import type { ScheduledTaskFeedback } from "../controller/scheduled-task-directory-model";
import { projectScheduledTaskMutationFailure } from "../controller/scheduled-task-mutation-outcome";
import { buildRunDiagnostic } from "./scheduled-task-run-diagnostic-model";

const RUN_HISTORY_ACTIONS = ["recover", "retry", "retryDelivery"] as const;
type RunHistoryAction = typeof RUN_HISTORY_ACTIONS[number];
type RunHistoryPendingActions = PendingCommandState<RunHistoryAction>;

interface RunHistoryActionState {
  copiedRunId: string | null;
  deliveryVerificationTarget: ScheduledTaskRunItem | null;
  feedback: ScheduledTaskFeedback | null;
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

interface RunActionCopy {
  failure: string;
  refreshFailure: string;
  success: string;
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
  const { t } = useI18n();
  const taskJobId = task?.job_id ?? null;
  const taskKey = runHistoryTaskKey(scopeKey, taskJobId);
  const taskDeletionState = task?.deletion_state?.trim() ?? "";
  const [state, setState] = useResettableState(
    createInitialActionState(),
    taskKey ?? "closed",
  );
  const activeTaskKeyRef = useRef<string | null>(taskKey);
  const pendingPromisesRef = useRef(new Map<string, Promise<void>>());

  useLayoutEffect(() => {
    activeTaskKeyRef.current = taskKey;
    pendingPromisesRef.current.clear();
    return () => {
      if (activeTaskKeyRef.current === taskKey) {
        activeTaskKeyRef.current = null;
      }
    };
  }, [taskKey]);

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
    if (activeTaskKeyRef.current === runHistoryTaskKey(scopeKey, jobId)) {
      setState(update);
    }
  }, [scopeKey, setState]);

  const runAction = useCallback((
    action: RunHistoryAction,
    run: ScheduledTaskRunItem,
    execute: (
      activeTask: ScheduledTaskItem,
    ) => ScheduledTaskRunHistoryActionResult | Promise<ScheduledTaskRunHistoryActionResult>,
    copy: RunActionCopy,
  ): Promise<void> => {
    if (!task) {
      return Promise.resolve();
    }
    const commandKey = `${taskKey ?? "no-task"}:${action}:${run.run_id}`;
    const pendingPromise = pendingPromisesRef.current.get(commandKey);
    if (pendingPromise) {
      return pendingPromise;
    }
    const activeTask = task;
    if (activeTask.deletion_state?.trim()) {
      updateActiveState(activeTask.job_id, (current) => ({
        ...current,
        feedback: blockedActionFeedback(activeTask),
      }));
      return Promise.resolve();
    }
    updateActiveState(activeTask.job_id, (current) => ({
      ...current,
      feedback: null,
      pending: setPendingCommand(current.pending, action, run.run_id, true),
    }));
    const nextPromise = (async () => {
      try {
        const result = await execute(activeTask);
        if (result.status === "blocked") {
          updateActiveState(activeTask.job_id, (current) => ({
            ...current,
            feedback: blockedActionFeedback(activeTask, result.message),
          }));
          return;
        }
        updateActiveState(activeTask.job_id, (current) => ({
          ...current,
          feedback: {
            impact: "服务端已接受这次操作；页面刷新不会再次提交它。",
            message: copy.success,
            nextStep: "运行历史会自动刷新，也可以使用右上角“刷新”再次核对。",
            title: copy.success,
            tone: "success",
          },
        }));
        try {
          await refresh();
        } catch (error) {
          const projection = projectScheduledTaskMutationFailure(error, copy.refreshFailure);
          updateActiveState(activeTask.job_id, (current) => ({
            ...current,
            feedback: {
              impact: "操作已经提交；下方仍显示提交前加载的历史，请勿重复操作。",
              message: `${copy.success}；${projection.message}`,
              nextStep: "点击右上角“刷新”核对最新运行记录。",
              title: "操作已提交，历史尚未刷新",
              tone: "warning",
            },
          }));
        }
      } catch (error) {
        const projection = projectScheduledTaskMutationFailure(error, copy.failure);
        const notApplied = projection.effect === "not_applied";
        updateActiveState(activeTask.job_id, (current) => ({
          ...current,
          feedback: {
            impact: t(notApplied
              ? "capability.scheduled_mutation_not_applied_impact"
              : projection.effect === "accepted"
                ? "capability.scheduled_mutation_accepted_impact"
                : projection.effect === "committed"
                  ? "capability.scheduled_mutation_committed_impact"
                  : "capability.scheduled_mutation_unknown_impact"),
            message: projection.message,
            nextStep: t(notApplied
              ? "capability.scheduled_mutation_not_applied_next_step"
              : "capability.scheduled_mutation_unknown_next_step"),
            title: notApplied
              ? copy.failure
              : t(projection.effect === "accepted"
                ? "capability.scheduled_mutation_accepted_title"
                : projection.effect === "committed"
                  ? "capability.scheduled_mutation_committed_title"
                  : "capability.scheduled_mutation_unknown_title"),
            tone: notApplied ? "error" : "warning",
          },
        }));
      } finally {
        pendingPromisesRef.current.delete(commandKey);
        updateActiveState(activeTask.job_id, (current) => ({
          ...current,
          pending: setPendingCommand(current.pending, action, run.run_id, false),
        }));
      }
    })();
    pendingPromisesRef.current.set(commandKey, nextPromise);
    return nextPromise;
  }, [refresh, t, task, taskKey, updateActiveState]);

  const copyDiagnostic = useCallback(async (run: ScheduledTaskRunItem): Promise<void> => {
    if (!task) {
      return;
    }
    const copied = await writeTextToClipboard(buildRunDiagnostic(task, run));
    updateActiveState(task.job_id, (current) => ({
      ...current,
      copiedRunId: copied ? run.run_id : current.copiedRunId,
      feedback: copied
        ? {
            impact: "任务和运行记录没有变化。",
            message: "诊断信息已复制到剪贴板。",
            nextStep: "可以把诊断信息粘贴到需要的位置。",
            title: "诊断信息已复制",
            tone: "success",
          }
        : {
            impact: "运行记录仍然保留。",
            message: "浏览器没有允许写入剪贴板。",
            nextStep: "请使用运行产物查看完整诊断，或允许剪贴板权限后再试。",
            title: "无法复制诊断信息",
            tone: "error",
          },
    }));
  }, [task, updateActiveState]);

  const retry = useCallback((run: ScheduledTaskRunItem): Promise<void> => (
    runAction("retry", run, (activeTask) => (
      onRetryTask(activeTask, reconcileHistory)
    ), {
      failure: "重新运行失败",
      refreshFailure: "运行历史刷新失败",
      success: "已触发重新运行",
    })
  ), [onRetryTask, reconcileHistory, runAction]);

  const executeRetryDelivery = useCallback((
    run: ScheduledTaskRunItem,
    confirmUnverifiedAttempt: boolean,
  ): Promise<void> => (
    runAction("retryDelivery", run, (activeTask) => (
      onRetryDelivery(activeTask, run, reconcileHistory, {
        confirmUnverifiedAttempt,
      })
    ), {
      failure: "重试投递失败",
      refreshFailure: "运行历史刷新失败",
      success: "已重试投递",
    })
  ), [onRetryDelivery, reconcileHistory, runAction]);

  const retryDelivery = useCallback((run: ScheduledTaskRunItem): Promise<void> => {
    if (task?.deletion_state?.trim()) {
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
  }, [executeRetryDelivery, task, updateActiveState]);

  const cancelDeliveryVerification = useCallback(() => {
    if (!task) return;
    updateActiveState(task.job_id, (current) => ({
      ...current,
      deliveryVerificationTarget: null,
    }));
  }, [task, updateActiveState]);

  const confirmDeliveryVerification = useCallback((): Promise<void> => {
    if (!task || !state.deliveryVerificationTarget) return Promise.resolve();
    if (task.deletion_state?.trim()) {
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
  }, [executeRetryDelivery, state.deliveryVerificationTarget, task, updateActiveState]);

  const recover = useCallback((run: ScheduledTaskRunItem): Promise<void> => {
    if (!task) return Promise.resolve();
    if (task.deletion_state?.trim()) {
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
  }, [task, updateActiveState]);

  const cancelRecovery = useCallback(() => {
    if (!task) return;
    updateActiveState(task.job_id, (current) => ({
      ...current,
      recoveryTarget: null,
    }));
  }, [task, updateActiveState]);

  const confirmRecovery = useCallback((): Promise<void> => {
    if (!task || !state.recoveryTarget) return Promise.resolve();
    if (task.deletion_state?.trim()) {
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
    ), {
      failure: "释放运行占用失败",
      refreshFailure: "运行历史刷新失败",
      success: "已释放运行占用",
    });
  }, [onRecoverTaskRun, runAction, state.recoveryTarget, task, updateActiveState]);

  return {
    ...state,
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

function blockedActionFeedback(
  task: ScheduledTaskItem,
  message?: string,
): ScheduledTaskFeedback {
  const deletionState = task.deletion_state?.trim() ?? "";
  const reviewRequired = deletionState === "review_required";
  return {
    impact: "本次点击没有修改任务、运行记录或投递状态。",
    message: message ?? (reviewRequired
      ? "删除正在等待管理员处理，任务不再接受新的运行或投递操作。"
      : "删除已经受理，任务不再接受新的运行或投递操作。"),
    nextStep: reviewRequired
      ? "返回任务详情，确认原执行端已经停止后再继续删除；也可以先刷新或查看历史。"
      : deletionState
        ? "等待删除收尾完成；也可以先刷新任务状态或查看历史。"
        : "先刷新任务和运行历史，完成当前待处理操作后再试。",
    title: "操作未执行",
    tone: "warning",
  };
}
