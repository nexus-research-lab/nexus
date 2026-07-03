"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { History, RefreshCw, X } from "lucide-react";

import { writeTextToClipboard } from "@/hooks/ui/clipboard";
import { useResettableState } from "@/hooks/ui/use-resettable-state";
import { listScheduledTaskRunsApi } from "@/lib/api/scheduled-task-api";
import { UiButton, UiIconButton } from "@/shared/ui/button";
import { closeOnEscape } from "@/shared/ui/dialog/dialog-keyboard";
import { UiSkeletonCardList } from "@/shared/ui/skeleton";
import { UiStateBlock } from "@/shared/ui/state-block";
import { WorkspaceStatusBadge } from "@/shared/ui/workspace/controls/workspace-status-badge";
import type { ScheduledTaskItem, ScheduledTaskRunItem } from "@/types/capability/scheduled-task";
import { ScheduledTaskRunHistoryItem } from "./scheduled-task-run-history-item";
import { buildRunDiagnostic } from "./scheduled-task-run-history-model";

interface ScheduledTaskRunHistoryDialogProps {
  task: ScheduledTaskItem | null;
  isOpen: boolean;
  onClose: () => void;
  onRetryTask?: (task: ScheduledTaskItem) => void | Promise<void>;
  onRetryDelivery?: (task: ScheduledTaskItem, run: ScheduledTaskRunItem) => void | Promise<void>;
  onRecoverTaskRun?: (task: ScheduledTaskItem, run: ScheduledTaskRunItem) => void | Promise<void>;
}

interface RunHistoryDialogState {
  actionMessage: string | null;
  copiedRunId: string | null;
  errorMessage: string | null;
  isLoading: boolean;
  recoveringRunId: string | null;
  retryingDeliveryRunId: string | null;
  retryingRunId: string | null;
  runs: ScheduledTaskRunItem[];
}

