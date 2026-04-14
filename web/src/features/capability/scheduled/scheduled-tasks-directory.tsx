"use client";

import { useState } from "react";
import { CalendarClock, Plus, RefreshCw } from "lucide-react";

import { useAutomationController } from "@/hooks/use-automation-controller";
import { deleteScheduledTaskApi, runScheduledTaskApi, updateScheduledTaskStatusApi } from "@/lib/scheduled-task-api";
import {
  WorkspaceSurfaceHeader,
  WorkspaceSurfaceToolbarAction,
} from "@/shared/ui/workspace/workspace-surface-header";
import { WorkspaceSurfaceScaffold } from "@/shared/ui/workspace/workspace-surface-scaffold";
import type { ScheduledTaskItem } from "@/types/scheduled-task";

import { FeedbackBanner } from "../skills/feedback-banner";
import { HeartbeatSettingsCard } from "./heartbeat-settings-card";
import { ScheduledTaskDialog } from "./scheduled-task-dialog";
import { ScheduledTaskList } from "./scheduled-task-list";
import { ScheduledTaskRunHistoryDialog } from "./scheduled-task-run-history-dialog";

interface FeedbackState {
  tone: "success" | "warning" | "error";
  title: string;
  message: string;
}

const SCHEDULED_TASKS_MUTATED_EVENT = "nexus:scheduled-tasks-mutated";

function notify_scheduled_tasks_mutated(agent_id: string) {
  if (typeof window === "undefined") {
    return;
  }
  window.dispatchEvent(new CustomEvent(SCHEDULED_TASKS_MUTATED_EVENT, { detail: { agent_id } }));
}

async function refresh_tasks_best_effort(
  automation: ReturnType<typeof useAutomationController>,
  agent_id: string,
  success_feedback: Omit<FeedbackState, "tone">,
  refresh_warning_message: string,
  set_feedback: (feedback: FeedbackState) => void,
) {
  try {
    await automation.refresh_tasks();
    notify_scheduled_tasks_mutated(agent_id);
    set_feedback({ tone: "success", ...success_feedback });
  } catch (error) {
    notify_scheduled_tasks_mutated(agent_id);
    set_feedback({
      tone: "warning",
      title: success_feedback.title,
      message: `${success_feedback.message}；${refresh_warning_message}${error instanceof Error ? `（${error.message}）` : ""}`,
    });
  }
}

