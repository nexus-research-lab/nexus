"use client";

import { useCallback, useLayoutEffect, useRef } from "react";

import { writeTextToClipboard } from "@/hooks/ui/clipboard";
import { useResettableState } from "@/hooks/ui/use-resettable-state";
import type {
  ScheduledTaskItem,
  ScheduledTaskRunItem,
} from "@/types/capability/scheduled-task";

import {
  createPendingCommandState,
  type PendingCommandState,
  setPendingCommand,
} from "../controller/pending-command-model";
import { buildRunDiagnostic } from "./scheduled-task-run-history-model";

const RUN_HISTORY_ACTIONS = ["recover", "retry", "retryDelivery"] as const;
type RunHistoryAction = typeof RUN_HISTORY_ACTIONS[number];
type RunHistoryPendingActions = PendingCommandState<RunHistoryAction>;

interface RunHistoryActionState {
  copiedRunId: string | null;
  message: string | null;
  pending: RunHistoryPendingActions;
}

interface RunHistoryActionCommands {
  onRecoverTaskRun: (
    task: ScheduledTaskItem,
    run: ScheduledTaskRunItem,
  ) => void | Promise<void>;
  onRetryDelivery: (
    task: ScheduledTaskItem,
    run: ScheduledTaskRunItem,
  ) => void | Promise<void>;
  onRetryTask: (task: ScheduledTaskItem) => void | Promise<void>;
}

interface RunHistoryActionsOptions extends RunHistoryActionCommands {
  refresh: () => Promise<void>;
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
    message: null,
    pending: createPendingCommandState(RUN_HISTORY_ACTIONS),
  };
}

export function useScheduledTaskRunHistoryActions({
  onRecoverTaskRun,
  onRetryDelivery,
  onRetryTask,
  refresh,
  task,
}: RunHistoryActionsOptions) {
  const taskJobId = task?.job_id ?? null;
  const [state, setState] = useResettableState(
    createInitialActionState(),
    taskJobId ?? "closed",
  );
  const activeTaskJobIdRef = useRef<string | null>(taskJobId);
  const pendingPromisesRef = useRef(new Map<string, Promise<void>>());

  useLayoutEffect(() => {
    activeTaskJobIdRef.current = taskJobId;
    pendingPromisesRef.current.clear();
    return () => {
      if (activeTaskJobIdRef.current === taskJobId) {
        activeTaskJobIdRef.current = null;
      }
    };
  }, [taskJobId]);

  const updateActiveState = useCallback((
    jobId: string,
    update: (current: RunHistoryActionState) => RunHistoryActionState,
  ): void => {
    if (activeTaskJobIdRef.current === jobId) {
      setState(update);
    }
  }, [setState]);

  const runAction = useCallback((
    action: RunHistoryAction,
    run: ScheduledTaskRunItem,
    execute: (activeTask: ScheduledTaskItem) => void | Promise<void>,
    copy: RunActionCopy,
  ): Promise<void> => {
    if (!task) {
      return Promise.resolve();
    }
    const commandKey = `${action}:${task.job_id}:${run.run_id}`;
    const pendingPromise = pendingPromisesRef.current.get(commandKey);
    if (pendingPromise) {
      return pendingPromise;
    }
    const activeTask = task;
    updateActiveState(activeTask.job_id, (current) => ({
      ...current,
      message: null,
      pending: setPendingCommand(current.pending, action, run.run_id, true),
    }));
    const nextPromise = (async () => {
      try {
        await execute(activeTask);
        updateActiveState(activeTask.job_id, (current) => ({
          ...current,
          message: copy.success,
        }));
        try {
          await refresh();
        } catch (error) {
          const detail = error instanceof Error ? `（${error.message}）` : "";
          updateActiveState(activeTask.job_id, (current) => ({
            ...current,
            message: `${copy.success}；${copy.refreshFailure}${detail}`,
          }));
        }
      } catch (error) {
        updateActiveState(activeTask.job_id, (current) => ({
          ...current,
          message: error instanceof Error ? error.message : copy.failure,
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
  }, [refresh, task, updateActiveState]);

  const copyDiagnostic = useCallback(async (run: ScheduledTaskRunItem): Promise<void> => {
    if (!task) {
      return;
    }
    const copied = await writeTextToClipboard(buildRunDiagnostic(task, run));
    updateActiveState(task.job_id, (current) => ({
      ...current,
      copiedRunId: copied ? run.run_id : current.copiedRunId,
      message: copied
        ? "诊断信息已复制"
        : "浏览器未允许写入剪贴板，请使用运行产物查看完整诊断",
    }));
  }, [task, updateActiveState]);

  const retry = useCallback((run: ScheduledTaskRunItem): Promise<void> => (
    runAction("retry", run, onRetryTask, {
      failure: "重新运行失败",
      refreshFailure: "运行历史刷新失败",
      success: "已触发重新运行",
    })
  ), [onRetryTask, runAction]);

  const retryDelivery = useCallback((run: ScheduledTaskRunItem): Promise<void> => (
    runAction("retryDelivery", run, (activeTask) => (
      onRetryDelivery(activeTask, run)
    ), {
      failure: "重试投递失败",
      refreshFailure: "运行历史刷新失败",
      success: "已重试投递",
    })
  ), [onRetryDelivery, runAction]);

  const recover = useCallback((run: ScheduledTaskRunItem): Promise<void> => {
    if (!window.confirm(`确认释放 run ${run.run_id} 的运行占用吗？该 run 会被标记为 cancelled。`)) {
      return Promise.resolve();
    }
    return runAction("recover", run, (activeTask) => (
      onRecoverTaskRun(activeTask, run)
    ), {
      failure: "释放运行占用失败",
      refreshFailure: "运行历史刷新失败",
      success: "已释放运行占用",
    });
  }, [onRecoverTaskRun, runAction]);

  return {
    ...state,
    copyDiagnostic,
    recover,
    retry,
    retryDelivery,
  };
}
