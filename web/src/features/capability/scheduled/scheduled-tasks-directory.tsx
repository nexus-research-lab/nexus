"use client";

import { useEffect, useState } from "react";
import { CalendarClock, Plus, RefreshCw } from "lucide-react";

import { useAutomationController } from "@/hooks/use-automation-controller";
import { delete_scheduled_task_api, run_scheduled_task_api, update_scheduled_task_status_api } from "@/lib/scheduled-task-api";
import {
  WorkspaceSurfaceHeader,
  WorkspaceSurfaceToolbarAction,
} from "@/shared/ui/workspace/workspace-surface-header";
import { WorkspaceSurfaceScaffold } from "@/shared/ui/workspace/workspace-surface-scaffold";
import type { ScheduledTaskItem } from "@/types/scheduled-task";

import { FeedbackBanner } from "../skills/feedback-banner";
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
  const [editing_task, set_editing_task] = useState<ScheduledTaskItem | null>(null);
  const [history_task, set_history_task] = useState<ScheduledTaskItem | null>(null);
  const [feedback, set_feedback] = useState<FeedbackState | null>(null);
  const [run_pending_job_id, set_run_pending_job_id] = useState<string | null>(null);
  const [toggle_pending_job_id, set_toggle_pending_job_id] = useState<string | null>(null);
  const [delete_pending_job_id, set_delete_pending_job_id] = useState<string | null>(null);
  const automation = useAutomationController({ include_all_tasks: true });
  const refresh_tasks = automation.refresh_tasks;
  const refresh_all = automation.refresh_all;
  const running_count = automation.scheduled_tasks.filter((task) => task.running).length;
  const enabled_count = automation.scheduled_tasks.filter((task) => task.enabled).length;
  const paused_count = automation.scheduled_tasks.length - enabled_count;

  useEffect(() => {
    if (typeof window === "undefined") {
      return;
    }

    const handle_page_revalidate = () => {
      if (document.visibilityState !== "visible") {
        return;
      }
      void refresh_tasks({ silent: true }).catch(() => undefined);
    };

    window.addEventListener("focus", handle_page_revalidate);
    document.addEventListener("visibilitychange", handle_page_revalidate);

    return () => {
      window.removeEventListener("focus", handle_page_revalidate);
      document.removeEventListener("visibilitychange", handle_page_revalidate);
    };
  }, [refresh_tasks]);

  useEffect(() => {
    if (typeof window === "undefined") {
      return;
    }
    const poll_interval_ms = running_count > 0 ? 3000 : enabled_count > 0 ? 15000 : 0;
    if (!poll_interval_ms) {
      return;
    }

    const interval_id = window.setInterval(() => {
      if (document.visibilityState !== "visible") {
        return;
      }
      void refresh_tasks({ silent: true }).catch(() => undefined);
    }, poll_interval_ms);

    return () => window.clearInterval(interval_id);
  }, [enabled_count, refresh_tasks, running_count]);

  const handle_create_success = async (task: ScheduledTaskItem) => {
    await refresh_tasks_best_effort(
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
      await refresh_all();
    } catch (error) {
      set_feedback({
        tone: "error",
        title: "刷新失败",
        message: error instanceof Error ? error.message : "刷新自动化数据失败",
      });
    }
  };

  const handle_save_success = async (task: ScheduledTaskItem) => {
    await refresh_tasks_best_effort(
      automation,
      task.agent_id,
      {
        title: "任务已更新",
        message: `${task.name} 的配置已保存`,
      },
      "任务列表刷新失败，稍后会自动同步",
      set_feedback,
    );
  };

  const handle_run_now = async (task: ScheduledTaskItem) => {
    set_run_pending_job_id(task.job_id);
    try {
      const result = await run_scheduled_task_api(task.job_id);
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
      await update_scheduled_task_status_api(task.job_id, { enabled: !task.enabled });
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
      await delete_scheduled_task_api(task.job_id);
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
            title="任务管理"
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
              当前有多少任务正在占用执行会话
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
              后续还会继续参与调度的任务数量
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
              仍保留在列表里，但暂时不会再自动触发
            </p>
          </div>
        </section>

        <div className="min-h-full">
          <ScheduledTaskList
            error_message={automation.tasks_error}
            is_loading={automation.tasks_loading}
            items={automation.scheduled_tasks}
            on_create={() => set_is_dialog_open(true)}
            on_open_history={set_history_task}
            on_refresh={() => void refresh_tasks().catch(() => undefined)}
            on_run_now={(task) => void handle_run_now(task)}
            on_toggle_enabled={(task) => void handle_toggle_enabled(task)}
            on_delete={(task) => void handle_delete(task)}
            on_edit={set_editing_task}
            delete_pending_job_id={delete_pending_job_id}
            run_pending_job_id={run_pending_job_id}
            toggle_pending_job_id={toggle_pending_job_id}
          />
        </div>
      </WorkspaceSurfaceScaffold>

      <ScheduledTaskDialog
        agent_id={automation.agent_id}
        initial_task={editing_task}
        is_open={is_dialog_open || editing_task !== null}
        on_close={() => {
          set_is_dialog_open(false);
          set_editing_task(null);
        }}
        on_created={(task) => void handle_create_success(task)}
        on_saved={(task) => void handle_save_success(task)}
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