export function ScheduledTasksDirectory() {
  const [is_dialog_open, set_is_dialog_open] = useState(false);
  const [history_task, set_history_task] = useState<ScheduledTaskItem | null>(null);
  const [feedback, set_feedback] = useState<FeedbackState | null>(null);
  const [wake_pending, set_wake_pending] = useState(false);
  const [run_pending_job_id, set_run_pending_job_id] = useState<string | null>(null);
  const [toggle_pending_job_id, set_toggle_pending_job_id] = useState<string | null>(null);
  const [delete_pending_job_id, set_delete_pending_job_id] = useState<string | null>(null);
  const automation = useAutomationController();
  const running_count = automation.scheduled_tasks.filter((task) => task.running).length;
  const enabled_count = automation.scheduled_tasks.filter((task) => task.enabled).length;
  const paused_count = automation.scheduled_tasks.length - enabled_count;

  const handle_create_success = (task: ScheduledTaskItem) => {
    void refresh_tasks_best_effort(
      automation,
      task.agent_id,
      {
        title: "任务已创建",
        message: `${task.name} 已加入自动化任务列表`,
      },
      "任务列表刷新失败，稍后会自动同步",
      set_feedback,
    );
  };

  const handle_refresh_all = async () => {
    try {
      await automation.refresh_all();
    } catch (error) {
      set_feedback({
        tone: "error",
        title: "刷新失败",
        message: error instanceof Error ? error.message : "刷新自动化数据失败",
      });
    }
  };

  const handle_wake = async () => {
    set_wake_pending(true);
    try {
      const result = await automation.wake_heartbeat();
      set_feedback({
        tone: "success",
        title: "Heartbeat 已触发",
        message: result.scheduled ? "已加入 heartbeat 执行队列" : "唤醒请求已发送",
      });
    } catch (error) {
      set_feedback({
        tone: "error",
        title: "Heartbeat 触发失败",
        message: error instanceof Error ? error.message : "唤醒请求失败",
      });
    } finally {
      set_wake_pending(false);
    }
  };

  const handle_run_now = async (task: ScheduledTaskItem) => {
    set_run_pending_job_id(task.job_id);
    try {
      const result = await runScheduledTaskApi(task.job_id);
      await refresh_tasks_best_effort(
        automation,
        automation.agent_id,
        {
          title: "任务已触发",
          message: result.status === "queued_to_main_session"
            ? `${task.name} 已排入主会话执行`
            : `${task.name} 已开始执行`,
        },
        "任务列表刷新失败，运行状态稍后会同步",
        set_feedback,
      );
    } catch (error) {
      set_feedback({
        tone: "error",
        title: "任务执行失败",
        message: error instanceof Error ? error.message : "立即运行失败",
      });
    } finally {
      set_run_pending_job_id(null);
    }
  };

  const handle_toggle_enabled = async (task: ScheduledTaskItem) => {
    set_toggle_pending_job_id(task.job_id);
    try {
      await updateScheduledTaskStatusApi(task.job_id, { enabled: !task.enabled });
      await refresh_tasks_best_effort(
        automation,
        automation.agent_id,
        {
          title: task.enabled ? "任务已暂停" : "任务已启用",
          message: task.enabled
            ? `${task.name} 不再参与后续调度`
            : `${task.name} 已恢复自动调度`,
        },
        "任务列表刷新失败，状态稍后会同步",
        set_feedback,
      );
    } catch (error) {
      set_feedback({
        tone: "error",
        title: "状态更新失败",
        message: error instanceof Error ? error.message : "切换任务状态失败",
      });
    } finally {
      set_toggle_pending_job_id(null);
    }
  };

  const handle_delete = async (task: ScheduledTaskItem) => {
    if (!window.confirm(`确认删除任务“${task.name}”吗？`)) {
      return;
    }
    set_delete_pending_job_id(task.job_id);
    try {
      await deleteScheduledTaskApi(task.job_id);
      await refresh_tasks_best_effort(
        automation,
        automation.agent_id,
        {
          title: "任务已删除",
          message: `${task.name} 已从自动化任务列表移除`,
        },
        "任务列表刷新失败，删除结果稍后会同步",
        set_feedback,
      );
    } catch (error) {
      set_feedback({
        tone: "error",
        title: "删除失败",
        message: error instanceof Error ? error.message : "删除任务失败",
      });
    } finally {
      set_delete_pending_job_id(null);
    }
  };

  return (
    <>
      <WorkspaceSurfaceScaffold
        body_class_name="px-5 py-5 xl:px-6"
        body_scrollable
        header={(
          <WorkspaceSurfaceHeader
            badge={`${automation.scheduled_tasks.length} 个任务`}
            density="compact"
            leading={<CalendarClock className="h-4 w-4" />}
            title="自动化看板"
            title_trailing={(
              <span className="truncate text-[11px] font-medium text-(--text-default)">
                Agent {automation.agent_id}
              </span>
            )}
            trailing={(
              <>
                <WorkspaceSurfaceToolbarAction onClick={() => void handle_refresh_all()}>
                  <RefreshCw className="h-3.5 w-3.5" />
                  刷新全部
                </WorkspaceSurfaceToolbarAction>
                <WorkspaceSurfaceToolbarAction onClick={() => set_is_dialog_open(true)} tone="primary">
                  <Plus className="h-3.5 w-3.5" />
                  新建任务
                </WorkspaceSurfaceToolbarAction>
              </>
            )}
          />
        )}
      >
        <section className="mb-4 grid gap-3 md:grid-cols-3">
          <div className="surface-card rounded-[20px] px-4 py-4">
            <p className="text-[11px] font-semibold uppercase tracking-[0.18em] text-(--text-muted)">
              执行中的任务
            </p>
            <p className="mt-2 text-2xl font-bold tracking-[-0.03em] text-(--text-strong)">
              {running_count}
            </p>
            <p className="mt-1 text-sm text-(--text-default)">
              当前正在执行中的自动化任务
            </p>
          </div>
          <div className="surface-card rounded-[20px] px-4 py-4">
            <p className="text-[11px] font-semibold uppercase tracking-[0.18em] text-(--text-muted)">
              已启用
            </p>
            <p className="mt-2 text-2xl font-bold tracking-[-0.03em] text-(--text-strong)">
              {enabled_count}
            </p>
            <p className="mt-1 text-sm text-(--text-default)">
              会继续自动执行的任务数量
            </p>
          </div>
          <div className="surface-card rounded-[20px] px-4 py-4">
            <p className="text-[11px] font-semibold uppercase tracking-[0.18em] text-(--text-muted)">
              已暂停
            </p>
            <p className="mt-2 text-2xl font-bold tracking-[-0.03em] text-(--text-strong)">
              {paused_count}
            </p>
            <p className="mt-1 text-sm text-(--text-default)">
              暂时保留但不会继续执行的任务
            </p>
          </div>
        </section>

        <div className="grid min-h-full gap-4 xl:grid-cols-[360px,minmax(0,1fr)]">
          <HeartbeatSettingsCard
            error_message={automation.heartbeat_error}
            heartbeat={automation.heartbeat}
            is_loading={automation.heartbeat_loading}
            on_refresh={() => void automation.refresh_heartbeat().catch(() => undefined)}
            on_wake={() => void handle_wake()}
            wake_pending={wake_pending}
          />
          <ScheduledTaskList
            error_message={automation.tasks_error}
            is_loading={automation.tasks_loading}
            items={automation.scheduled_tasks}
            on_create={() => set_is_dialog_open(true)}
            on_open_history={set_history_task}
            on_refresh={() => void automation.refresh_tasks().catch(() => undefined)}
            on_run_now={(task) => void handle_run_now(task)}
            on_toggle_enabled={(task) => void handle_toggle_enabled(task)}
            on_delete={(task) => void handle_delete(task)}
            delete_pending_job_id={delete_pending_job_id}
            run_pending_job_id={run_pending_job_id}
            toggle_pending_job_id={toggle_pending_job_id}
          />
        </div>
      </WorkspaceSurfaceScaffold>

      <ScheduledTaskDialog
        agent_id={automation.agent_id}
        is_open={is_dialog_open}
        on_close={() => set_is_dialog_open(false)}
        on_created={(task) => void handle_create_success(task)}
      />
      <ScheduledTaskRunHistoryDialog
        is_open={history_task !== null}
        on_close={() => set_history_task(null)}
        task={history_task}
      />

      {feedback ? (
        <div className="pointer-events-none fixed right-6 top-24 z-40 flex flex-col gap-2">
          <FeedbackBanner
            message={feedback.message}
            on_dismiss={() => set_feedback(null)}
            title={feedback.title}
            tone={feedback.tone}
          />
        </div>
      ) : null}
    </>
  );
}
