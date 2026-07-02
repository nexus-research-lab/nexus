"use client";

import { useState } from "react";
import { CalendarClock, Plus, RefreshCw } from "lucide-react";

import { useAutomationController } from "@/hooks/capability/use-automation-controller";
import {
  delete_scheduled_task_api,
  recover_scheduled_task_run_api,
  retry_scheduled_task_run_delivery_api,
  run_scheduled_task_api,
} from "@/lib/api/scheduled-task-api";
import { useI18n } from "@/shared/i18n/i18n-context";
import {
  WorkspaceSurfaceHeader,
  WorkspaceSurfaceToolbarAction,
} from "@/shared/ui/workspace/surface/workspace-surface-header";
import { WorkspaceSurfaceScaffold } from "@/shared/ui/workspace/surface/workspace-surface-scaffold";
import type { ScheduledTaskItem, ScheduledTaskRunItem } from "@/types/capability/scheduled-task";
import {
  CapabilityPageLayout,
  CapabilitySectionHeader,
} from "@/features/capability/shared/capability-page-layout";

import { FeedbackBannerStack } from "@/shared/ui/feedback/feedback-banner-stack";
import { notify_scheduled_tasks_mutated } from "../scheduled-task-events";
import { ScheduledTaskDialog } from "./dialog/scheduled-task-dialog";
import { ScheduledTaskList } from "./scheduled-task-list";
import { ScheduledTaskRunHistoryDialog } from "./scheduled-task-run-history-dialog";
import { useScheduledTaskRealtimeRefresh } from "./use-scheduled-task-realtime-refresh";

interface FeedbackState {
  tone: "success" | "warning" | "error";
  title: string;
  message: string;
}

interface ScheduledMetricItemProps {
  description: string;
  label: string;
  value: number;
}

function ScheduledMetricItem({ description, label, value }: ScheduledMetricItemProps) {
  return (
    <div className="min-w-0 py-3 md:px-4 md:first:pl-0 md:last:pr-0">
      <p className="text-[11px] font-semibold uppercase tracking-[0.16em] text-(--text-muted)">
        {label}
      </p>
      <div className="mt-1.5 flex items-baseline gap-2">
        <p className="text-[28px] font-semibold tracking-[-0.04em] text-(--text-strong)">
          {value}
        </p>
        <p className="min-w-0 truncate text-[12px] leading-5 text-(--text-muted)">
          {description}
        </p>
      </div>
    </div>
  );
}

async function refreshTasksBestEffort(
  automation: ReturnType<typeof useAutomationController>,
  agentId: string,
  successFeedback: Omit<FeedbackState, "tone">,
  refreshWarningMessage: string,
  setFeedback: (feedback: FeedbackState) => void,
) {
  try {
    await automation.refresh_tasks();
    notify_scheduled_tasks_mutated(agentId);
    setFeedback({ tone: "success", ...successFeedback });
  } catch (error) {
    notify_scheduled_tasks_mutated(agentId);
    setFeedback({
      tone: "warning",
      title: successFeedback.title,
      message: `${successFeedback.message}；${refreshWarningMessage}${error instanceof Error ? `（${error.message}）` : ""}`,
    });
  }
}

