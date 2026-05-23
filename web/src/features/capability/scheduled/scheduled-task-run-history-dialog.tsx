"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { History, RefreshCw, X } from "lucide-react";

import { list_scheduled_task_runs_api } from "@/lib/api/scheduled-task-api";
import { UiButton, UiIconButton } from "@/shared/ui/button";
import { close_on_escape } from "@/shared/ui/dialog/dialog-keyboard";
import { UiSkeletonCardList } from "@/shared/ui/skeleton";
import { UiStateBlock } from "@/shared/ui/state-block";
import { WorkspaceStatusBadge } from "@/shared/ui/workspace/controls/workspace-status-badge";
import type { ScheduledTaskItem, ScheduledTaskRunItem } from "@/types/capability/scheduled-task";
import { format_scheduled_datetime } from "./scheduled-formatters";

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
  if (status === "queued_to_main_session") {
    return { label: "已入主会话", tone: "default" as const };
  }
  if (status === "skipped") {
    return { label: "已跳过", tone: "idle" as const };
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
      const result = await list_scheduled_task_runs_api(job_id);
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
    const on_key_down = (event: KeyboardEvent) => close_on_escape(event, on_close);
    window.addEventListener("keydown", on_key_down);
    return () => {
      window.removeEventListener("keydown", on_key_down);
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
      data-modal-root="true"
      role="dialog"
    >
      <div className="dialog-shell radius-shell-md flex h-[82vh] w-full max-w-4xl flex-col overflow-hidden">
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
            <UiButton
              onClick={() => void handle_refresh()}
              size="xs"
              type="button"
              variant="text"
            >
              <RefreshCw className="h-3.5 w-3.5" />
              刷新
            </UiButton>
            <UiIconButton
              aria-label="关闭"
              onClick={on_close}
              size="md"
              type="button"
            >
              <X className="h-4 w-4" />
            </UiIconButton>
          </div>
        </div>

        <div className="soft-scrollbar min-h-0 flex-1 overflow-y-auto px-6 py-5">
          {is_loading ? (
            <UiSkeletonCardList card_class_name="min-h-[108px]" count={4} />
          ) : error_message ? (
            <UiStateBlock description={error_message} title="运行历史加载失败" tone="danger" />
          ) : runs.length === 0 ? (
            <UiStateBlock
              description="手动执行或等调度器首次触发后，这里会显示每次运行的状态、耗时和错误信息。"
              icon={<History className="h-6 w-6 text-(--icon-strong)" />}
              title="还没有运行记录"
            />
          ) : (
            <div className="divide-y divide-(--divider-subtle-color)">
              {runs.map((run) => {
                const status = get_status_meta(run.status);
                return (
                  <article
                    key={run.run_id}
                    className="py-4 first:pt-0 last:pb-0"
                  >
                    <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
                      <div className="min-w-0 flex-1">
                        <div className="flex flex-wrap items-center gap-2">
                          <WorkspaceStatusBadge label={status.label} size="compact" tone={status.tone} />
                          <span className="text-xs font-medium text-(--text-default)">
                            Run ID {run.run_id}
                          </span>
                        </div>
                        <div className="mt-3 grid gap-3 text-sm text-(--text-default) md:grid-cols-2">
                          <div>
                            <p className="text-[11px] font-semibold uppercase tracking-[0.18em] text-(--text-muted)">
                              调度时间
                            </p>
                            <p className="mt-1.5 font-medium text-(--text-strong)">
                              {format_scheduled_datetime(run.scheduled_for, { include_seconds: true })}
                            </p>
                          </div>
                          <div>
                            <p className="text-[11px] font-semibold uppercase tracking-[0.18em] text-(--text-muted)">
                              执行耗时
                            </p>
                            <p className="mt-1.5 font-medium text-(--text-strong)">
                              {format_duration(run.started_at, run.finished_at)}
                            </p>
                          </div>
                          {run.trigger_kind ? (
                            <div>
                              <p className="text-[11px] font-semibold uppercase tracking-[0.18em] text-(--text-muted)">
                                触发方式
                              </p>
                              <p className="mt-1.5 font-medium text-(--text-strong)">
                                {run.trigger_kind}
                              </p>
                            </div>
                          ) : null}
                          {typeof run.message_count === "number" ? (
                            <div>
                              <p className="text-[11px] font-semibold uppercase tracking-[0.18em] text-(--text-muted)">
                                消息数
                              </p>
                              <p className="mt-1.5 font-medium text-(--text-strong)">
                                {run.message_count}
                              </p>
                            </div>
                          ) : null}
                        </div>
                        {(run.session_key || run.round_id || run.session_id || run.delivery_to) ? (
                          <div className="mt-3 space-y-1.5 text-xs text-(--text-default)">
                            {run.session_key ? <p className="break-all">Session {run.session_key}</p> : null}
                            {run.round_id ? <p className="break-all">Round {run.round_id}</p> : null}
                            {run.session_id ? <p className="break-all">Runtime {run.session_id}</p> : null}
                            {run.delivery_to ? <p className="break-all">Delivery {run.delivery_to}</p> : null}
                          </div>
                        ) : null}
                      </div>

                      <div className="shrink-0 text-right text-sm text-(--text-default)">
                        <p>开始 {format_scheduled_datetime(run.started_at, { include_seconds: true })}</p>
                        <p className="mt-1">结束 {format_scheduled_datetime(run.finished_at, { include_seconds: true })}</p>
                        <p className="mt-1">尝试次数 {run.attempts}</p>
                      </div>
                    </div>
                    {run.error_message ? (
                      <div className="mt-3 rounded-[14px] border border-[color:color-mix(in_srgb,var(--destructive)_15%,transparent)] px-3 py-2.5 text-sm text-(--destructive)">
                        {run.error_message}
                      </div>
                    ) : null}
                    {run.result_summary ? (
                      <div className="mt-3 rounded-[14px] border border-(--divider-subtle-color) px-3 py-2.5 text-sm leading-6 text-(--text-default)">
                        {run.result_summary}
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
