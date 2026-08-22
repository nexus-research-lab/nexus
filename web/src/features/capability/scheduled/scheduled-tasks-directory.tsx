/**
 * INPUT: 定时任务资源、写命令与页面导航。
 * OUTPUT: 定时任务目录、编辑/历史工作面和产品内删除确认。
 * POS: Scheduled 页面装配边界；不使用浏览器原生确认框。
 */
"use client";

import { useState } from "react";
import { Plus, RefreshCw } from "lucide-react";
import { useNavigate } from "react-router-dom";

import { AppRouteBuilders } from "@/app/router/route-paths";
import { CapabilityPageLayout } from "@/features/capability/shared/capability-page-layout";
import { useI18n } from "@/shared/i18n/i18n-context";
import { ConfirmDialog } from "@/shared/ui/dialog/decision/decision-dialog";
import {
  type FeedbackBannerProps,
} from "@/shared/ui/feedback/feedback-banner";
import { FeedbackBannerViewport } from "@/shared/ui/feedback/feedback-banner-viewport";
import { WorkspaceSurfaceToolbarAction } from "@/shared/ui/workspace/surface/workspace-surface-toolbar-action";
import { WorkspaceSurfaceScaffold } from "@/shared/ui/workspace/surface/workspace-surface-scaffold";
import type { ScheduledTaskRunItem } from "@/types/capability/scheduled-task/run";
import type { ScheduledTaskItem } from "@/types/capability/scheduled-task/task";
import type { AutomationPermissionDecision } from "@/types/capability/scheduled-task/permission";

import { ScheduledTaskBoard } from "./board/scheduled-task-board";
import { useScheduledTaskCommands } from "./controller/use-scheduled-task-commands";
import { useScheduledTasksResource } from "./controller/use-scheduled-tasks-resource";
import { ScheduledTaskDialog } from "./dialog/scheduled-task-dialog";
import type { TaskDialogCreatePreset } from "./dialog/scheduled-task-dialog-types";
import { ScheduledTaskRunHistoryDialog } from "./history/scheduled-task-run-history-dialog";
import { useScheduledTaskRealtimeRefresh } from "./use-scheduled-task-realtime-refresh";

type TaskDialogState =
  | { kind: "closed" }
  | { kind: "create"; preset: TaskDialogCreatePreset | null }
  | { kind: "edit"; task: ScheduledTaskItem };

export function ScheduledTasksDirectory() {
  const { t } = useI18n();
  const navigate = useNavigate();
  const [dialog, setDialog] = useState<TaskDialogState>({ kind: "closed" });
  const [historyTask, setHistoryTask] = useState<ScheduledTaskItem | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<ScheduledTaskItem | null>(null);
  const resource = useScheduledTasksResource();
  const commands = useScheduledTaskCommands({
    refresh: resource.refresh,
    removeTask: resource.removeTask,
    upsertTask: resource.upsertTask,
  });
  const feedbackItem: FeedbackBannerProps | null = commands.feedback
    ? {
        ...commands.feedback,
        onDismiss: commands.dismissFeedback,
      }
    : null;
  const editingTask = dialog.kind === "edit" ? dialog.task : null;
  const createPreset = dialog.kind === "create" ? dialog.preset : null;

  useScheduledTaskRealtimeRefresh({
    refreshTasks: resource.refresh,
  });

  const closeDialog = () => setDialog({ kind: "closed" });
  const refreshTasks = () => {
    void resource.refresh().catch(() => undefined);
  };
  const runTask = (task: ScheduledTaskItem) => {
    void commands.runTask(task).catch(() => undefined);
  };
  const toggleTask = (task: ScheduledTaskItem) => {
    void commands.toggleTask(task).then((updatedTask) => {
      setHistoryTask((current) => (
        current?.job_id === updatedTask.job_id ? updatedTask : current
      ));
    }).catch(() => undefined);
  };
  const deleteTask = (task: ScheduledTaskItem) => setDeleteTarget(task);
  const confirmDeleteTask = () => {
    const task = deleteTarget;
    if (!task) return;
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
    void commands.decidePermission(task, decision).catch(() => undefined);
  };
  const resumePermissionRun = (task: ScheduledTaskItem) => {
    void commands.resumePermissionRun(task).catch(() => undefined);
  };
  const recoverRun = async (
    task: ScheduledTaskItem,
    run: ScheduledTaskRunItem,
  ): Promise<void> => {
    const updatedTask = await commands.recoverRun(task, run);
    setHistoryTask((current) => (
      current?.job_id === updatedTask.job_id ? updatedTask : current
    ));
  };

  return (
    <>
      <WorkspaceSurfaceScaffold>
        <CapabilityPageLayout
          actions={(
            <div className="flex items-center gap-2">
              <WorkspaceSurfaceToolbarAction onClick={refreshTasks}>
                <RefreshCw className="h-3.5 w-3.5" />
                {t("capability.refresh")}
              </WorkspaceSurfaceToolbarAction>
              <WorkspaceSurfaceToolbarAction
                onClick={() => setDialog({ kind: "create", preset: null })}
                tone="primary"
              >
                <Plus className="h-3.5 w-3.5" />
                {t("capability.create_task")}
              </WorkspaceSurfaceToolbarAction>
            </div>
          )}
          description={t("capability.scheduled_intro_description")}
          className="flex h-full min-h-0 flex-col"
          title={t("capability.scheduled_intro_title")}
        >
          <ScheduledTaskBoard
            errorMessage={resource.errorMessage}
            isLoading={resource.isLoading}
            items={resource.items}
            onCreate={() => setDialog({ kind: "create", preset: null })}
            onCreateFromPreset={(preset) => setDialog({ kind: "create", preset })}
            onDelete={deleteTask}
            onEdit={(task) => setDialog({ kind: "edit", task })}
            onOpenHistory={setHistoryTask}
            onOpenConnector={(connectorId) => navigate(AppRouteBuilders.connectorDetail(connectorId))}
            onPermissionDecision={decidePermission}
            onPermissionResume={resumePermissionRun}
            onRefresh={refreshTasks}
            onRunNow={runTask}
            onToggleEnabled={toggleTask}
            pending={commands.pending}
          />
        </CapabilityPageLayout>
      </WorkspaceSurfaceScaffold>

      <ScheduledTaskDialog
        agentId={resource.agentId}
        createPreset={createPreset}
        initialTask={editingTask}
        isOpen={dialog.kind !== "closed"}
        onClose={closeDialog}
        onCreated={commands.acceptCreatedTask}
        onSaved={commands.acceptSavedTask}
      />
      <ScheduledTaskRunHistoryDialog
        isOpen={historyTask !== null}
        onClose={() => setHistoryTask(null)}
        onRecoverTaskRun={recoverRun}
        onRetryDelivery={commands.retryDelivery}
        onRetryTask={async (task) => {
          await commands.runTask(task);
        }}
        task={historyTask}
      />
      <ConfirmDialog
        confirmText="删除"
        isOpen={deleteTarget !== null}
        message={deleteTarget ? `删除“${deleteTarget.name}”后，这个任务将不再运行。` : ""}
        onCancel={() => setDeleteTarget(null)}
        onConfirm={confirmDeleteTask}
        title="删除任务"
        variant="danger"
      />

      <FeedbackBannerViewport item={feedbackItem} />
    </>
  );
}
