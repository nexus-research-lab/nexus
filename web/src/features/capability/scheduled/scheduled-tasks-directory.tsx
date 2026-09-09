/**
 * INPUT: 定时任务资源、durable 删除态、写命令与页面导航。
 * OUTPUT: 任务看板、同 scope 访问重验、编辑/历史工作面、删除确认和删除态写操作 guard。
 * POS: Scheduled 页面装配边界；不使用浏览器原生确认框，不只依赖 disabled 按钮保护。
 */
"use client";

import { useCallback, useEffect, useLayoutEffect, useRef, useState } from "react";
import { Plus, RefreshCw } from "lucide-react";
import { useNavigate } from "react-router-dom";

import { AppRouteBuilders } from "@/shared/navigation/route-paths";
import { CapabilityPageLayout } from "@/features/capability/shared/capability-page-layout";
import { useAuth } from "@/shared/auth/auth-context";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton } from "@/shared/ui/button/button";
import { ConfirmDialog } from "@/shared/ui/dialog/decision/decision-dialog";
import { getUiSpinnerClassName } from "@/shared/ui/display/spinner-styles";
import {
  completeFeedbackBanner,
  type FeedbackBannerProps,
} from "@/shared/ui/feedback/feedback-banner-contract";
import { FeedbackBannerViewport } from "@/shared/ui/feedback/feedback-banner-viewport";
import { WorkspaceSurfaceScaffold } from "@/shared/ui/workspace/surface/workspace-surface-scaffold";
import type { ScheduledTaskRunItem } from "@/types/capability/scheduled-task/run";
import type {
  ScheduledTaskCreateRequestStatus,
  ScheduledTaskItem,
} from "@/types/capability/scheduled-task/task";
import type { AutomationPermissionDecision } from "@/types/capability/scheduled-task/permission";

import { ScheduledTaskBoard } from "./board/scheduled-task-board";
import { isScheduledTaskDeleting } from "./board/scheduled-task-board-model";
import { hasScheduledTaskCommandForJob } from "./controller/scheduled-task-directory-model";
import {
  loadScheduledTaskCreateRequestId,
  subscribeScheduledTaskCreateIntents,
} from "./controller/scheduled-task-create-intent";
import { useScheduledTaskCommands } from "./controller/use-scheduled-task-commands";
import { useScheduledTasksResource } from "./controller/use-scheduled-tasks-resource";
import { ScheduledTaskDialog } from "./dialog/scheduled-task-dialog";
import type { TaskDialogCreatePreset } from "./dialog/scheduled-task-dialog-types";
import { ScheduledTaskRunHistoryDialog } from "./history/scheduled-task-run-history-dialog";
import type { ScheduledTaskRunHistoryActionResult } from "./history/use-scheduled-task-run-history-actions";
import { useScheduledTaskRealtimeRefresh } from "./use-scheduled-task-realtime-refresh";

type TaskDialogState =
  | { kind: "closed" }
  | { kind: "create"; preset: TaskDialogCreatePreset | null }
  | { kind: "edit"; task: ScheduledTaskItem };

