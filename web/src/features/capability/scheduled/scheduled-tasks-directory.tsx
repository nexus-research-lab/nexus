"use client";

import { useCallback, useEffect, useState } from "react";
import { CalendarClock, Plus, RefreshCw } from "lucide-react";

import { get_desktop_websocket_protocols } from "@/config/desktop-runtime";
import { get_agent_ws_url } from "@/config/options";
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

import { useWebSocket } from "@/lib/websocket";
import { FeedbackBannerStack } from "@/shared/ui/feedback/feedback-banner-stack";
import { ScheduledTaskDialog } from "./dialog/scheduled-task-dialog";
import { ScheduledTaskList } from "./scheduled-task-list";
import { ScheduledTaskRunHistoryDialog } from "./scheduled-task-run-history-dialog";
import type { EventMessage } from "@/types/conversation/message";

interface FeedbackState {
  tone: "success" | "warning" | "error";
  title: string;
  message: string;
}

const SCHEDULED_TASKS_MUTATED_EVENT = "nexus:scheduled-tasks-mutated";
const RUNNING_TASK_FALLBACK_POLL_INTERVAL_MS = 30000;
const ENABLED_TASK_FALLBACK_POLL_INTERVAL_MS = 120000;

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
  const { t } = useI18n();
  const ws_url = get_agent_ws_url();
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
  const feedback_items = feedback
    ? [
        {
          key: "feedback",
          message: feedback.message,
          on_dismiss: () => set_feedback(null),
          title: feedback.title,
          tone: feedback.tone,
        },
      ]
    : [];

  const handle_realtime_message = useCallback((raw_message: unknown) => {
    const event = raw_message as EventMessage;
    if (event.event_type !== "scheduled_task_changed") {
      return;
    }
    notify_scheduled_tasks_mutated(event.agent_id ?? "");
    if (typeof document !== "undefined" && document.visibilityState !== "visible") {
      return;
    }
    void refresh_tasks({ silent: true }).catch((err: unknown) => console.debug("[scheduled-tasks] Realtime refresh failed:", err));
  }, [refresh_tasks]);

  const { send: ws_send, state: ws_state } = useWebSocket({
    url: ws_url,
    protocols: get_desktop_websocket_protocols(),
    auto_connect: true,
    reconnect: true,
    heartbeat_interval: 30000,
    on_message: handle_realtime_message,
  });

  useEffect(() => {
    if (ws_state !== "connected") {
      return;
    }
    ws_send({ type: "subscribe_app_events" });
    return () => {
      ws_send({ type: "unsubscribe_app_events" });
    };
  }, [ws_send, ws_state]);

  useEffect(() => {
    if (typeof window === "undefined") {
      return;
    }

    const handle_page_revalidate = () => {
      if (document.visibilityState !== "visible") {
        return;
      }
      void refresh_tasks({ silent: true }).catch((err: unknown) => console.debug("[scheduled-tasks] Background refresh failed:", err));
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
    if (ws_state === "connected") {
      return;
    }
    const poll_interval_ms = running_count > 0
      ? RUNNING_TASK_FALLBACK_POLL_INTERVAL_MS
      : enabled_count > 0 ? ENABLED_TASK_FALLBACK_POLL_INTERVAL_MS : 0;
    if (!poll_interval_ms) {
      return;
    }

    const interval_id = window.setInterval(() => {
      if (document.visibilityState !== "visible") {
        return;
      }
      void refresh_tasks({ silent: true }).catch((err: unknown) => console.debug("[scheduled-tasks] Background refresh failed:", err));
    }, poll_interval_ms);

    return () => window.clearInterval(interval_id);
  }, [enabled_count, refresh_tasks, running_count, ws_state]);

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
      const updated_task = await automation.toggle_task(task);
      set_history_task((current_task) => {
        if (!current_task || current_task.job_id !== updated_task.job_id) {
          return current_task;
        }
        return updated_task;
      });
      await refresh_tasks_best_effort(
        automation,
        updated_task.agent_id,
        {
          title: updated_task.enabled ? "任务已启用" : "任务已暂停",
          message: updated_task.enabled
            ? `${updated_task.name} 已恢复自动调度`
            : `${updated_task.name} 不再参与后续调度`,
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

  const handle_recover_task_run = async (task: ScheduledTaskItem, run: ScheduledTaskRunItem) => {
    try {
      const updated_task = await recover_scheduled_task_run_api(task.job_id, { run_id: run.run_id });
      set_history_task((current) => current?.job_id === updated_task.job_id ? updated_task : current);
      await refresh_tasks_best_effort(
        automation,
        automation.agent_id,
        {
          title: "运行占用已释放",
          message: `${task.name} 的当前 run 已标记为 cancelled`,
        },
        "任务列表刷新失败，运行状态稍后会同步",
        set_feedback,
      );
    } catch (error) {
      set_feedback({
        tone: "error",
        title: "释放运行占用失败",
        message: error instanceof Error ? error.message : "释放运行占用失败",
      });
      throw error;
    }
  };

  const handle_retry_delivery = async (task: ScheduledTaskItem, run: ScheduledTaskRunItem) => {
    try {
      const updated_run = await retry_scheduled_task_run_delivery_api(task.job_id, run.run_id);
      await refresh_tasks_best_effort(
        automation,
        automation.agent_id,
        {
          title: updated_run.delivery_status === "succeeded" ? "投递已恢复" : "投递已重试",
          message: updated_run.delivery_status === "succeeded"
            ? `${task.name} 的运行结果已重新投递`
            : `${task.name} 的投递状态已更新为 ${updated_run.delivery_status ?? "unknown"}`,
        },
        "任务列表刷新失败，投递状态稍后会同步",
        set_feedback,
      );
    } catch (error) {
      set_feedback({
        tone: "error",
        title: "重试投递失败",
        message: error instanceof Error ? error.message : "重试投递失败",
      });
      throw error;
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
                <WorkspaceSurfaceToolbarAction onClick={() => void handle_refresh_all()}>
                  <RefreshCw className="h-3.5 w-3.5" />
                  {t("capability.refresh_all")}
                </WorkspaceSurfaceToolbarAction>
                <WorkspaceSurfaceToolbarAction onClick={() => set_is_dialog_open(true)} tone="primary">
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
              value={running_count}
            />
            <ScheduledMetricItem
              description="后续继续参与调度"
              label="已启用"
              value={enabled_count}
            />
            <ScheduledMetricItem
              description="暂时不会自动触发"
              label="已暂停"
              value={paused_count}
            />
          </section>

          <ScheduledTaskList
            error_message={automation.tasks_error}
            is_loading={automation.tasks_loading}
            items={automation.scheduled_tasks}
            on_create={() => set_is_dialog_open(true)}
            on_open_history={set_history_task}
            on_refresh={() => void refresh_tasks().catch((err: unknown) => console.debug("[scheduled-tasks] Manual refresh failed:", err))}
            on_run_now={(task) => void handle_run_now(task)}
            on_toggle_enabled={(task) => void handle_toggle_enabled(task)}
            on_delete={(task) => void handle_delete(task)}
            on_edit={set_editing_task}
            delete_pending_job_id={delete_pending_job_id}
            run_pending_job_id={run_pending_job_id}
            toggle_pending_job_id={toggle_pending_job_id}
          />
        </CapabilityPageLayout>
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
        on_recover_task_run={(task, run) => handle_recover_task_run(task, run)}
        on_retry_delivery={(task, run) => handle_retry_delivery(task, run)}
        on_retry_task={(task) => handle_run_now(task)}
        task={history_task}
      />

      <FeedbackBannerStack items={feedback_items} />
    </>
  );
}
