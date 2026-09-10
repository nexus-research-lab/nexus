/**
 * INPUT: 当前 owner scope、定时任务、运行历史资源与恢复/重试动作。
 * OUTPUT: 公共标题命名、随语言更新的 plain 历史工作面；结果事实分别映射 success/warning/error 反馈。
 * POS: Scheduled 历史模态边界；内部 Job ID 只留在诊断详情。
 */
"use client";

import { useEffect } from "react";
import { RefreshCw } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton } from "@/shared/ui/button/button";
import { UiResourceState } from "@/shared/ui/display/resource-state";
import { ConfirmDialog } from "@/shared/ui/dialog/decision/decision-dialog";
import {
  UiDialogBackdrop,
  UiDialogBody,
  UiDialogHeader,
  UiDialogPortal,
  UiDialogShell,
} from "@/shared/ui/dialog/dialog";
import { UiBadge } from "@/shared/ui/display/badge";
import type { ScheduledTaskRunItem } from "@/types/capability/scheduled-task/run";
import type { ScheduledTaskItem } from "@/types/capability/scheduled-task/task";

import type { ScheduledTaskUnconfirmedCommands } from "../controller/scheduled-task-directory-model";
import { getTaskStatusMeta } from "./scheduled-task-run-history-model";
import {
  useScheduledTaskRunHistoryActions,
  type ScheduledTaskRunHistoryActionResult,
} from "./use-scheduled-task-run-history-actions";
import { useScheduledTaskRunHistoryResource } from "./use-scheduled-task-run-history-resource";
import { ScheduledTaskRunHistoryContent } from "./view/scheduled-task-run-history-content";

const EMPTY_PENDING_RUN_IDS: ReadonlySet<string> = new Set();

interface ScheduledTaskRunHistoryDialogProps {
  isOpen: boolean;
  onClose: () => void;
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
  onRunHistoryReconciled: (
    task: ScheduledTaskItem,
    runs: ScheduledTaskRunItem[],
  ) => void;
  onRetryTask: (
    task: ScheduledTaskItem,
    reconcileHistory: () => Promise<void>,
  ) => ScheduledTaskRunHistoryActionResult | Promise<ScheduledTaskRunHistoryActionResult>;
  scopeKey: string | null;
  task: ScheduledTaskItem | null;
  unconfirmed: ScheduledTaskUnconfirmedCommands;
}

export function ScheduledTaskRunHistoryDialog({
  isOpen,
  onClose,
  onRecoverTaskRun,
  onRetryDelivery,
  onRunHistoryReconciled,
  onRetryTask,
  scopeKey,
  task,
  unconfirmed,
}: ScheduledTaskRunHistoryDialogProps) {
  const { t } = useI18n();
  const activeTask = isOpen ? task : null;
  const resource = useScheduledTaskRunHistoryResource(
    activeTask?.job_id ?? null,
    scopeKey,
  );
  const refreshAndReconcile = async (): Promise<void> => {
    if (!activeTask) return;
    const runs = await resource.refresh();
    onRunHistoryReconciled(activeTask, runs);
  };
  const actions = useScheduledTaskRunHistoryActions({
    onRecoverTaskRun,
    onRetryDelivery,
    onRetryTask,
    reconcileHistory: refreshAndReconcile,
    refresh: resource.refresh,
    scopeKey,
    task: activeTask,
  });
  const accessBlocked = Boolean(resource.failure?.access);
  const deletionBlocked = Boolean(activeTask?.deletion_state?.trim());
  const cancelDeliveryVerification = actions.cancelDeliveryVerification;
  const cancelRecovery = actions.cancelRecovery;
  useEffect(() => {
    if (accessBlocked || deletionBlocked) {
      cancelDeliveryVerification();
      cancelRecovery();
    }
  }, [accessBlocked, cancelDeliveryVerification, cancelRecovery, deletionBlocked]);

  if (!activeTask) {
    return null;
  }

  const taskStatus = getTaskStatusMeta(activeTask, t);
  return (
    <>
      <UiDialogPortal>
        <UiDialogBackdrop
          closeOnBackdrop={false}
          onClose={onClose}
        >
        <UiDialogShell size="xl" viewport="adaptive">
            <UiDialogHeader
              appearance="plain"
              actions={(
                <UiButton
                  aria-busy={resource.isLoading}
                  disabled={resource.isLoading}
                  onClick={() => void refreshAndReconcile().catch(() => undefined)}
                  size="xs"
                  type="button"
                  variant="text"
                >
                  <RefreshCw className="h-3.5 w-3.5" />
                  {t("common.refresh")}
                </UiButton>
              )}
              onClose={onClose}
              title={(
                <span className="flex min-w-0 items-center gap-2">
                  <span className="min-w-0 truncate">{activeTask.name}</span>
                  <UiBadge showDot size="xs" tone={taskStatus.tone}>
                    {taskStatus.label}
                  </UiBadge>
                </span>
              )}
            />

            <UiDialogBody className="min-h-0 flex-1" scrollable>
              {actions.feedback ? (
                <UiResourceState
                  className="mb-3 min-h-0 py-4"
                  impact={actions.feedback.impact}
                  nextStep={actions.feedback.nextStep}
                  size="sm"
                  {...(actions.feedback.tone === "success"
                    ? { state: "success" as const }
                    : { state: "error" as const, tone: actions.feedback.tone === "warning" ? "warning" as const : "danger" as const })}
                  title={actions.feedback.title}
                />
              ) : null}
              <ScheduledTaskRunHistoryContent
                copiedRunId={actions.copiedRunId}
                failure={resource.failure}
                hasSnapshot={resource.hasSnapshot}
                isLoading={resource.isLoading}
                onCopyDiagnostic={actions.copyDiagnostic}
                onRecover={actions.recover}
                onRefresh={() => void refreshAndReconcile().catch(() => undefined)}
                onRetry={actions.retry}
                onRetryDelivery={actions.retryDelivery}
                pendingRecoveries={actions.pending.get("recover") ?? EMPTY_PENDING_RUN_IDS}
                pendingRetries={actions.pending.get("retry") ?? EMPTY_PENDING_RUN_IDS}
                pendingRetryDeliveries={actions.pending.get("retryDelivery") ?? EMPTY_PENDING_RUN_IDS}
                runs={resource.runs}
                task={activeTask}
                unconfirmed={unconfirmed}
              />
            </UiDialogBody>
          </UiDialogShell>
        </UiDialogBackdrop>
      </UiDialogPortal>
      <ConfirmDialog
        confirmText={t("capability.scheduled_history_recover")}
        isOpen={!accessBlocked && !deletionBlocked && actions.recoveryTarget !== null}
        message={t("capability.scheduled_history_recover_message")}
        onCancel={cancelRecovery}
        onConfirm={() => {
          if (accessBlocked || deletionBlocked) {
            cancelRecovery();
            return;
          }
          void actions.confirmRecovery();
        }}
        title={t("capability.scheduled_history_recover_title")}
        variant="danger"
      />
      <ConfirmDialog
        confirmText={t("capability.scheduled_history_delivery_confirm")}
        isOpen={!accessBlocked && !deletionBlocked && actions.deliveryVerificationTarget !== null}
        message={t("capability.scheduled_history_delivery_confirm_message")}
        onCancel={cancelDeliveryVerification}
        onConfirm={() => {
          if (accessBlocked || deletionBlocked) {
            cancelDeliveryVerification();
            return;
          }
          void actions.confirmDeliveryVerification();
        }}
        title={t("capability.scheduled_history_delivery_confirm_title")}
      />
    </>
  );
}