export function ScheduledTasksDirectory() {
  const { t } = useI18n();
  const { status: authStatus } = useAuth();
  const navigate = useNavigate();
  const [dialog, setDialog] = useState<TaskDialogState>({ kind: "closed" });
  const [editorBusy, setEditorBusy] = useState(false);
  const [nextEditor, setNextEditor] = useState<TaskDialogState | null>(null);
  const openEditor = (next: TaskDialogState) => {
    if (editorBusy) return;
    if (dialog.kind !== "closed") {
      setNextEditor(next);
      return;
    }
    setDialog(next);
  };
  const [historyTask, setHistoryTask] = useState<ScheduledTaskItem | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<ScheduledTaskItem | null>(null);
  const [deletionStoppedTarget, setDeletionStoppedTarget] = useState<
    ScheduledTaskItem | null
  >(null);
  const ownerScopeKey = authStatus?.authenticated
    ? authStatus.user_id
      ? `owner:${authStatus.user_id}`
      : !authStatus.auth_required
        ? "owner:local-system"
        : null
    : null;
  const [hasPendingCreateIntent, setHasPendingCreateIntent] = useState(() => (
    Boolean(loadScheduledTaskCreateRequestId(ownerScopeKey))
  ));
  const [createRequestResolution, setCreateRequestResolution] = useState<
    Extract<ScheduledTaskCreateRequestStatus, "gone" | "not_found"> | null
  >(null);
  const resource = useScheduledTasksResource(ownerScopeKey);
  const commands = useScheduledTaskCommands({
    invalidateAccess: resource.invalidateAccess,
    isAccessInvalidated: resource.isAccessInvalidated,
    refresh: resource.refresh,
    refreshPermissionRequests: resource.refreshPermissionRequests,
    removeTask: resource.removeTask,
    upsertTask: resource.upsertTask,
  }, ownerScopeKey);
  const accessBlocked = Boolean(resource.failure?.access);
  const scopeUnavailable = ownerScopeKey === null;
  const setCommandsAccessBlocked = commands.setAccessBlocked;
  const invalidateResourceAccess = resource.invalidateAccess;
  const invalidateMutationAccess = useCallback((failure: Parameters<
    typeof invalidateResourceAccess
  >[0]): void => {
    // Mutation 的访问失效必须同步 fence 其他在途命令，再让资源层清快照。
    setCommandsAccessBlocked(true);
    invalidateResourceAccess(failure);
  }, [invalidateResourceAccess, setCommandsAccessBlocked]);
  const handleCreateIntentResolved = useCallback((
    status?: ScheduledTaskCreateRequestStatus,
  ): void => {
    setHasPendingCreateIntent(Boolean(
      loadScheduledTaskCreateRequestId(ownerScopeKey),
    ));
    setCreateRequestResolution(
      status === "gone" || status === "not_found" ? status : null,
    );
  }, [ownerScopeKey]);
  const feedbackItem: FeedbackBannerProps | null = accessBlocked
    ? null
    : commands.feedback
      ? completeFeedbackBanner(
          commands.feedback.tone === "success"
            ? {
                message: commands.feedback.message ?? commands.feedback.title,
                onDismiss: commands.dismissFeedback,
                title: commands.feedback.title,
                tone: "success",
              }
            : {
                action: commands.feedback.action,
                impact: commands.feedback.impact,
                nextStep: commands.feedback.nextStep,
                onDismiss: commands.dismissFeedback,
                title: commands.feedback.title,
                tone: commands.feedback.tone,
              },
          {
            impact: t("feedback.unconfirmed_impact"),
          },
        )
      : hasPendingCreateIntent
        ? {
            action: {
              label: t("capability.scheduled_create_pending_review_action"),
              onClick: () => openEditor({ kind: "create", preset: null }),
            },
            impact: t("capability.scheduled_create_pending_impact"),
            title: t("capability.scheduled_create_pending_title"),
            tone: "warning",
          }
        : createRequestResolution
          ? {
              impact: t("capability.scheduled_create_checked_impact"),
              nextStep: t("capability.scheduled_create_checked_next_step"),
              onDismiss: () => setCreateRequestResolution(null),
              title: t("capability.scheduled_create_checked_title"),
              tone: "warning",
            }
          : null;
  const editingTask = dialog.kind === "edit" ? dialog.task : null;
  const authoritativeEditingTask = editingTask
    ? resource.items.find((item) => item.job_id === editingTask.job_id) ?? null
    : null;
  const editingTaskUnavailable = Boolean(editingTask && (
    (authoritativeEditingTask && isScheduledTaskDeleting(authoritativeEditingTask))
    || (!authoritativeEditingTask && resource.hasSnapshot)
  ));
  const visibleHistoryTask = historyTask
    ? resource.items.find((item) => item.job_id === historyTask.job_id)
      ?? (resource.hasSnapshot ? null : historyTask)
    : null;
  const authoritativeDeleteTarget = deleteTarget
    ? resource.items.find((item) => item.job_id === deleteTarget.job_id) ?? null
    : null;
  const deleteTargetUnavailable = Boolean(deleteTarget && (
    (authoritativeDeleteTarget && isScheduledTaskDeleting(authoritativeDeleteTarget))
    || (!authoritativeDeleteTarget && resource.hasSnapshot)
  ));
  const authoritativeDeletionStoppedTarget = deletionStoppedTarget
    ? resource.items.find((item) => item.job_id === deletionStoppedTarget.job_id) ?? null
    : null;
  const deletionStoppedTargetUnavailable = Boolean(deletionStoppedTarget && (
    !authoritativeDeletionStoppedTarget
    || authoritativeDeletionStoppedTarget.deletion_state?.trim() !== "review_required"
    || authoritativeDeletionStoppedTarget.configuration_version
      !== deletionStoppedTarget.configuration_version
  ));
  const createPreset = dialog.kind === "create" ? dialog.preset : null;
  const accessBlockedRef = useRef(accessBlocked);
  const visibleScopeKeyRef = useRef(ownerScopeKey);
  accessBlockedRef.current = accessBlocked;

  useLayoutEffect(() => {
    if (visibleScopeKeyRef.current === ownerScopeKey) {
      return;
    }
    visibleScopeKeyRef.current = ownerScopeKey;
    setHasPendingCreateIntent(Boolean(
      loadScheduledTaskCreateRequestId(ownerScopeKey),
    ));
    setCreateRequestResolution(null);
    setDialog({ kind: "closed" });
    setNextEditor(null);
    setHistoryTask(null);
    setDeleteTarget(null);
    setDeletionStoppedTarget(null);
  }, [ownerScopeKey]);

  useEffect(() => subscribeScheduledTaskCreateIntents(ownerScopeKey, () => {
    if (visibleScopeKeyRef.current !== ownerScopeKey) {
      return;
    }
    setHasPendingCreateIntent(Boolean(
      loadScheduledTaskCreateRequestId(ownerScopeKey),
    ));
  }), [ownerScopeKey]);

  useLayoutEffect(() => {
    setCommandsAccessBlocked(accessBlocked);
    if (!accessBlocked) {
      return;
    }
    setDialog({ kind: "closed" });
    setNextEditor(null);
    setHistoryTask(null);
    setDeleteTarget(null);
    setDeletionStoppedTarget(null);
  }, [accessBlocked, setCommandsAccessBlocked]);

  useEffect(() => {
    setHistoryTask((current) => {
      if (!current) {
        return null;
      }
      return resource.items.find((item) => item.job_id === current.job_id)
        ?? (resource.hasSnapshot ? null : current);
    });
  }, [resource.hasSnapshot, resource.items]);

  useLayoutEffect(() => {
    setDeleteTarget((current) => {
      if (!current) {
        return null;
      }
      const authoritative = resource.items.find(
        (item) => item.job_id === current.job_id,
      );
      if (!authoritative) {
        return resource.hasSnapshot ? null : current;
      }
      if (isScheduledTaskDeleting(authoritative)) {
        return null;
      }
      return authoritative.configuration_version === current.configuration_version
        ? current
        : authoritative;
    });
  }, [resource.hasSnapshot, resource.items]);

  useLayoutEffect(() => {
    setDeletionStoppedTarget((current) => {
      if (!current) {
        return null;
      }
      const authoritative = resource.items.find(
        (item) => item.job_id === current.job_id,
      );
      if (!authoritative) {
        return resource.hasSnapshot ? null : current;
      }
      if (
        authoritative.deletion_state?.trim() !== "review_required"
        || authoritative.configuration_version !== current.configuration_version
      ) {
        return null;
      }
      return current;
    });
  }, [resource.hasSnapshot, resource.items]);

  const editingJobId = dialog.kind === "edit" ? dialog.task.job_id : null;
  useLayoutEffect(() => {
    if (!editingJobId) {
      return;
    }
    const current = resource.items.find((item) => item.job_id === editingJobId);
    if (
      (current && isScheduledTaskDeleting(current))
      || (!current && resource.hasSnapshot)
    ) {
      setDialog({ kind: "closed" });
      setNextEditor(null);
    }
  }, [editingJobId, resource.hasSnapshot, resource.items]);

  useScheduledTaskRealtimeRefresh({
    refreshTasks: resource.refresh,
  });

  const closeDialog = useCallback(() => setDialog({ kind: "closed" }), []);
  const refreshTasks = () => {
    const refresh = accessBlockedRef.current
      ? resource.revalidateAccess()
      : commands.reconcile();
    void refresh.catch(() => undefined);
  };
  const getTaskMutationBlockReason = (task: ScheduledTaskItem): string | null => {
    if (accessBlockedRef.current) {
      return t("capability.scheduled_board_access_blocked");
    }
    const authoritativeTask = resource.items.find(
      (item) => item.job_id === task.job_id,
    );
    if (!authoritativeTask && resource.hasSnapshot) {
      return t("capability.scheduled_board_task_missing");
    }
    const currentTask = authoritativeTask ?? task;
    if (currentTask.deletion_state?.trim() === "review_required") {
      return t("capability.scheduled_board_mutation_review");
    }
    if (isScheduledTaskDeleting(currentTask)) {
      return t("capability.scheduled_board_mutation_deleting");
    }
    if (commands.isTaskMutationBlocked(currentTask.job_id)) {
      return t("capability.scheduled_board_mutation_pending");
    }
    return null;
  };
  const taskAcceptsMutations = (task: ScheduledTaskItem): boolean => (
    getTaskMutationBlockReason(task) === null
  );
  const blockedHistoryAction = (
    task: ScheduledTaskItem,
  ): ScheduledTaskRunHistoryActionResult | null => {
    const message = getTaskMutationBlockReason(task);
    return message ? { message, status: "blocked" } : null;
  };
  const isDeletionStoppedConfirmationBlocked = (jobId: string): boolean => (
    [commands.pending, commands.unconfirmed].some((state) => (
      hasScheduledTaskCommandForJob(
        state,
        "confirmDeletionStopped",
        jobId,
      )
    ))
  );
  const requestDeletionStoppedConfirmation = (task: ScheduledTaskItem) => {
    const authoritative = resource.items.find((item) => item.job_id === task.job_id);
    if (
      authoritative?.deletion_state?.trim() !== "review_required"
      || isDeletionStoppedConfirmationBlocked(task.job_id)
    ) {
      return;
    }
    setDeletionStoppedTarget(authoritative);
  };
  const confirmDeletionStopped = () => {
    const selectedTask = deletionStoppedTarget;
    const task = selectedTask
      ? resource.items.find((item) => item.job_id === selectedTask.job_id)
      : null;
    if (
      !task
      || task.deletion_state?.trim() !== "review_required"
      || task.configuration_version !== selectedTask?.configuration_version
      || isDeletionStoppedConfirmationBlocked(task.job_id)
    ) {
      setDeletionStoppedTarget(null);
      return;
    }
    setDeletionStoppedTarget(null);
    void commands.confirmDeletionStopped(task).then(() => {
      setHistoryTask((current) => (
        current?.job_id === task.job_id ? null : current
      ));
    }).catch(() => undefined);
  };
  const runTask = (task: ScheduledTaskItem) => {
    if (!taskAcceptsMutations(task)) return;
    void commands.runTask(task, () => setHistoryTask(task)).catch(() => undefined);
  };
  const toggleTask = (task: ScheduledTaskItem) => {
    if (!taskAcceptsMutations(task)) return;
    void commands.toggleTask(task).then((updatedTask) => {
      setHistoryTask((current) => (
        current?.job_id === updatedTask.job_id ? updatedTask : current
      ));
    }).catch(() => undefined);
  };
  const deleteTask = (task: ScheduledTaskItem) => {
    if (!taskAcceptsMutations(task)) return;
    setDeleteTarget(task);
  };
  const confirmDeleteTask = () => {
    const selectedTask = deleteTarget;
    const task = selectedTask
      ? resource.items.find((item) => item.job_id === selectedTask.job_id)
      : null;
    if (!task || !taskAcceptsMutations(task)) {
      setDeleteTarget(null);
      return;
    }
    setDeleteTarget(null);
    void commands.deleteTask(task).then(() => {
      setHistoryTask((current) => (
        current?.job_id === task.job_id ? null : current
      ));
    }).catch(() => undefined);
  };
  const decidePermission = (
    task: ScheduledTaskItem,
    decision: AutomationPermissionDecision,
  ) => {
    if (!taskAcceptsMutations(task)) return;
    void commands.decidePermission(task, decision).catch(() => undefined);
  };
  const resumePermissionRun = (task: ScheduledTaskItem) => {
    if (!taskAcceptsMutations(task)) return;
    void commands.resumePermissionRun(task).catch(() => undefined);
  };
  const recoverRun = async (
    task: ScheduledTaskItem,
    run: ScheduledTaskRunItem,
  ): Promise<ScheduledTaskRunHistoryActionResult> => {
    const blocked = blockedHistoryAction(task);
    if (blocked) return blocked;
    const updatedTask = await commands.recoverRun(task, run);
    setHistoryTask((current) => (
      current?.job_id === updatedTask.job_id ? updatedTask : current
    ));
    return { status: "completed" };
  };

  return (
    <>
      <WorkspaceSurfaceScaffold>
        <CapabilityPageLayout
          actions={(
            <div className="flex items-center gap-2">
              <UiButton
                disabled={resource.isLoading}
                onClick={refreshTasks}
                size="2xs"
                variant="text"
              >
                <RefreshCw className={resource.isLoading
                  ? getUiSpinnerClassName({ size: "sm" })
                  : "h-3.5 w-3.5"}
                />
                {t("capability.refresh")}
              </UiButton>
              <UiButton
                disabled={editorBusy || scopeUnavailable
                  || accessBlocked
                  || (resource.isLoading && !resource.hasSnapshot)}
                onClick={() => {
                  if (!scopeUnavailable && !accessBlockedRef.current) {
                    openEditor({ kind: "create", preset: null });
                  }
                }}
                size="2xs"
                tone="primary"
                variant="text"
              >
                <Plus className="h-3.5 w-3.5" />
                {t("capability.create_task")}
              </UiButton>
            </div>
          )}
          description={t("capability.scheduled_intro_description")}
          className="flex h-full min-h-0 flex-col"
          title={t("capability.scheduled_intro_title")}
        >
          <div className="flex min-h-0 flex-1 overflow-hidden">
          <div className={dialog.kind === "closed" ? "flex min-h-0 min-w-0 flex-1 overflow-hidden" : "hidden min-h-0 min-w-0 flex-1 overflow-hidden lg:flex"}>
          <ScheduledTaskBoard
            failure={resource.failure}
            hasSnapshot={resource.hasSnapshot}
            isLoading={resource.isLoading}
            isPermissionLoading={resource.isPermissionLoading}
            items={resource.items}
            onCreate={() => {
              if (!scopeUnavailable && !accessBlockedRef.current) {
                openEditor({ kind: "create", preset: null });
              }
            }}
            onCreateFromPreset={(preset) => {
              if (!scopeUnavailable && !accessBlockedRef.current) {
                openEditor({ kind: "create", preset });
              }
            }}
            onConfirmDeletionStopped={requestDeletionStoppedConfirmation}
            onDelete={deleteTask}
            onEdit={(task) => {
              if (taskAcceptsMutations(task)) {
                openEditor({ kind: "edit", task });
              }
            }}
            onOpenHistory={(task) => {
              if (!accessBlockedRef.current) {
                setHistoryTask(task);
              }
            }}
            onOpenConnector={(connectorId) => navigate(AppRouteBuilders.connectorDetail(connectorId))}
            onPermissionDecision={decidePermission}
            onPermissionResume={resumePermissionRun}
            onRefresh={refreshTasks}
            onRunNow={runTask}
            onToggleEnabled={toggleTask}
            pending={commands.pending}
            permissionFailure={resource.permissionFailure}
            unconfirmed={commands.unconfirmed}
          />
          </div>
          <aside aria-label={t("capability.scheduled_editor")}
            className={dialog.kind === "closed" ? "hidden" : "relative min-h-0 w-full shrink-0 bg-(--background) border-l border-(--divider-subtle-color) lg:ml-4 lg:w-[55%] lg:min-w-[420px] lg:max-w-[760px]"}>
      <ScheduledTaskDialog
        onBusyChange={setEditorBusy}
        agentId={resource.agentId}
        createPreset={createPreset}
        initialTask={editingTask}
        isOpen={!scopeUnavailable
          && !accessBlocked
          && !editingTaskUnavailable
          && dialog.kind !== "closed"}
        onAccessFailure={invalidateMutationAccess}
        onClose={closeDialog}
        onCreated={commands.acceptCreatedTask}
        onCreateIntentResolved={handleCreateIntentResolved}
        onConfirmMutationReviewed={commands.confirmReviewedMutation}
        onIsMutationBlocked={commands.isTaskMutationBlocked}
        onReconcile={commands.reconcile}
        onSaved={commands.acceptSavedTask}
        scopeKey={ownerScopeKey}
      />
          </aside>
          </div>
        </CapabilityPageLayout>
      </WorkspaceSurfaceScaffold>

      <ConfirmDialog
        isOpen={nextEditor !== null && !editorBusy && !scopeUnavailable && !accessBlocked}
        title={t("capability.scheduled_discard_title")}
        message={t("capability.scheduled_discard_message")}
        confirmText={t("capability.scheduled_continue")}
        onCancel={() => setNextEditor(null)}
        onConfirm={() => {
          if (nextEditor && !editorBusy) setDialog(nextEditor);
          setNextEditor(null);
        }}
      />
      <ScheduledTaskRunHistoryDialog
        isOpen={!scopeUnavailable && !accessBlocked && visibleHistoryTask !== null}
        onClose={() => setHistoryTask(null)}
        onRecoverTaskRun={recoverRun}
        onRetryDelivery={async (task, run, reconcileHistory, options) => {
          const blocked = blockedHistoryAction(task);
          if (blocked) return blocked;
          await commands.retryDelivery(task, run, reconcileHistory, options);
          return { status: "completed" };
        }}
        onRunHistoryReconciled={commands.confirmRunHistoryReconciled}
        onRetryTask={async (task, reconcileHistory) => {
          const blocked = blockedHistoryAction(task);
          if (blocked) return blocked;
          await commands.runTask(task, () => {
            void reconcileHistory().catch(() => undefined);
          });
          return { status: "completed" };
        }}
        scopeKey={ownerScopeKey}
        task={visibleHistoryTask}
        unconfirmed={commands.unconfirmed}
      />
      <ConfirmDialog
        confirmText={t("capability.scheduled_board_confirm_delete")}
        isOpen={!scopeUnavailable
          && !accessBlocked
          && !deletionStoppedTargetUnavailable
          && deletionStoppedTarget !== null}
        message={t("capability.scheduled_board_confirm_delete_message")}
        onCancel={() => setDeletionStoppedTarget(null)}
        onConfirm={confirmDeletionStopped}
        title={t("capability.scheduled_board_confirm_delete_title")}
        variant="danger"
      />
      <ConfirmDialog
        confirmText={t("capability.scheduled_board_delete")}
        isOpen={!scopeUnavailable
          && !accessBlocked
          && !deleteTargetUnavailable
          && deleteTarget !== null}
        message={!accessBlocked && !deleteTargetUnavailable && deleteTarget
          ? t("capability.scheduled_board_delete_message", { name: deleteTarget.name })
          : ""}
        onCancel={() => setDeleteTarget(null)}
        onConfirm={confirmDeleteTask}
        title={t("capability.scheduled_card_delete")}
        variant="danger"
      />

      <FeedbackBannerViewport item={feedbackItem} />
    </>
  );
}
