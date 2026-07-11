"use client";

import { useState } from "react";
import { CalendarClock, Plus, RefreshCw } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import {
  FeedbackBannerStack,
  type FeedbackBannerItem,
} from "@/shared/ui/feedback/feedback-banner-stack";
import { WorkspaceSurfaceHeader } from "@/shared/ui/workspace/surface/workspace-surface-header";
import { WorkspaceSurfaceToolbarAction } from "@/shared/ui/workspace/surface/workspace-surface-toolbar-action";
import { WorkspaceSurfaceScaffold } from "@/shared/ui/workspace/surface/workspace-surface-scaffold";
import type { ScheduledTaskRunItem } from "@/types/capability/scheduled-task/run";
import type { ScheduledTaskItem } from "@/types/capability/scheduled-task/task";

import {
  CapabilityPageLayout,
  CapabilitySectionHeader,
} from "../shared/capability-page-layout";
import { getScheduledTaskMetrics } from "./controller/scheduled-task-directory-model";
import { useScheduledTaskCommands } from "./controller/use-scheduled-task-commands";
import { useScheduledTasksResource } from "./controller/use-scheduled-tasks-resource";
import { ScheduledTaskDialog } from "./dialog/scheduled-task-dialog";
import { ScheduledTaskRunHistoryDialog } from "./history/scheduled-task-run-history-dialog";
import { ScheduledTaskList } from "./list/scheduled-task-list";
import { ScheduledTaskOverview } from "./scheduled-task-overview";
import { useScheduledTaskRealtimeRefresh } from "./use-scheduled-task-realtime-refresh";

type TaskDialogState =
  | { kind: "closed" }
  | { kind: "create" }
  | { kind: "edit"; task: ScheduledTaskItem };

export function ScheduledTasksDirectory() {
  const { t } = useI18n();
  const [dialog, setDialog] = useState<TaskDialogState>({ kind: "closed" });
  const [historyTask, setHistoryTask] = useState<ScheduledTaskItem | null>(null);
  const resource = useScheduledTasksResource();
  const commands = useScheduledTaskCommands({
    refresh: resource.refresh,
    removeTask: resource.removeTask,
    upsertTask: resource.upsertTask,
  });
  const metrics = getScheduledTaskMetrics(resource.items);
  const feedbackItems: FeedbackBannerItem[] = commands.feedback
    ? [{
        ...commands.feedback,
        key: "scheduled-task-feedback",
        onDismiss: commands.dismissFeedback,
      }]
    : [];
  const editingTask = dialog.kind === "edit" ? dialog.task : null;

  useScheduledTaskRealtimeRefresh({
    enabledCount: metrics.enabled,
    refreshTasks: resource.refresh,
    runningCount: metrics.running,
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
  const deleteTask = (task: ScheduledTaskItem) => {
    if (!window.confirm(`确认删除任务“${task.name}”吗？`)) {
      return;
    }
    void commands.deleteTask(task).then(() => {
      setHistoryTask((current) => (
        current?.job_id === task.job_id ? null : current
      ));
    }).catch(() => undefined);
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
      <WorkspaceSurfaceScaffold
        bodyScrollable
        header={(
          <WorkspaceSurfaceHeader
            badge={t("capability.scheduled_badge", { count: resource.items.length })}
            leading={<CalendarClock className="h-4 w-4" />}
            subtitle={t("capability.scheduled_subtitle")}
            title={t("capability.scheduled")}
            trailing={(
              <>
                <WorkspaceSurfaceToolbarAction onClick={refreshTasks}>
                  <RefreshCw className="h-3.5 w-3.5" />
                  {t("capability.refresh_all")}
                </WorkspaceSurfaceToolbarAction>
                <WorkspaceSurfaceToolbarAction
                  onClick={() => setDialog({ kind: "create" })}
                  tone="primary"
                >
                  <Plus className="h-3.5 w-3.5" />
                  {t("capability.create_task")}
                </WorkspaceSurfaceToolbarAction>
              </>
            )}
          />
        )}
        stableGutter
      >
        <CapabilityPageLayout
          description={t("capability.scheduled_intro_description")}
          title={t("capability.scheduled_intro_title")}
        >
          <CapabilitySectionHeader title={t("capability.scheduled_overview_title")} />
          <ScheduledTaskOverview {...metrics} />
          <ScheduledTaskList
            errorMessage={resource.errorMessage}
            isLoading={resource.isLoading}
            items={resource.items}
            onCreate={() => setDialog({ kind: "create" })}
            onDelete={deleteTask}
            onEdit={(task) => setDialog({ kind: "edit", task })}
            onOpenHistory={setHistoryTask}
            onRefresh={refreshTasks}
            onRunNow={runTask}
            onToggleEnabled={toggleTask}
            pending={commands.pending}
          />
        </CapabilityPageLayout>
      </WorkspaceSurfaceScaffold>

      <ScheduledTaskDialog
        agentId={resource.agentId}
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

      <FeedbackBannerStack items={feedbackItems} />
    </>
  );
}