export function ScheduledTaskRunHistoryDialog({
  task,
  isOpen: isOpen,
  onClose: onClose,
  onRetryTask: onRetryTask,
  onRetryDelivery: onRetryDelivery,
  onRecoverTaskRun: onRecoverTaskRun,
}: ScheduledTaskRunHistoryDialogProps) {
  const activeTaskJobIdRef = useRef<string | null>(null);
  const runsRequestTokenRef = useRef(0);
  const taskJobId = task?.job_id ?? null;
  const [state, setState] = useResettableState<RunHistoryDialogState>(
    {
      actionMessage: null,
      copiedRunId: null,
      errorMessage: null,
      isLoading: Boolean(isOpen && taskJobId),
      recoveringRunId: null,
      retryingDeliveryRunId: null,
      retryingRunId: null,
      runs: [],
    },
    isOpen && taskJobId ? taskJobId : "closed",
  );
  const {
    actionMessage,
    copiedRunId,
    errorMessage,
    isLoading,
    recoveringRunId,
    retryingDeliveryRunId,
    retryingRunId,
    runs,
  } = state;

  const loadRuns = useCallback(async (jobId: string) => {
    const requestToken = runsRequestTokenRef.current + 1;
    runsRequestTokenRef.current = requestToken;
    setState((current) => ({ ...current, errorMessage: null, isLoading: true }));
    try {
      const result = await listScheduledTaskRunsApi(jobId);
      if (activeTaskJobIdRef.current !== jobId || runsRequestTokenRef.current !== requestToken) {
        return;
      }
      setState((current) => ({ ...current, runs: result }));
    } catch (error) {
      if (activeTaskJobIdRef.current !== jobId || runsRequestTokenRef.current !== requestToken) {
        return;
      }
      setState((current) => ({
        ...current,
        errorMessage: error instanceof Error ? error.message : "加载运行历史失败",
        runs: [],
      }));
    } finally {
      if (activeTaskJobIdRef.current !== jobId || runsRequestTokenRef.current !== requestToken) {
        return;
      }
      setState((current) => ({ ...current, isLoading: false }));
    }
  }, [setState]);

  useEffect(() => {
    if (!isOpen) {
      activeTaskJobIdRef.current = null;
      runsRequestTokenRef.current += 1;
      return;
    }
    const onKeyDown = (event: KeyboardEvent) => closeOnEscape(event, onClose);
    window.addEventListener("keydown", onKeyDown);
    return () => {
      window.removeEventListener("keydown", onKeyDown);
    };
  }, [isOpen, onClose]);

  useEffect(() => {
    if (!isOpen || !taskJobId) {
      activeTaskJobIdRef.current = null;
      runsRequestTokenRef.current += 1;
      return;
    }
    activeTaskJobIdRef.current = taskJobId;
    void loadRuns(taskJobId);
  }, [isOpen, loadRuns, taskJobId]);

  if (!isOpen || !task) {
    return null;
  }

  const handleRefresh = () => {
    void loadRuns(taskJobId ?? "");
  };

  const handleCopyDiagnostic = async (run: ScheduledTaskRunItem) => {
    const diagnostic = buildRunDiagnostic(task, run);
    if (await writeTextToClipboard(diagnostic)) {
      setState((current) => ({
        ...current,
        actionMessage: "诊断信息已复制",
        copiedRunId: run.run_id,
      }));
      return;
    }
    setState((current) => ({
      ...current,
      actionMessage: "浏览器未允许写入剪贴板，请使用运行产物查看完整诊断",
    }));
  };

  const handleRetry = async (run: ScheduledTaskRunItem) => {
    if (!onRetryTask || !taskJobId) {
      return;
    }
    setState((current) => ({ ...current, actionMessage: null, retryingRunId: run.run_id }));
    try {
      await onRetryTask(task);
      await loadRuns(taskJobId);
      setState((current) => ({ ...current, actionMessage: "已触发重新运行" }));
    } catch (error) {
      setState((current) => ({
        ...current,
        actionMessage: error instanceof Error ? error.message : "重新运行失败",
      }));
    } finally {
      setState((current) => ({ ...current, retryingRunId: null }));
    }
  };

  const handleRetryDelivery = async (run: ScheduledTaskRunItem) => {
    if (!onRetryDelivery || !taskJobId) {
      return;
    }
    setState((current) => ({ ...current, actionMessage: null, retryingDeliveryRunId: run.run_id }));
    try {
      await onRetryDelivery(task, run);
      await loadRuns(taskJobId);
      setState((current) => ({ ...current, actionMessage: "已重试投递" }));
    } catch (error) {
      setState((current) => ({
        ...current,
        actionMessage: error instanceof Error ? error.message : "重试投递失败",
      }));
    } finally {
      setState((current) => ({ ...current, retryingDeliveryRunId: null }));
    }
  };

  const handleRecover = async (run: ScheduledTaskRunItem) => {
    if (!onRecoverTaskRun || !taskJobId) {
      return;
    }
    if (!window.confirm(`确认释放 run ${run.run_id} 的运行占用吗？该 run 会被标记为 cancelled。`)) {
      return;
    }
    setState((current) => ({ ...current, actionMessage: null, recoveringRunId: run.run_id }));
    try {
      await onRecoverTaskRun(task, run);
      await loadRuns(taskJobId);
      setState((current) => ({ ...current, actionMessage: "已释放运行占用" }));
    } catch (error) {
      setState((current) => ({
        ...current,
        actionMessage: error instanceof Error ? error.message : "释放运行占用失败",
      }));
    } finally {
      setState((current) => ({ ...current, recoveringRunId: null }));
    }
  };

  return (
    <div
      aria-labelledby="scheduled-task-run-history-title"
      aria-modal="true"
      className="dialog-backdrop"
      data-modal-root="true"
      role="dialog"
    >
      <div className="dialog-shell surface-radius-md flex h-[82vh] w-full max-w-4xl flex-col overflow-hidden">
        <div className="dialog-header">
          <div className="min-w-0 flex-1">
            <div className="flex items-center gap-2">
              <h3 className="dialog-title" id="scheduled-task-run-history-title">
                {task.name} 运行历史
              </h3>
              <WorkspaceStatusBadge
                label={task.running ? "运行中" : task.enabled ? "已启用" : "已暂停"}
                size="compact"
                tone={task.running ? "running" : task.enabled ? "active" : "idle"}
              />
            </div>
            <p className="dialog-subtitle mt-1">
              Job ID: {task.job_id}
            </p>
            {actionMessage ? (
              <p className="mt-2 text-xs font-medium text-(--text-default)">
                {actionMessage}
              </p>
            ) : null}
          </div>
          <div className="flex items-center gap-2">
            <UiButton
              onClick={() => void handleRefresh()}
              size="xs"
              type="button"
              variant="text"
            >
              <RefreshCw className="h-3.5 w-3.5" />
              刷新
            </UiButton>
            <UiIconButton
              aria-label="关闭"
              onClick={onClose}
              size="md"
              type="button"
            >
              <X className="h-4 w-4" />
            </UiIconButton>
          </div>
        </div>

        <div className="soft-scrollbar min-h-0 flex-1 overflow-y-auto px-6 py-5">
          {isLoading ? (
            <UiSkeletonCardList cardClassName="min-h-[108px]" count={4} />
          ) : errorMessage ? (
            <UiStateBlock description={errorMessage} title="运行历史加载失败" tone="danger" />
          ) : runs.length === 0 ? (
            <UiStateBlock
              description="手动执行或等调度器首次触发后，这里会显示每次运行的状态、耗时和错误信息。"
              icon={<History className="h-6 w-6 text-(--icon-strong)" />}
              title="还没有运行记录"
            />
          ) : (
            <div className="divide-y divide-(--divider-subtle-color)">
              {runs.map((run) => (
                <ScheduledTaskRunHistoryItem
                  canRecoverTaskRun={Boolean(onRecoverTaskRun)}
                  canRetryDelivery={Boolean(onRetryDelivery)}
                  canRetryTask={Boolean(onRetryTask)}
                  copiedRunId={copiedRunId}
                  key={run.run_id}
                  onCopyDiagnostic={handleCopyDiagnostic}
                  onRecover={handleRecover}
                  onRetry={handleRetry}
                  onRetryDelivery={handleRetryDelivery}
                  recoveringRunId={recoveringRunId}
                  retryingDeliveryRunId={retryingDeliveryRunId}
                  retryingRunId={retryingRunId}
                  run={run}
                  task={task}
                />
              ))}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
