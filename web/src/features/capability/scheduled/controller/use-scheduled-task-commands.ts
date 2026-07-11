"use client";

import { useCallback, useRef, useState } from "react";

import {
  deleteScheduledTaskApi,
  recoverScheduledTaskRunApi,
  retryScheduledTaskRunDeliveryApi,
  runScheduledTaskApi,
  updateScheduledTaskStatusApi,
} from "@/lib/api/scheduled-task-api";
import type {
  ScheduledTaskItem,
  ScheduledTaskRunItem,
  ScheduledTaskRunNowResponse,
} from "@/types/capability/scheduled-task";

import { notifyScheduledTasksMutated } from "../../scheduled-task-events";
import {
  getCommandErrorMessage,
  SCHEDULED_TASK_COMMAND_KINDS,
  type ScheduledTaskCommandKind,
  type ScheduledTaskFeedback,
} from "./scheduled-task-directory-model";
import {
  createPendingCommandState,
  setPendingCommand,
} from "./pending-command-model";

interface ScheduledTaskCommandResource {
  refresh: (options?: { silent?: boolean }) => Promise<void>;
  removeTask: (jobId: string) => void;
  upsertTask: (task: ScheduledTaskItem) => void;
}

interface MutationFeedback {
  message: string;
  refreshWarning: string;
  title: string;
}

interface CommandFailureFeedback {
  fallbackMessage: string;
  title: string;
}

