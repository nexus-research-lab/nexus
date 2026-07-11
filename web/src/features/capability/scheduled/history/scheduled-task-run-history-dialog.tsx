"use client";

import { useEffect } from "react";
import { RefreshCw, X } from "lucide-react";

import { UiButton, UiIconButton } from "@/shared/ui/button/button";
import { closeOnEscape } from "@/shared/ui/dialog/dialog-keyboard";
import { WorkspaceStatusBadge } from "@/shared/ui/workspace/controls/workspace-status-badge";
import type {
  ScheduledTaskItem,
  ScheduledTaskRunItem,
} from "@/types/capability/scheduled-task";

import { getTaskStatusMeta } from "./scheduled-task-run-history-model";
import { useScheduledTaskRunHistoryActions } from "./use-scheduled-task-run-history-actions";
import { useScheduledTaskRunHistoryResource } from "./use-scheduled-task-run-history-resource";
import { ScheduledTaskRunHistoryContent } from "./view/scheduled-task-run-history-content";

const EMPTY_PENDING_RUN_IDS: ReadonlySet<string> = new Set();

interface ScheduledTaskRunHistoryDialogProps {
  isOpen: boolean;
  onClose: () => void;
  onRecoverTaskRun: (
    task: ScheduledTaskItem,
    run: ScheduledTaskRunItem,
  ) => void | Promise<void>;
  onRetryDelivery: (
    task: ScheduledTaskItem,
    run: ScheduledTaskRunItem,
  ) => void | Promise<void>;
  onRetryTask: (task: ScheduledTaskItem) => void | Promise<void>;
  task: ScheduledTaskItem | null;
}

export function ScheduledTaskRunHistoryDialog({
  isOpen,
  onClose,
  onRecoverTaskRun,
  onRetryDelivery,
  onRetryTask,
  task,
}: ScheduledTaskRunHistoryDialogProps) {
  const activeTask = isOpen ? task : null;
  const resource = useScheduledTaskRunHistoryResource(activeTask?.job_id ?? null);
  const actions = useScheduledTaskRunHistoryActions({
    onRecoverTaskRun,
    onRetryDelivery,
    onRetryTask,
    refresh: resource.refresh,
    task: activeTask,
  });

  useEffect(() => {
    if (!isOpen) {
      return;
    }
    const onKeyDown = (event: KeyboardEvent) => closeOnEscape(event, onClose);
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [isOpen, onClose]);

  if (!activeTask) {
    return null;
  }

  const taskStatus = getTaskStatusMeta(activeTask);
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
                {activeTask.name} 运行历史
              </h3>
              <WorkspaceStatusBadge
                label={taskStatus.label}
                size="compact"
                tone={taskStatus.tone}
              />
            </div>
            <p className="dialog-subtitle mt-1">Job ID: {activeTask.job_id}</p>
            {actions.message ? (
              <p className="mt-2 text-xs font-medium text-(--text-default)">
                {actions.message}
              </p>
            ) : null}
          </div>
          <div className="flex items-center gap-2">
            <UiButton
              onClick={() => void resource.refresh().catch(() => undefined)}
              size="xs"
              type="button"
              variant="text"
            >
              <RefreshCw className="h-3.5 w-3.5" />
              刷新
            </UiButton>
            <UiIconButton aria-label="关闭" onClick={onClose} size="md" type="button">
              <X className="h-4 w-4" />
            </UiIconButton>
          </div>
        </div>

        <ScheduledTaskRunHistoryContent
          copiedRunId={actions.copiedRunId}
          errorMessage={resource.errorMessage}
          isLoading={resource.isLoading}
          onCopyDiagnostic={actions.copyDiagnostic}
          onRecover={actions.recover}
          onRetry={actions.retry}
          onRetryDelivery={actions.retryDelivery}
          pendingRecoveries={actions.pending.get("recover") ?? EMPTY_PENDING_RUN_IDS}
          pendingRetries={actions.pending.get("retry") ?? EMPTY_PENDING_RUN_IDS}
          pendingRetryDeliveries={actions.pending.get("retryDelivery") ?? EMPTY_PENDING_RUN_IDS}
          runs={resource.runs}
          task={activeTask}
        />
      </div>
    </div>
  );
}
