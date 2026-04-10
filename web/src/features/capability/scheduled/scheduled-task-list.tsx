"use client";

import { Clock3, History, Play, RefreshCw } from "lucide-react";

import { WorkspacePillButton } from "@/shared/ui/workspace/workspace-pill-button";
import { WorkspaceStatusBadge } from "@/shared/ui/workspace/workspace-status-badge";
import type {
  ScheduledTaskItem,
  ScheduledTaskSchedule,
  ScheduledTaskSessionTarget,
} from "@/types/scheduled-task";

function format_datetime(value: number | null): string {
  if (!value) {
    return "未安排";
  }
  return new Intl.DateTimeFormat("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  }).format(value);
}

function format_interval(seconds: number): string {
  if (seconds % 86400 === 0) {
    return `${seconds / 86400} 天`;
  }
  if (seconds % 3600 === 0) {
    return `${seconds / 3600} 小时`;
  }
  if (seconds % 60 === 0) {
    return `${seconds / 60} 分钟`;
  }
  return `${seconds} 秒`;
}

function get_schedule_summary(schedule: ScheduledTaskSchedule): string {
  if (schedule.kind === "every") {
    return `每 ${format_interval(schedule.interval_seconds)}`;
  }
  if (schedule.kind === "cron") {
    return `Cron · ${schedule.cron_expression}`;
  }
  return `单次 · ${format_datetime(new Date(schedule.run_at).getTime())}`;
}

function get_session_target_summary(target: ScheduledTaskSessionTarget): string {
  const wake_label = target.wake_mode === "now" ? "立即唤醒" : "心跳唤醒";
  if (target.kind === "main") {
    return `主会话 · ${wake_label}`;
  }
  if (target.kind === "bound") {
    return `绑定会话 · ${target.bound_session_key} · ${wake_label}`;
  }
  if (target.kind === "named") {
    return `命名会话 · ${target.named_session_key} · ${wake_label}`;
  }
  return `独立会话 · ${wake_label}`;
}

function get_primary_status(task: ScheduledTaskItem) {
  if (task.running) {
    return { label: "运行中", tone: "running" as const };
  }
  if (task.enabled) {
    return { label: "已启用", tone: "active" as const };
  }
  return { label: "已暂停", tone: "idle" as const };
}

interface ScheduledTaskListProps {
  items: ScheduledTaskItem[];
  is_loading: boolean;
  error_message: string | null;
  run_pending_job_id?: string | null;
  toggle_pending_job_id?: string | null;
  on_create: () => void;
  on_refresh?: () => void | Promise<void>;
  on_run_now?: (task: ScheduledTaskItem) => void | Promise<void>;
  on_toggle_enabled?: (task: ScheduledTaskItem) => void | Promise<void>;
  on_open_history?: (task: ScheduledTaskItem) => void;
}

export function ScheduledTaskList({
  items,
  is_loading,
  error_message,
  run_pending_job_id = null,
  toggle_pending_job_id = null,
  on_create,
  on_refresh,
  on_run_now,
  on_toggle_enabled,
  on_open_history,
}: ScheduledTaskListProps) {
  return (
    <section className="glass-panel-subtle radius-shell-xl flex min-h-[360px] flex-col overflow-hidden">
      <div className="border-b border-[var(--divider-subtle-color)] px-5 py-4">
        <div className="flex items-start justify-between gap-3">
          <div className="min-w-0">
            <div className="flex items-center gap-2">
              <div className="glass-chip flex h-9 w-9 items-center justify-center rounded-2xl">
                <Clock3 className="h-4 w-4 text-slate-900/80" />
              </div>
              <div>
                <h2 className="text-[15px] font-semibold tracking-[-0.03em] text-[color:var(--text-strong)]">
                  调度任务
                </h2>
                <p className="text-xs text-[color:var(--text-default)]">
                  共 {items.length} 个任务，支持立即执行、启停切换和查看运行记录。
                </p>
              </div>
            </div>
          </div>
          <div className="flex items-center gap-2">
            <WorkspacePillButton
              density="compact"
              disabled={is_loading}
              onClick={() => void on_refresh?.()}
              size="sm"
              variant="outlined"
            >
              <RefreshCw className="h-3.5 w-3.5" />
              刷新
            </WorkspacePillButton>
            <WorkspacePillButton
              density="compact"
              onClick={on_create}
              size="sm"
              variant="primary"
            >
              新建任务
            </WorkspacePillButton>
          </div>
        </div>
      </div>

      <div className="soft-scrollbar min-h-0 flex-1 overflow-y-auto p-5">
        {is_loading ? (
          <div className="space-y-3">
            {Array.from({ length: 3 }).map((_, index) => (
              <div
                key={index}
                className="h-[148px] animate-pulse rounded-[24px] bg-white/45"
              />
            ))}
          </div>
        ) : error_message ? (
          <div className="flex min-h-[240px] flex-col items-center justify-center rounded-[24px] border border-rose-500/15 bg-rose-500/6 px-5 text-center">
            <p className="text-sm font-semibold text-rose-500">任务列表加载失败</p>
            <p className="mt-2 max-w-md text-sm leading-6 text-[color:var(--text-default)]">
              {error_message}
            </p>
            <WorkspacePillButton class_name="mt-4" onClick={() => void on_refresh?.()} size="sm" variant="outlined">
              重试
            </WorkspacePillButton>
          </div>
        ) : items.length === 0 ? (
          <div className="flex min-h-[240px] flex-col items-center justify-center rounded-[24px] border border-dashed border-[var(--divider-subtle-color)] px-5 text-center">
            <div className="glass-chip flex h-14 w-14 items-center justify-center rounded-[20px]">
              <Clock3 className="h-6 w-6 text-slate-900/78" />
            </div>
            <h3 className="mt-5 text-lg font-bold tracking-[-0.03em] text-[color:var(--text-strong)]">
              还没有定时任务
            </h3>
            <p className="mt-2 max-w-sm text-sm leading-6 text-[color:var(--text-default)]">
              创建第一个自动化任务后，这里会显示调度频率、目标会话和最近运行情况。
            </p>
            <WorkspacePillButton class_name="mt-4" onClick={on_create} size="sm" variant="primary">
              创建任务
            </WorkspacePillButton>
          </div>
        ) : (
          <div className="space-y-3">
            {items.map((task) => {
              const status = get_primary_status(task);
              const run_pending = run_pending_job_id === task.job_id;
              const toggle_pending = toggle_pending_job_id === task.job_id;
              return (
                <article
                  key={task.job_id}
                  className="rounded-[24px] border border-[var(--divider-subtle-color)] bg-white/55 px-5 py-4 shadow-[0_16px_40px_rgba(15,23,42,0.07)] backdrop-blur-xl"
                >
                  <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
                    <div className="min-w-0 flex-1">
                      <div className="flex flex-wrap items-center gap-2">
                        <h3 className="text-[15px] font-semibold text-[color:var(--text-strong)]">
                          {task.name}
                        </h3>
                        <WorkspaceStatusBadge label={status.label} size="compact" tone={status.tone} />
                        {task.running ? (
                          <WorkspaceStatusBadge label="执行占用中" size="compact" tone="running" />
                        ) : null}
                      </div>
                      <div className="mt-3 grid gap-3 text-sm text-[color:var(--text-default)] md:grid-cols-2">
                        <div className="rounded-[18px] border border-[var(--divider-subtle-color)] bg-white/45 px-3 py-2.5">
                          <p className="text-[11px] font-semibold uppercase tracking-[0.18em] text-[color:var(--text-muted)]">
                            调度规则
                          </p>
                          <p className="mt-1.5 font-medium text-[color:var(--text-strong)]">
                            {get_schedule_summary(task.schedule)}
                          </p>
                        </div>
                        <div className="rounded-[18px] border border-[var(--divider-subtle-color)] bg-white/45 px-3 py-2.5">
                          <p className="text-[11px] font-semibold uppercase tracking-[0.18em] text-[color:var(--text-muted)]">
                            目标会话
                          </p>
                          <p className="mt-1.5 font-medium text-[color:var(--text-strong)]">
                            {get_session_target_summary(task.session_target)}
                          </p>
                        </div>
                      </div>
                      <div className="mt-3 flex flex-wrap gap-x-5 gap-y-2 text-xs text-[color:var(--text-default)]">
                        <span>下次运行 {format_datetime(task.next_run_at)}</span>
                        <span>最近执行 {format_datetime(task.last_run_at)}</span>
                        <span>Agent {task.agent_id}</span>
                      </div>
                    </div>

                    <div className="flex shrink-0 flex-wrap items-center gap-2 lg:justify-end">
                      <WorkspacePillButton
                        density="compact"
                        disabled={run_pending || task.running}
                        onClick={() => void on_run_now?.(task)}
                        size="sm"
                        variant="outlined"
                      >
                        <Play className="h-3.5 w-3.5" />
                        {run_pending ? "执行中" : "立即运行"}
                      </WorkspacePillButton>
                      <WorkspacePillButton
                        density="compact"
                        disabled={toggle_pending}
                        onClick={() => void on_toggle_enabled?.(task)}
                        size="sm"
                        variant={task.enabled ? "tonal" : "primary"}
                      >
                        {toggle_pending ? "处理中" : task.enabled ? "暂停" : "启用"}
                      </WorkspacePillButton>
                      <WorkspacePillButton
                        density="compact"
                        onClick={() => on_open_history?.(task)}
                        size="sm"
                        variant="outlined"
                      >
                        <History className="h-3.5 w-3.5" />
                        运行历史
                      </WorkspacePillButton>
                    </div>
                  </div>
                </article>
              );
            })}
          </div>
        )}
      </div>
    </section>
  );
}