export function useScheduledTaskCommands(resource: ScheduledTaskCommandResource) {
  const { refresh, removeTask, upsertTask } = resource;
  const [feedback, setFeedback] = useState<ScheduledTaskFeedback | null>(null);
  const [pending, setPending] = useState(() => (
    createPendingCommandState(SCHEDULED_TASK_COMMAND_KINDS)
  ));
  const pendingPromisesRef = useRef(new Map<string, Promise<unknown>>());

  const runPending = useCallback(<Result,>(
    command: ScheduledTaskCommandKind,
    jobId: string,
    execute: () => Promise<Result>,
  ): Promise<Result> => {
    const commandKey = `${command}:${jobId}`;
    const pendingPromise = pendingPromisesRef.current.get(commandKey);
    if (pendingPromise) {
      return pendingPromise as Promise<Result>;
    }
    setPending((current) => setPendingCommand(current, command, jobId, true));
    const nextPromise = execute().finally(() => {
      pendingPromisesRef.current.delete(commandKey);
      setPending((current) => setPendingCommand(current, command, jobId, false));
    });
    pendingPromisesRef.current.set(commandKey, nextPromise);
    return nextPromise;
  }, []);

  const executeCommand = useCallback(async <Result,>(
    execute: () => Promise<Result>,
    failure: CommandFailureFeedback,
  ): Promise<Result> => {
    try {
      return await execute();
    } catch (error) {
      setFeedback({
        message: getCommandErrorMessage(error, failure.fallbackMessage),
        title: failure.title,
        tone: "error",
      });
      throw error;
    }
  }, []);

  const synchronizeMutation = useCallback(async (
    agentId: string,
    success: MutationFeedback,
  ): Promise<void> => {
    notifyScheduledTasksMutated(agentId);
    try {
      await refresh({ silent: true });
      setFeedback({
        message: success.message,
        title: success.title,
        tone: "success",
      });
    } catch (error) {
      const detail = error instanceof Error ? `（${error.message}）` : "";
      setFeedback({
        message: `${success.message}；${success.refreshWarning}${detail}`,
        title: success.title,
        tone: "warning",
      });
    }
  }, [refresh]);

  const acceptCreatedTask = useCallback(async (task: ScheduledTaskItem): Promise<void> => {
    upsertTask(task);
    await synchronizeMutation(task.agent_id, {
      message: `${task.name} 已加入自动化任务列表`,
      refreshWarning: "任务列表刷新失败，稍后会自动同步",
      title: "任务已创建",
    });
  }, [synchronizeMutation, upsertTask]);

  const acceptSavedTask = useCallback(async (task: ScheduledTaskItem): Promise<void> => {
    upsertTask(task);
    await synchronizeMutation(task.agent_id, {
      message: `${task.name} 的配置已保存`,
      refreshWarning: "任务列表刷新失败，稍后会自动同步",
      title: "任务已更新",
    });
  }, [synchronizeMutation, upsertTask]);

  const runTask = useCallback(async (
    task: ScheduledTaskItem,
  ): Promise<ScheduledTaskRunNowResponse> => runPending(
    "run",
    task.job_id,
    () => executeCommand(async () => {
      const result = await runScheduledTaskApi(task.job_id);
      await synchronizeMutation(task.agent_id, {
        message: result.status === "queued_to_main_session"
          ? `${task.name} 已排入主会话执行`
          : `${task.name} 已开始执行`,
        refreshWarning: "任务列表刷新失败，运行状态稍后会同步",
        title: "任务已触发",
      });
      return result;
    }, {
      fallbackMessage: "立即运行失败",
      title: "任务执行失败",
    }),
  ), [executeCommand, runPending, synchronizeMutation]);

  const toggleTask = useCallback(async (
    task: ScheduledTaskItem,
  ): Promise<ScheduledTaskItem> => runPending(
    "toggle",
    task.job_id,
    () => executeCommand(async () => {
      const updatedTask = await updateScheduledTaskStatusApi(task.job_id, {
        enabled: !task.enabled,
      });
      upsertTask(updatedTask);
      await synchronizeMutation(updatedTask.agent_id, {
        message: updatedTask.enabled
          ? `${updatedTask.name} 已恢复自动调度`
          : `${updatedTask.name} 不再参与后续调度`,
        refreshWarning: "任务列表刷新失败，状态稍后会同步",
        title: updatedTask.enabled ? "任务已启用" : "任务已暂停",
      });
      return updatedTask;
    }, {
      fallbackMessage: "切换任务状态失败",
      title: "状态更新失败",
    }),
  ), [executeCommand, runPending, synchronizeMutation, upsertTask]);

  const deleteTask = useCallback(async (task: ScheduledTaskItem): Promise<void> => (
    runPending(
      "delete",
      task.job_id,
      () => executeCommand(async () => {
        await deleteScheduledTaskApi(task.job_id);
        removeTask(task.job_id);
        await synchronizeMutation(task.agent_id, {
          message: `${task.name} 已从自动化任务列表移除`,
          refreshWarning: "任务列表刷新失败，删除结果稍后会同步",
          title: "任务已删除",
        });
      }, {
        fallbackMessage: "删除任务失败",
        title: "删除失败",
      }),
    )
  ), [executeCommand, removeTask, runPending, synchronizeMutation]);

  const recoverRun = useCallback(async (
    task: ScheduledTaskItem,
    run: ScheduledTaskRunItem,
  ): Promise<ScheduledTaskItem> => executeCommand(async () => {
    const updatedTask = await recoverScheduledTaskRunApi(task.job_id, {
      run_id: run.run_id,
    });
    upsertTask(updatedTask);
    await synchronizeMutation(updatedTask.agent_id, {
      message: `${task.name} 的当前 run 已标记为 cancelled`,
      refreshWarning: "任务列表刷新失败，运行状态稍后会同步",
      title: "运行占用已释放",
    });
    return updatedTask;
  }, {
    fallbackMessage: "释放运行占用失败",
    title: "释放运行占用失败",
  }), [executeCommand, synchronizeMutation, upsertTask]);

  const retryDelivery = useCallback(async (
    task: ScheduledTaskItem,
    run: ScheduledTaskRunItem,
  ): Promise<void> => executeCommand(async () => {
    const updatedRun = await retryScheduledTaskRunDeliveryApi(
      task.job_id,
      run.run_id,
    );
    const deliverySucceeded = updatedRun.delivery_status === "succeeded";
    await synchronizeMutation(task.agent_id, {
      message: deliverySucceeded
        ? `${task.name} 的运行结果已重新投递`
        : `${task.name} 的投递状态已更新为 ${updatedRun.delivery_status ?? "unknown"}`,
      refreshWarning: "任务列表刷新失败，投递状态稍后会同步",
      title: deliverySucceeded ? "投递已恢复" : "投递已重试",
    });
  }, {
    fallbackMessage: "重试投递失败",
    title: "重试投递失败",
  }), [executeCommand, synchronizeMutation]);
  const dismissFeedback = useCallback(() => setFeedback(null), []);

  return {
    acceptCreatedTask,
    acceptSavedTask,
    deleteTask,
    dismissFeedback,
    feedback,
    pending,
    recoverRun,
    retryDelivery,
    runTask,
    toggleTask,
  };
}
