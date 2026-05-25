"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { Copy, Download, History, RefreshCw, RotateCcw, X } from "lucide-react";

import { get_workspace_file_download_url } from "@/lib/api/agent-manage-api";
import { list_scheduled_task_runs_api } from "@/lib/api/scheduled-task-api";
import { close_on_escape } from "@/shared/ui/dialog/dialog-keyboard";
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

function get_delivery_status_meta(status: ScheduledTaskRunItem["delivery_status"]) {
  if (status === "succeeded") {
    return { label: "投递成功", tone: "success" as const };
  }
  if (status === "failed") {
    return { label: "投递失败", tone: "default" as const };
  }
  if (status === "pending") {
    return { label: "待投递", tone: "running" as const };
  }
  if (status === "not_attempted") {
    return { label: "未投递", tone: "idle" as const };
  }
  if (status === "not_required" || status === "skipped") {
    return { label: "无需投递", tone: "idle" as const };
  }
  return null;
}

function should_show_assistant_text(run: ScheduledTaskRunItem): boolean {
  if (!run.assistant_text) {
    return false;
  }
  return run.assistant_text.trim() !== (run.result_text ?? "").trim();
}

function artifact_file_name(path: string): string {
  return path.split("/").filter(Boolean).at(-1) ?? "automation-run.md";
}

function is_retryable_status(status: ScheduledTaskRunItem["status"]): boolean {
  return status === "failed" || status === "cancelled" || status === "skipped";
}

function build_run_diagnostic(task: ScheduledTaskItem, run: ScheduledTaskRunItem): string {
  const lines = [
    `Task: ${task.name}`,
    `Job ID: ${task.job_id}`,
    `Agent ID: ${task.agent_id}`,
    `Execution: ${task.execution_kind ?? "agent"}`,
    `Run ID: ${run.run_id}`,
    `Status: ${run.status}`,
    `Delivery Status: ${run.delivery_status || ""}`,
    `Delivery Attempts: ${run.delivery_attempts ?? 0}`,
    `Delivered At: ${format_scheduled_datetime(run.delivered_at, { include_seconds: true })}`,
    `Delivery Next Attempt: ${format_scheduled_datetime(run.delivery_next_attempt_at, { include_seconds: true })}`,
    `Delivery Dead Letter At: ${format_scheduled_datetime(run.delivery_dead_letter_at, { include_seconds: true })}`,
    `Trigger: ${run.trigger_kind || ""}`,
    `Scheduled: ${format_scheduled_datetime(run.scheduled_for, { include_seconds: true })}`,
    `Started: ${format_scheduled_datetime(run.started_at, { include_seconds: true })}`,
    `Finished: ${format_scheduled_datetime(run.finished_at, { include_seconds: true })}`,
    `Duration: ${format_duration(run.started_at, run.finished_at)}`,
    `Attempts: ${run.attempts}`,
    `Session: ${run.session_key || ""}`,
    `Round: ${run.round_id || ""}`,
    `Runtime: ${run.session_id || ""}`,
    `Artifact: ${run.artifact_path || ""}`,
  ];
  if (run.delivery_error) {
    lines.push("", "Delivery Error:", run.delivery_error);
  }
  if (run.error_message) {
    lines.push("", "Error:", run.error_message);
  }
  if (run.result_summary) {
    lines.push("", "Summary:", run.result_summary);
  }
  if (run.result_text) {
    lines.push("", "Result:", run.result_text);
  }
  if (run.assistant_text && run.assistant_text.trim() !== (run.result_text ?? "").trim()) {
    lines.push("", "Assistant:", run.assistant_text);
  }
  return lines.join("\n");
}

interface ScheduledTaskRunHistoryDialogProps {
  task: ScheduledTaskItem | null;
  is_open: boolean;
  on_close: () => void;
  on_retry_task?: (task: ScheduledTaskItem) => void | Promise<void>;
  on_retry_delivery?: (task: ScheduledTaskItem, run: ScheduledTaskRunItem) => void | Promise<void>;
  on_recover_task_run?: (task: ScheduledTaskItem, run: ScheduledTaskRunItem) => void | Promise<void>;
}