export function ScheduledTasksDirectory() {
  const { t } = useI18n();
  const [isDialogOpen, setIsDialogOpen] = useState(false);
  const [editingTask, setEditingTask] = useState<ScheduledTaskItem | null>(null);
  const [historyTask, setHistoryTask] = useState<ScheduledTaskItem | null>(null);
  const [feedback, setFeedback] = useState<FeedbackState | null>(null);
  const [runPendingJobId, setRunPendingJobId] = useState<string | null>(null);
  const [togglePendingJobId, setTogglePendingJobId] = useState<string | null>(null);
  const [deletePendingJobId, setDeletePendingJobId] = useState<string | null>(null);
  const automation = useAutomationController({ include_all_tasks: true });
  const refreshTasks = automation.refresh_tasks;
  const refreshAll = automation.refresh_all;
  const runningCount = automation.scheduled_tasks.filter((task) => task.running).length;
  const enabledCount = automation.scheduled_tasks.filter((task) => task.enabled).length;
  const pausedCount = automation.scheduled_tasks.length - enabledCount;
  const feedbackItems = feedback
    ? [
        {
          key: "feedback",
          message: feedback.message,
          on_dismiss: () => setFeedback(null),
          title: feedback.title,
          tone: feedback.tone,
        },
      ]
    : [];

  useScheduledTaskRealtimeRefresh({ enabled_count: enabledCount, refresh_tasks: refreshTasks, running_count: runningCount });

  const handleCreateSuccess = async (task: ScheduledTaskItem) => {
    await refreshTasksBestEffort(
      automation,
      task.agent_id,
      {
        title: "任务已创建",
        message: `${task.name} 已加入自动化任务列表`,
      },
      "任务列表刷新失败，稍后会自动同步",
      setFeedback,
    );
  };

  const handleRefreshAll = async () => {
    try {
      await refreshAll();
    } catch (error) {
      setFeedback({
        tone: "error",
        title: "刷新失败",
        message: error instanceof Error ? error.message : "刷新自动化数据失败",
      });
    }
  };

  const handleSaveSuccess = async (task: ScheduledTaskItem) => {
    await refreshTasksBestEffort(
      automation,
      task.agent_id,
      {
        title: "任务已更新",
        message: `${task.name} 的配置已保存`,
      },
      "任务列表刷新失败，稍后会自动同步",
      setFeedback,
    );
  };

  const handleRunNow = async (task: ScheduledTaskItem) => {
    setRunPendingJobId(task.job_id);
    try {
      const result = await run_scheduled_task_api(task.job_id);
      await refreshTasksBestEffort(
        automation,
        automation.agent_id,
        {
          title: "任务已触发",
          message: result.status === "queued_to_main_session"
            ? `${task.name} 已排入主会话执行`
            : `${task.name} 已开始执行`,
        },
        "任务列表刷新失败，运行状态稍后会同步",
        setFeedback,
      );
    } catch (error) {
      setFeedback({
        tone: "error",
        title: "任务执行失败",
        message: error instanceof Error ? error.message : "立即运行失败",
      });
    } finally {
      setRunPendingJobId(null);
    }
  };

  const handleToggleEnabled = async (task: ScheduledTaskItem) => {
    setTogglePendingJobId(task.job_id);
    try {
      const updatedTask = await automation.toggle_task(task);
      setHistoryTask((currentTask) => {
        if (!currentTask || currentTask.job_id !== updatedTask.job_id) {
          return currentTask;
        }
        return updatedTask;
      });
      await refreshTasksBestEffort(
        automation,
        updatedTask.agent_id,
        {
          title: updatedTask.enabled ? "任务已启用" : "任务已暂停",
          message: updatedTask.enabled
            ? `${updatedTask.name} 已恢复自动调度`
            : `${updatedTask.name} 不再参与后续调度`,
        },
        "任务列表刷新失败，状态稍后会同步",
        setFeedback,
      );
    } catch (error) {
      setFeedback({
        tone: "error",
        title: "状态更新失败",
        message: error instanceof Error ? error.message : "切换任务状态失败",
      });
    } finally {
      setTogglePendingJobId(null);
    }
  };

  const handleRecoverTaskRun = async (task: ScheduledTaskItem, run: ScheduledTaskRunItem) => {
    try {
      const updatedTask = await recover_scheduled_task_run_api(task.job_id, { run_id: run.run_id });
      setHistoryTask((current) => current?.job_id === updatedTask.job_id ? updatedTask : current);
      await refreshTasksBestEffort(
        automation,
        automation.agent_id,
        {
          title: "运行占用已释放",
          message: `${task.name} 的当前 run 已标记为 cancelled`,
        },
        "任务列表刷新失败，运行状态稍后会同步",
        setFeedback,
      );
    } catch (error) {
      setFeedback({
        tone: "error",
        title: "释放运行占用失败",
        message: error instanceof Error ? error.message : "释放运行占用失败",
      });
      throw error;
    }
  };

  const handleRetryDelivery = async (task: ScheduledTaskItem, run: ScheduledTaskRunItem) => {
    try {
      const updatedRun = await retry_scheduled_task_run_delivery_api(task.job_id, run.run_id);
      await refreshTasksBestEffort(
        automation,
        automation.agent_id,
        {
          title: updatedRun.delivery_status === "succeeded" ? "投递已恢复" : "投递已重试",
          message: updatedRun.delivery_status === "succeeded"
            ? `${task.name} 的运行结果已重新投递`
            : `${task.name} 的投递状态已更新为 ${updatedRun.delivery_status ?? "unknown"}`,
        },
        "任务列表刷新失败，投递状态稍后会同步",
        setFeedback,
      );
    } catch (error) {
      setFeedback({
        tone: "error",
        title: "重试投递失败",
        message: error instanceof Error ? error.message : "重试投递失败",
      });
      throw error;
    }
  };

  const handleDelete = async (task: ScheduledTaskItem) => {
    if (!window.confirm(`确认删除任务“${task.name}”吗？`)) {
      return;
    }
    setDeletePendingJobId(task.job_id);
    try {
      await delete_scheduled_task_api(task.job_id);
      await refreshTasksBestEffort(
        automation,
        automation.agent_id,
        {
          title: "任务已删除",
          message: `${task.name} 已从自动化任务列表移除`,
        },
        "任务列表刷新失败，删除结果稍后会同步",
        setFeedback,
      );
    } catch (error) {
      setFeedback({
        tone: "error",
        title: "删除失败",
        message: error instanceof Error ? error.message : "删除任务失败",
      });
    } finally {
      setDeletePendingJobId(null);
    }
  };

  return (
    <>
      <WorkspaceSurfaceScaffold
        body_scrollable
        header={(
          <WorkspaceSurfaceHeader
            badge={t("capability.scheduled_badge", { count: automation.scheduled_tasks.length })}
            density="compact"
            leading={<CalendarClock className="h-4 w-4" />}
            subtitle={t("capability.scheduled_subtitle")}
            title={t("capability.scheduled")}
            trailing={(
              <>
                <WorkspaceSurfaceToolbarAction onClick={() => void handleRefreshAll()}>
                  <RefreshCw className="h-3.5 w-3.5" />
                  {t("capability.refresh_all")}
                </WorkspaceSurfaceToolbarAction>
                <WorkspaceSurfaceToolbarAction onClick={() => setIsDialogOpen(true)} tone="primary">
                  <Plus className="h-3.5 w-3.5" />
                  {t("capability.create_task")}
                </WorkspaceSurfaceToolbarAction>
              </>
            )}
          />
        )}
        stable_gutter
      >
        <CapabilityPageLayout
          description={t("capability.scheduled_intro_description")}
          title={t("capability.scheduled_intro_title")}
        >
          <CapabilitySectionHeader title={t("capability.scheduled_overview_title")} />
          <section className="mb-7 grid gap-0 divide-y divide-(--divider-subtle-color) border-b border-(--divider-subtle-color) pb-2 md:grid-cols-3 md:divide-x md:divide-y-0">
            <ScheduledMetricItem
              description="当前占用执行会话"
              label="执行中"
              value={runningCount}
            />
            <ScheduledMetricItem
              description="后续继续参与调度"
              label="已启用"
              value={enabledCount}
            />
            <ScheduledMetricItem
              description="暂时不会自动触发"
              label="已暂停"
              value={pausedCount}
            />
          </section>

          <ScheduledTaskList
            error_message={automation.tasks_error}
            is_loading={automation.tasks_loading}
            items={automation.scheduled_tasks}
            on_create={() => setIsDialogOpen(true)}
            on_open_history={setHistoryTask}
            on_refresh={() => void refreshTasks().catch((err: unknown) => console.debug("[scheduled-tasks] Manual refresh failed:", err))}
            on_run_now={(task) => void handleRunNow(task)}
            on_toggle_enabled={(task) => void handleToggleEnabled(task)}
            on_delete={(task) => void handleDelete(task)}
            on_edit={setEditingTask}
            delete_pending_job_id={deletePendingJobId}
            run_pending_job_id={runPendingJobId}
            toggle_pending_job_id={togglePendingJobId}
          />
        </CapabilityPageLayout>
      </WorkspaceSurfaceScaffold>

      <ScheduledTaskDialog
        agent_id={automation.agent_id}
        initial_task={editingTask}
        is_open={isDialogOpen || editingTask !== null}
        on_close={() => {
          setIsDialogOpen(false);
          setEditingTask(null);
        }}
        on_created={(task) => void handleCreateSuccess(task)}
        on_saved={(task) => void handleSaveSuccess(task)}
      />
      <ScheduledTaskRunHistoryDialog
        is_open={historyTask !== null}
        on_close={() => setHistoryTask(null)}
        on_recover_task_run={(task, run) => handleRecoverTaskRun(task, run)}
        on_retry_delivery={(task, run) => handleRetryDelivery(task, run)}
        on_retry_task={(task) => handleRunNow(task)}
        task={historyTask}
      />

      <FeedbackBannerStack items={feedbackItems} />
    </>
  );
}
