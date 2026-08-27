/**
 * INPUT: 当前定时任务、运行历史资源与恢复/重试动作。
 * OUTPUT: 以任务名和状态命名的 plain 运行历史工作面。
 * POS: Scheduled 历史模态边界；内部 Job ID 只留在诊断详情。
 */
"use client";

import { useEffect } from "react";
import { RefreshCw } from "lucide-react";

import { UiButton } from "@/shared/ui/button/button";
import { ConfirmDialog } from "@/shared/ui/dialog/decision/decision-dialog";
import {
  UiDialogBackdrop,
  UiDialogBody,
  UiDialogHeader,
  UiDialogPortal,
  UiDialogShell,
} from "@/shared/ui/dialog/dialog";
import { WorkspaceStatusBadge } from "@/shared/ui/workspace/controls/workspace-status-badge";
import type { ScheduledTaskRunItem } from "@/types/capability/scheduled-task/run";
import type { ScheduledTaskItem } from "@/types/capability/scheduled-task/task";

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
  const accessBlocked = Boolean(resource.failure?.access);
  const cancelRecovery = actions.cancelRecovery;
  useEffect(() => {
    if (accessBlocked) {
      cancelRecovery();
    }
  }, [accessBlocked, cancelRecovery]);

  if (!activeTask) {
    return null;
  }

  const taskStatus = getTaskStatusMeta(activeTask);
  return (
    <>
      <UiDialogPortal>
        <UiDialogBackdrop
          closeOnBackdrop={false}
          labelledBy="scheduled-task-run-history-title"
          onClose={onClose}
        >
          <UiDialogShell className="h-[82vh]" size="xl">
            <UiDialogHeader
              appearance="plain"
              actions={(
                <UiButton
                  onClick={() => void resource.refresh().catch(() => undefined)}
                  size="xs"
                  type="button"
                  variant="text"
                >
                  <RefreshCw className="h-3.5 w-3.5" />
                  刷新
                </UiButton>
              )}
              onClose={onClose}
            >
              <div className="flex min-w-0 flex-1 items-center gap-2">
                <h2 className="dialog-title" id="scheduled-task-run-history-title">
                  {activeTask.name}
                </h2>
                <WorkspaceStatusBadge
                  label={taskStatus.label}
                  size="compact"
                  tone={taskStatus.tone}
                />
              </div>
            </UiDialogHeader>

            <UiDialogBody className="min-h-0 flex-1" scrollable>
              {actions.message ? (
                <p className="mb-3 text-xs font-medium text-(--text-default)">
                  {actions.message}
                </p>
              ) : null}
              <ScheduledTaskRunHistoryContent
                copiedRunId={actions.copiedRunId}
                failure={resource.failure}
                hasSnapshot={resource.hasSnapshot}
                isLoading={resource.isLoading}
                onCopyDiagnostic={actions.copyDiagnostic}
                onRecover={actions.recover}
                onRefresh={() => void resource.refresh().catch(() => undefined)}
                onRetry={actions.retry}
                onRetryDelivery={actions.retryDelivery}
                pendingRecoveries={actions.pending.get("recover") ?? EMPTY_PENDING_RUN_IDS}
                pendingRetries={actions.pending.get("retry") ?? EMPTY_PENDING_RUN_IDS}
                pendingRetryDeliveries={actions.pending.get("retryDelivery") ?? EMPTY_PENDING_RUN_IDS}
                runs={resource.runs}
                task={activeTask}
              />
            </UiDialogBody>
          </UiDialogShell>
        </UiDialogBackdrop>
      </UiDialogPortal>
      <ConfirmDialog
        confirmText="释放占用"
        isOpen={!accessBlocked && actions.recoveryTarget !== null}
        message="这次运行会标记为已取消，任务随后可以重新运行。"
        onCancel={cancelRecovery}
        onConfirm={() => {
          if (accessBlocked) {
            cancelRecovery();
            return;
          }
          void actions.confirmRecovery();
        }}
        title="释放运行占用"
        variant="danger"
      />
    </>
  );
}