export function ScheduledTaskRunHistoryDialog({
  task,
  is_open,
  on_close,
  on_retry_task,
  on_retry_delivery,
  on_recover_task_run,
}: ScheduledTaskRunHistoryDialogProps) {
  const [runs, set_runs] = useState<ScheduledTaskRunItem[]>([]);
  const [is_loading, set_is_loading] = useState(false);
  const [error_message, set_error_message] = useState<string | null>(null);
  const [action_message, set_action_message] = useState<string | null>(null);
  const [copied_run_id, set_copied_run_id] = useState<string | null>(null);
  const [retrying_run_id, set_retrying_run_id] = useState<string | null>(null);
  const [retrying_delivery_run_id, set_retrying_delivery_run_id] = useState<string | null>(null);
  const [recovering_run_id, set_recovering_run_id] = useState<string | null>(null);
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
      set_action_message(null);
      set_copied_run_id(null);
      set_retrying_run_id(null);
      set_retrying_delivery_run_id(null);
      set_recovering_run_id(null);
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
      set_action_message(null);
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

  const handle_copy_diagnostic = async (run: ScheduledTaskRunItem) => {
    const diagnostic = build_run_diagnostic(task, run);
    try {
      await navigator.clipboard.writeText(diagnostic);
      set_copied_run_id(run.run_id);
      set_action_message("诊断信息已复制");
    } catch {
      set_action_message("浏览器未允许写入剪贴板，请使用运行产物查看完整诊断");
    }
  };

  const handle_retry = async (run: ScheduledTaskRunItem) => {
    if (!on_retry_task || !task_job_id) {
      return;
    }
    set_retrying_run_id(run.run_id);
    set_action_message(null);
    try {
      await on_retry_task(task);
      await load_runs(task_job_id);
      set_action_message("已触发重新运行");
    } catch (error) {
      set_action_message(error instanceof Error ? error.message : "重新运行失败");
    } finally {
      set_retrying_run_id(null);
    }
  };

  const handle_retry_delivery = async (run: ScheduledTaskRunItem) => {
    if (!on_retry_delivery || !task_job_id) {
      return;
    }
    set_retrying_delivery_run_id(run.run_id);
    set_action_message(null);
    try {
      await on_retry_delivery(task, run);
      await load_runs(task_job_id);
      set_action_message("已重试投递");
    } catch (error) {
      set_action_message(error instanceof Error ? error.message : "重试投递失败");
    } finally {
      set_retrying_delivery_run_id(null);
    }
  };

  const handle_recover = async (run: ScheduledTaskRunItem) => {
    if (!on_recover_task_run || !task_job_id) {
      return;
    }
    if (!window.confirm(`确认释放 run ${run.run_id} 的运行占用吗？该 run 会被标记为 cancelled。`)) {
      return;
    }
    set_recovering_run_id(run.run_id);
    set_action_message(null);
    try {
      await on_recover_task_run(task, run);
      await load_runs(task_job_id);
      set_action_message("已释放运行占用");
    } catch (error) {
      set_action_message(error instanceof Error ? error.message : "释放运行占用失败");
    } finally {
      set_recovering_run_id(null);
    }
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
            {action_message ? (
              <p className="mt-2 text-xs font-medium text-(--text-default)">
                {action_message}
              </p>
            ) : null}
          </div>
          <div className="flex items-center gap-2">
            <button
              className="inline-flex items-center gap-1.5 text-[11px] font-semibold text-(--text-default) transition duration-(--motion-duration-fast) hover:text-(--text-strong)"
              onClick={() => void handle_refresh()}
              type="button"
            >
              <RefreshCw className="h-3.5 w-3.5" />
              刷新
            </button>
            <button
              aria-label="关闭"
              className="inline-flex h-8 w-8 items-center justify-center rounded-[10px] text-(--icon-default) transition duration-(--motion-duration-fast) hover:bg-(--surface-interactive-hover-background) hover:text-(--icon-strong)"
              onClick={on_close}
              type="button"
            >
              <X className="h-4 w-4" />
            </button>
          </div>
        </div>

        <div className="soft-scrollbar min-h-0 flex-1 overflow-y-auto px-6 py-5">
          {is_loading ? (
            <div className="space-y-3">
              {Array.from({ length: 4 }).map((_, index) => (
                <div
                  key={index}
                  className="h-[108px] animate-pulse rounded-[16px] border border-(--divider-subtle-color)"
                />
              ))}
            </div>
          ) : error_message ? (
            <div className="flex min-h-[320px] flex-col items-center justify-center rounded-[18px] border border-[color:color-mix(in_srgb,var(--destructive)_15%,transparent)] px-5 text-center">
              <p className="text-sm font-semibold text-(--destructive)">运行历史加载失败</p>
              <p className="mt-2 max-w-md text-sm leading-6 text-(--text-default)">
                {error_message}
              </p>
            </div>
          ) : runs.length === 0 ? (
            <div className="flex min-h-[320px] flex-col items-center justify-center rounded-[18px] border border-dashed border-(--divider-subtle-color) px-5 text-center">
              <div className="flex h-14 w-14 items-center justify-center rounded-[16px] border border-(--divider-subtle-color)">
                <History className="h-6 w-6 text-(--icon-strong)" />
              </div>
              <h4 className="mt-5 text-lg font-bold tracking-[-0.03em] text-(--text-strong)">
                还没有运行记录
              </h4>
              <p className="mt-2 max-w-sm text-sm leading-6 text-(--text-default)">
                手动执行或等调度器首次触发后，这里会显示每次运行的状态、耗时和错误信息。
              </p>
            </div>
          ) : (
            <div className="divide-y divide-(--divider-subtle-color)">
              {runs.map((run) => {
                const status = get_status_meta(run.status);
                const delivery_status = get_delivery_status_meta(run.delivery_status);
                return (
                  <article
                    key={run.run_id}
                    className="py-4 first:pt-0 last:pb-0"
                  >
                    <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
                      <div className="min-w-0 flex-1">
                        <div className="flex flex-wrap items-center gap-2">
                          <WorkspaceStatusBadge label={status.label} size="compact" tone={status.tone} />
                          {delivery_status ? (
                            <WorkspaceStatusBadge label={delivery_status.label} size="compact" tone={delivery_status.tone} />
                          ) : null}
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
                        {(run.session_key || run.round_id || run.session_id || run.delivery_to || run.delivered_at || run.delivery_attempts || run.delivery_next_attempt_at || run.delivery_dead_letter_at) ? (
                          <div className="mt-3 space-y-1.5 text-xs text-(--text-default)">
                            {run.session_key ? <p className="break-all">Session {run.session_key}</p> : null}
                            {run.round_id ? <p className="break-all">Round {run.round_id}</p> : null}
                            {run.session_id ? <p className="break-all">Runtime {run.session_id}</p> : null}
                            {run.delivery_to ? <p className="break-all">Delivery {run.delivery_to}</p> : null}
                            {run.delivered_at ? <p>Delivered {format_scheduled_datetime(run.delivered_at, { include_seconds: true })}</p> : null}
                            {run.delivery_attempts ? <p>Delivery attempts {run.delivery_attempts}</p> : null}
                            {run.delivery_next_attempt_at ? <p>Next delivery retry {format_scheduled_datetime(run.delivery_next_attempt_at, { include_seconds: true })}</p> : null}
                            {run.delivery_dead_letter_at ? <p>Delivery dead letter {format_scheduled_datetime(run.delivery_dead_letter_at, { include_seconds: true })}</p> : null}
                          </div>
                        ) : null}
                      </div>

                      <div className="shrink-0 text-right text-sm text-(--text-default)">
                        <p>开始 {format_scheduled_datetime(run.started_at, { include_seconds: true })}</p>
                        <p className="mt-1">结束 {format_scheduled_datetime(run.finished_at, { include_seconds: true })}</p>
                        <p className="mt-1">尝试次数 {run.attempts}</p>
                        <div className="mt-2 flex flex-col items-end gap-1.5">
                          <button
                            className="inline-flex items-center justify-end gap-1.5 text-xs font-semibold text-(--text-default) transition duration-(--motion-duration-fast) hover:text-(--text-strong)"
                            onClick={() => void handle_copy_diagnostic(run)}
                            type="button"
                          >
                            <Copy className="h-3.5 w-3.5" />
                            {copied_run_id === run.run_id ? "已复制" : "复制诊断"}
                          </button>
                          {is_retryable_status(run.status) && on_retry_task ? (
                            <button
                              className="inline-flex items-center justify-end gap-1.5 text-xs font-semibold text-(--primary) transition duration-(--motion-duration-fast) hover:text-(--primary-hover) disabled:opacity-60"
                              disabled={retrying_run_id === run.run_id || task.running}
                              onClick={() => void handle_retry(run)}
                              title={task.running ? "任务当前正在运行" : "用当前任务配置重新运行一次"}
                              type="button"
                            >
                              <RotateCcw className="h-3.5 w-3.5" />
                              {retrying_run_id === run.run_id ? "触发中" : "重新运行"}
                            </button>
                          ) : null}
                          {run.delivery_status === "failed" && on_retry_delivery ? (
                            <button
                              className="inline-flex items-center justify-end gap-1.5 text-xs font-semibold text-(--primary) transition duration-(--motion-duration-fast) hover:text-(--primary-hover) disabled:opacity-60"
                              disabled={retrying_delivery_run_id === run.run_id}
                              onClick={() => void handle_retry_delivery(run)}
                              title="只重试这次运行的结果投递，不重新执行任务"
                              type="button"
                            >
                              <RotateCcw className="h-3.5 w-3.5" />
                              {retrying_delivery_run_id === run.run_id ? "投递中" : "重试投递"}
                            </button>
                          ) : null}
                          {run.status === "running" && task.running && on_recover_task_run ? (
                            <button
                              className="inline-flex items-center justify-end gap-1.5 text-xs font-semibold text-(--destructive) transition duration-(--motion-duration-fast) hover:text-(--destructive) disabled:opacity-60"
                              disabled={recovering_run_id === run.run_id}
                              onClick={() => void handle_recover(run)}
                              title="把该运行标记为取消，并释放任务占用"
                              type="button"
                            >
                              <X className="h-3.5 w-3.5" />
                              {recovering_run_id === run.run_id ? "释放中" : "释放占用"}
                            </button>
                          ) : null}
                        </div>
                        {run.artifact_path ? (
                          <a
                            className="mt-2 inline-flex items-center justify-end gap-1.5 text-xs font-semibold text-(--primary) transition duration-(--motion-duration-fast) hover:text-(--primary-hover)"
                            download={artifact_file_name(run.artifact_path)}
                            href={get_workspace_file_download_url(task.agent_id, run.artifact_path)}
                            rel="noopener noreferrer"
                            target="_blank"
                          >
                            <Download className="h-3.5 w-3.5" />
                            下载产物
                          </a>
                        ) : null}
                      </div>
                    </div>
                    {run.error_message ? (
                      <div className="mt-3 rounded-[14px] border border-[color:color-mix(in_srgb,var(--destructive)_15%,transparent)] px-3 py-2.5 text-sm text-(--destructive)">
                        {run.error_message}
                      </div>
                    ) : null}
                    {run.delivery_error ? (
                      <div className="mt-3 rounded-[14px] border border-[color:color-mix(in_srgb,var(--destructive)_15%,transparent)] px-3 py-2.5 text-sm text-(--destructive)">
                        投递失败：{run.delivery_error}
                      </div>
                    ) : null}
                    {run.result_summary ? (
                      <div className="mt-3 rounded-[14px] border border-(--divider-subtle-color) px-3 py-2.5 text-sm leading-6 text-(--text-default)">
                        {run.result_summary}
                      </div>
                    ) : null}
                    {run.result_text ? (
                      <div className="mt-3 rounded-[14px] border border-(--divider-subtle-color) px-3 py-2.5">
                        <p className="text-[11px] font-semibold uppercase tracking-[0.18em] text-(--text-muted)">
                          运行输出
                        </p>
                        <pre className="mt-2 max-h-64 whitespace-pre-wrap break-words text-sm leading-6 text-(--text-default)">
                          {run.result_text}
                        </pre>
                      </div>
                    ) : null}
                    {should_show_assistant_text(run) ? (
                      <div className="mt-3 rounded-[14px] border border-(--divider-subtle-color) px-3 py-2.5">
                        <p className="text-[11px] font-semibold uppercase tracking-[0.18em] text-(--text-muted)">
                          助手回复
                        </p>
                        <pre className="mt-2 max-h-64 whitespace-pre-wrap break-words text-sm leading-6 text-(--text-default)">
                          {run.assistant_text}
                        </pre>
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
