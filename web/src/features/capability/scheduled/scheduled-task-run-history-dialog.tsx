"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { History, RefreshCw, X } from "lucide-react";

import { listScheduledTaskRunsApi } from "@/lib/scheduled-task-api";
import { WorkspacePillButton } from "@/shared/ui/workspace/workspace-pill-button";
import { WorkspaceStatusBadge } from "@/shared/ui/workspace/workspace-status-badge";
import type { ScheduledTaskItem, ScheduledTaskRunItem } from "@/types/scheduled-task";

function format_datetime(value: number | null): string {
  if (!value) {
    return "未记录";
  }
  return new Intl.DateTimeFormat("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  }).format(value);
}

function format_duration(started_at: number | null, finished_at: number | null): string {
  if (!started_at || !finished_at) {
    return "未完成";
  }
  const diff_seconds = Math.max(0, Math.round((finished_at - started_at) / 1000));
  if (diff_seconds < 60) {
    return `${diff_seconds} 秒`;
  }
  const minutes = Math.floor(diff_seconds / 60);
  const seconds = diff_seconds % 60;
  return `${minutes} 分 ${seconds} 秒`;
}

function get_status_meta(status: ScheduledTaskRunItem["status"]) {
  if (status === "succeeded") {
    return { label: "成功", tone: "success" as const };
  }
  if (status === "running") {
    return { label: "运行中", tone: "running" as const };
  }
  if (status === "pending") {
    return { label: "等待中", tone: "default" as const };
  }
  if (status === "cancelled") {
    return { label: "已取消", tone: "idle" as const };
  }
  return { label: "失败", tone: "default" as const };
}

interface ScheduledTaskRunHistoryDialogProps {
  task: ScheduledTaskItem | null;
  is_open: boolean;
  on_close: () => void;
}

export function ScheduledTaskRunHistoryDialog({
  task,
  is_open,
  on_close,
}: ScheduledTaskRunHistoryDialogProps) {
  const [runs, set_runs] = useState<ScheduledTaskRunItem[]>([]);
  const [is_loading, set_is_loading] = useState(false);
  const [error_message, set_error_message] = useState<string | null>(null);
  const active_task_job_id_ref = useRef<string | null>(null);
  const runs_request_token_ref = useRef(0);
  const task_job_id = task?.job_id ?? null;

  const load_runs = useCallback(async (job_id: string) => {
    const request_token = runs_request_token_ref.current + 1;
    runs_request_token_ref.current = request_token;
    set_is_loading(true);
    set_error_message(null);
    try {
      const result = await listScheduledTaskRunsApi(job_id);
      if (active_task_job_id_ref.current !== job_id || runs_request_token_ref.current !== request_token) {
        return;
      }
      set_runs(result);
    } catch (error) {
      if (active_task_job_id_ref.current !== job_id || runs_request_token_ref.current !== request_token) {
        return;
      }
      set_error_message(error instanceof Error ? error.message : "加载运行历史失败");
      set_runs([]);
    } finally {
      if (active_task_job_id_ref.current !== job_id || runs_request_token_ref.current !== request_token) {
        return;
      }
      set_is_loading(false);
    }
  }, []);

  useEffect(() => {
    if (!is_open) {
      active_task_job_id_ref.current = null;
      runs_request_token_ref.current += 1;
      set_runs([]);
      set_error_message(null);
      set_is_loading(false);
      return;
    }
    const handle_key_down = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        event.preventDefault();
        on_close();
      }
    };
    window.addEventListener("keydown", handle_key_down);
    return () => {
      window.removeEventListener("keydown", handle_key_down);
    };
  }, [is_open, on_close]);

  useEffect(() => {
    if (!is_open || !task_job_id) {
      active_task_job_id_ref.current = null;
      runs_request_token_ref.current += 1;
      set_runs([]);
      set_error_message(null);
      set_is_loading(false);
      return;
    }
    active_task_job_id_ref.current = task_job_id;
    set_runs([]);
    void load_runs(task_job_id);
  }, [is_open, load_runs, task_job_id]);

  if (!is_open || !task) {
    return null;
  }

  const handle_refresh = () => {
    void load_runs(task_job_id ?? "");
  };

  return (
    <div
      aria-labelledby="scheduled-task-run-history-title"
      aria-modal="true"
      className="dialog-backdrop"
      role="dialog"
    >
      <div className="dialog-shell radius-shell-xl flex h-[82vh] w-full max-w-4xl flex-col overflow-hidden">
        <div className="dialog-header">
          <div className="min-w-0 flex-1">
            <div className="flex items-center gap-2">
              <h3 className="dialog-title" id="scheduled-task-run-history-title">
                {task.name} 运行历史
              </h3>
              <WorkspaceStatusBadge
                label={task.running ? "运行中" : task.enabled ? "已启用" : "已暂停"}
                size="compact"
                tone={task.running ? "running" : task.enabled ? "active" : "idle"}
              />
            </div>
            <p className="dialog-subtitle mt-1">
              Job ID: {task.job_id}
            </p>
          </div>
          <div className="flex items-center gap-2">
            <WorkspacePillButton density="compact" onClick={() => void handle_refresh()} size="sm" variant="outlined">
              <RefreshCw className="h-3.5 w-3.5" />
              刷新
            </WorkspacePillButton>
            <WorkspacePillButton aria-label="关闭" density="compact" onClick={on_close} size="icon" variant="icon">
              <X className="h-4 w-4" />
            </WorkspacePillButton>
          </div>
        </div>

        <div className="soft-scrollbar min-h-0 flex-1 overflow-y-auto px-6 py-5">
          {is_loading ? (
            <div className="space-y-3">
              {Array.from({ length: 4 }).map((_, index) => (
                <div
                  key={index}
                  className="h-[132px] animate-pulse rounded-[24px] bg-white/45"
                />
              ))}
            </div>
          ) : error_message ? (
            <div className="flex min-h-[320px] flex-col items-center justify-center rounded-[24px] border border-rose-500/15 bg-rose-500/6 px-5 text-center">
              <p className="text-sm font-semibold text-rose-500">运行历史加载失败</p>
              <p className="mt-2 max-w-md text-sm leading-6 text-[color:var(--text-default)]">
                {error_message}
              </p>
            </div>
          ) : runs.length === 0 ? (
            <div className="flex min-h-[320px] flex-col items-center justify-center rounded-[24px] border border-dashed border-[var(--divider-subtle-color)] px-5 text-center">
              <div className="glass-chip flex h-14 w-14 items-center justify-center rounded-[20px]">
                <History className="h-6 w-6 text-slate-900/78" />
              </div>
              <h4 className="mt-5 text-lg font-bold tracking-[-0.03em] text-[color:var(--text-strong)]">
                还没有运行记录
              </h4>
              <p className="mt-2 max-w-sm text-sm leading-6 text-[color:var(--text-default)]">
                手动执行或等调度器首次触发后，这里会显示每次运行的状态、耗时和错误信息。
              </p>
            </div>
          ) : (
            <div className="space-y-3">
              {runs.map((run) => {
                const status = get_status_meta(run.status);
                return (
                  <article
                    key={run.run_id}
                    className="rounded-[24px] border border-[var(--divider-subtle-color)] bg-white/55 px-5 py-4 shadow-[0_16px_40px_rgba(15,23,42,0.07)] backdrop-blur-xl"
                  >
                    <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
                      <div className="min-w-0 flex-1">
                        <div className="flex flex-wrap items-center gap-2">
                          <WorkspaceStatusBadge label={status.label} size="compact" tone={status.tone} />
                          <span className="text-xs font-medium text-[color:var(--text-default)]">
                            Run ID {run.run_id}
                          </span>
                        </div>
                        <div className="mt-3 grid gap-3 text-sm text-[color:var(--text-default)] md:grid-cols-2">
                          <div className="rounded-[18px] border border-[var(--divider-subtle-color)] bg-white/45 px-3 py-2.5">
                            <p className="text-[11px] font-semibold uppercase tracking-[0.18em] text-[color:var(--text-muted)]">
                              调度时间
                            </p>
                            <p className="mt-1.5 font-medium text-[color:var(--text-strong)]">
                              {format_datetime(run.scheduled_for)}
                            </p>
                          </div>
                          <div className="rounded-[18px] border border-[var(--divider-subtle-color)] bg-white/45 px-3 py-2.5">
                            <p className="text-[11px] font-semibold uppercase tracking-[0.18em] text-[color:var(--text-muted)]">
                              执行耗时
                            </p>
                            <p className="mt-1.5 font-medium text-[color:var(--text-strong)]">
                              {format_duration(run.started_at, run.finished_at)}
                            </p>
                          </div>
                        </div>
                      </div>

                      <div className="shrink-0 text-right text-sm text-[color:var(--text-default)]">
                        <p>开始 {format_datetime(run.started_at)}</p>
                        <p className="mt-1">结束 {format_datetime(run.finished_at)}</p>
                        <p className="mt-1">尝试次数 {run.attempts}</p>
                      </div>
                    </div>
                    {run.error_message ? (
                      <div className="mt-3 rounded-[18px] border border-rose-500/15 bg-rose-500/6 px-3 py-2.5 text-sm text-rose-600">
                        {run.error_message}
                      </div>
                    ) : null}
                  </article>
                );
              })}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
