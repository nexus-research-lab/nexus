"use client";

import { Bot, CheckCircle2, Clock3, FileText, Loader2, RefreshCcw, Send, Square, TriangleAlert } from "lucide-react";
import { type FormEvent, useCallback, useEffect, useMemo, useState } from "react";

import {
  get_room_subagent_task_messages_api,
  get_session_subagent_task_messages_api,
  list_room_subagent_tasks_api,
  list_session_subagent_tasks_api,
  send_room_subagent_task_message_api,
  send_session_subagent_task_message_api,
  stop_room_subagent_task_api,
  stop_session_subagent_task_api,
  type SubagentTask,
  type SubagentTaskMessagesResponse,
} from "@/lib/api/subagent-task-api";
import { cn, format_tokens } from "@/lib/utils";
import {
  group_messages_by_round,
} from "@/features/conversation/shared/utils";
import { MessageItem } from "@/features/conversation/shared/message";
import {
  UiDialogBackdrop,
  UiDialogBody,
  UiDialogHeader,
  UiDialogPortal,
  UiDialogShell,
} from "@/shared/ui/dialog/dialog";

export type BackgroundTasksSource =
  | { kind: "session"; session_key: string }
  | { kind: "room"; room_id: string; conversation_id: string };

interface BackgroundTasksDialogProps {
  compact?: boolean;
  open: boolean;
  source: BackgroundTasksSource | null;
  on_close: () => void;
}

const POLL_INTERVAL_MS = 3000;

function is_terminal_status(status?: string | null): boolean {
  return [
    "completed",
    "success",
    "done",
    "stopped",
    "cancelled",
    "canceled",
    "killed",
    "interrupted",
    "failed",
    "error",
  ].includes((status ?? "").toLowerCase().trim());
}

function status_label(status?: string | null): string {
  switch ((status ?? "").toLowerCase().trim()) {
    case "completed":
    case "success":
    case "done":
      return "完成";
    case "stopped":
    case "cancelled":
    case "canceled":
    case "killed":
    case "interrupted":
      return "停止";
    case "failed":
    case "error":
      return "失败";
    default:
      return "运行中";
  }
}

function status_class_name(status?: string | null): string {
  switch (status_label(status)) {
    case "完成":
      return "bg-[color:color-mix(in_srgb,var(--success)_10%,transparent)] text-(--success)";
    case "失败":
      return "bg-[color:color-mix(in_srgb,var(--destructive)_10%,transparent)] text-(--destructive)";
    case "停止":
      return "bg-[color:color-mix(in_srgb,var(--warning)_12%,transparent)] text-(--warning)";
    default:
      return "bg-[color:color-mix(in_srgb,var(--primary)_10%,transparent)] text-(--primary)";
  }
}

function status_icon(status?: string | null) {
  switch (status_label(status)) {
    case "完成":
      return <CheckCircle2 className="h-3.5 w-3.5" />;
    case "失败":
      return <TriangleAlert className="h-3.5 w-3.5" />;
    case "停止":
      return <Square className="h-3.5 w-3.5" />;
    default:
      return <Loader2 className="h-3.5 w-3.5 animate-spin" />;
  }
}

function number_from_usage(usage: Record<string, unknown> | undefined, key: string): number | undefined {
  const value = usage?.[key];
  return typeof value === "number" && value > 0 ? value : undefined;
}

function normalize_timestamp(value?: number): number | null {
  if (!value || value <= 0) {
    return null;
  }
  return value < 1_000_000_000_000 ? value * 1000 : value;
}

function duration_label(duration_ms?: number): string | null {
  if (!duration_ms || duration_ms <= 0) {
    return null;
  }
  const seconds = Math.max(1, Math.round(duration_ms / 1000));
  if (seconds < 60) {
    return `${seconds}s`;
  }
  return `${Math.floor(seconds / 60)}m ${seconds % 60}s`;
}

function task_duration(task: SubagentTask): number | undefined {
  const usage_duration = number_from_usage(task.usage, "duration_ms");
  if (usage_duration) {
    return usage_duration;
  }
  const started_at = normalize_timestamp(task.started_at);
  const updated_at = normalize_timestamp(task.updated_at);
  if (!started_at || !updated_at || updated_at <= started_at) {
    return undefined;
  }
  return updated_at - started_at;
}

function task_metrics(task: SubagentTask): string {
  const total_tokens = number_from_usage(task.usage, "total_tokens");
  const tool_uses = number_from_usage(task.usage, "tool_uses");
  return [
    total_tokens ? `${format_tokens(total_tokens)} tokens` : null,
    tool_uses ? `${tool_uses} tools` : null,
    duration_label(task_duration(task)),
  ].filter(Boolean).join(" · ");
}

function task_title(task: SubagentTask): string {
  return task.name?.trim() || task.agent_type?.trim() || "Subagent";
}

function source_key(source: BackgroundTasksSource | null): string {
  if (!source) {
    return "";
  }
  if (source.kind === "session") {
    return `session:${source.session_key}`;
  }
  return `room:${source.room_id}:${source.conversation_id}`;
}

async function list_tasks(source: BackgroundTasksSource): Promise<SubagentTask[]> {
  if (source.kind === "session") {
    return list_session_subagent_tasks_api(source.session_key);
  }
  return list_room_subagent_tasks_api(source.room_id, source.conversation_id);
}

async function get_task_messages(
  source: BackgroundTasksSource,
  task_id: string,
): Promise<SubagentTaskMessagesResponse> {
  if (source.kind === "session") {
    return get_session_subagent_task_messages_api(source.session_key, task_id);
  }
  return get_room_subagent_task_messages_api(
    source.room_id,
    source.conversation_id,
    task_id,
  );
}

async function stop_task(source: BackgroundTasksSource, task_id: string): Promise<void> {
  if (source.kind === "session") {
    await stop_session_subagent_task_api(source.session_key, task_id);
    return;
  }
  await stop_room_subagent_task_api(source.room_id, source.conversation_id, task_id);
}

async function send_task_message(
  source: BackgroundTasksSource,
  task_id: string,
  message: string,
): Promise<void> {
  if (source.kind === "session") {
    await send_session_subagent_task_message_api(source.session_key, task_id, message);
    return;
  }
  await send_room_subagent_task_message_api(
    source.room_id,
    source.conversation_id,
    task_id,
    message,
  );
}

function TaskTranscript({
  detail,
}: {
  detail: SubagentTaskMessagesResponse | null;
}) {
  const message_groups = useMemo(
    () => group_messages_by_round(detail?.messages ?? []),
    [detail?.messages],
  );
  const round_ids = useMemo(
    () => Array.from(message_groups.keys()),
    [message_groups],
  );

  if (!detail) {
    return (
      <div className="flex min-h-[180px] items-center justify-center text-sm text-(--text-muted)">
        选择一个任务查看 transcript。
      </div>
    );
  }

  if (round_ids.length === 0) {
    return (
      <div className="space-y-3 rounded-[8px] border border-(--divider-subtle-color) bg-(--surface-elevated-background) p-4 text-sm text-(--text-muted)">
        <div className="flex items-center gap-2 font-medium text-(--text-default)">
          <FileText className="h-4 w-4" />
          暂无 transcript
        </div>
        {detail.output ? (
          <pre className="soft-scrollbar max-h-[360px] overflow-auto whitespace-pre-wrap rounded-[8px] bg-(--surface-canvas-background) p-3 text-xs leading-5 text-(--text-default)">
            {detail.output}
          </pre>
        ) : (
          <p>任务还没有写入可展示的输出。</p>
        )}
      </div>
    );
  }

  return (
    <div className="space-y-3">
      {round_ids.map((round_id) => (
        <MessageItem
          key={round_id}
          assistant_content_mode="dm_archived"
          class_name="rounded-[8px] border border-(--divider-subtle-color) bg-(--surface-elevated-background) px-3 py-2"
          compact
          current_agent_name={task_title(detail.task)}
          messages={message_groups.get(round_id) ?? []}
          round_id={round_id}
          workspace_agent_id={detail.task.agent_id ?? null}
        />
      ))}
      {detail.output ? (
        <details className="rounded-[8px] border border-(--divider-subtle-color) bg-(--surface-elevated-background) p-3 text-sm">
          <summary className="cursor-pointer font-medium text-(--text-default)">输出摘要</summary>
          <pre className="soft-scrollbar mt-3 max-h-[280px] overflow-auto whitespace-pre-wrap rounded-[8px] bg-(--surface-canvas-background) p-3 text-xs leading-5 text-(--text-default)">
            {detail.output}
          </pre>
        </details>
      ) : null}
    </div>
  );
}

export function BackgroundTasksDialog({
  compact = false,
  open,
  source,
  on_close,
}: BackgroundTasksDialogProps) {
  const [tasks, set_tasks] = useState<SubagentTask[]>([]);
  const [selected_task_id, set_selected_task_id] = useState<string | null>(null);
  const [detail, set_detail] = useState<SubagentTaskMessagesResponse | null>(null);
  const [list_error, set_list_error] = useState<string | null>(null);
  const [detail_error, set_detail_error] = useState<string | null>(null);
  const [is_loading_list, set_is_loading_list] = useState(false);
  const [is_loading_detail, set_is_loading_detail] = useState(false);
  const [stopping_task_id, set_stopping_task_id] = useState<string | null>(null);
  const [task_message, set_task_message] = useState("");
  const [is_sending_message, set_is_sending_message] = useState(false);
  const current_source_key = source_key(source);
  const selected_task = tasks.find((task) => task.task_id === selected_task_id) ?? null;
  const selected_task_version = selected_task
    ? `${selected_task.status}:${selected_task.updated_at ?? ""}`
    : "";
  const has_running_tasks = tasks.some((task) => !is_terminal_status(task.status));

  const refresh_tasks = useCallback(async () => {
    if (!source) {
      return;
    }
    set_is_loading_list(true);
    set_list_error(null);
    try {
      const items = await list_tasks(source);
      set_tasks(items);
      set_selected_task_id((current) => {
        if (current && items.some((task) => task.task_id === current)) {
          return current;
        }
        return items[0]?.task_id ?? null;
      });
    } catch (error) {
      set_list_error(error instanceof Error ? error.message : String(error));
    } finally {
      set_is_loading_list(false);
    }
  }, [source]);

  useEffect(() => {
    if (!open || !source) {
      return;
    }
    set_tasks([]);
    set_detail(null);
    set_selected_task_id(null);
    void refresh_tasks();
  }, [current_source_key, open, refresh_tasks, source]);

  useEffect(() => {
    if (!open || !source || !has_running_tasks) {
      return;
    }
    const interval_id = window.setInterval(() => {
      void refresh_tasks();
    }, POLL_INTERVAL_MS);
    return () => window.clearInterval(interval_id);
  }, [has_running_tasks, open, refresh_tasks, source]);

  useEffect(() => {
    if (!open || !source || !selected_task_id) {
      set_detail(null);
      return;
    }
    let cancelled = false;
    set_is_loading_detail(true);
    set_detail_error(null);
    void get_task_messages(source, selected_task_id)
      .then((result) => {
        if (!cancelled) {
          set_detail(result);
        }
      })
      .catch((error) => {
        if (!cancelled) {
          set_detail_error(error instanceof Error ? error.message : String(error));
        }
      })
      .finally(() => {
        if (!cancelled) {
          set_is_loading_detail(false);
        }
      });
    return () => {
      cancelled = true;
    };
  }, [current_source_key, open, selected_task_id, selected_task_version, source]);

  useEffect(() => {
    set_task_message("");
  }, [current_source_key, selected_task_id]);

  const handle_stop_task = useCallback(async () => {
    if (!source || !selected_task || is_terminal_status(selected_task.status)) {
      return;
    }
    set_stopping_task_id(selected_task.task_id);
    set_detail_error(null);
    try {
      await stop_task(source, selected_task.task_id);
      await refresh_tasks();
    } catch (error) {
      set_detail_error(error instanceof Error ? error.message : String(error));
    } finally {
      set_stopping_task_id(null);
    }
  }, [refresh_tasks, selected_task, source]);

  const handle_send_task_message = useCallback(async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!source || !selected_task || is_terminal_status(selected_task.status)) {
      return;
    }
    const message = task_message.trim();
    if (!message) {
      return;
    }
    set_is_sending_message(true);
    set_detail_error(null);
    try {
      await send_task_message(source, selected_task.task_id, message);
      set_task_message("");
      const refreshed = await get_task_messages(source, selected_task.task_id);
      set_detail(refreshed);
      await refresh_tasks();
    } catch (error) {
      set_detail_error(error instanceof Error ? error.message : String(error));
    } finally {
      set_is_sending_message(false);
    }
  }, [refresh_tasks, selected_task, source, task_message]);

  if (!open || !source) {
    return null;
  }

  return (
    <UiDialogPortal>
      <UiDialogBackdrop class_name="z-[9999]" on_close={on_close}>
        <UiDialogShell
          class_name={cn(
            "h-[100dvh] max-h-[100dvh] rounded-none sm:h-[82vh] sm:max-h-[82vh] sm:rounded-[12px]",
            compact && "sm:h-[90vh] sm:max-h-[90vh]",
          )}
          size="wide"
        >
          <UiDialogHeader
            icon={<Bot className="h-4 w-4" />}
            on_close={on_close}
            subtitle="查看后台 Subagent 的状态、输出和 transcript"
            title="后台任务"
            actions={
              <div className="flex items-center gap-2">
                {selected_task && !is_terminal_status(selected_task.status) ? (
                  <button
                    className="inline-flex h-8 items-center gap-1.5 rounded-[7px] border border-(--destructive) px-2.5 text-xs font-medium text-(--destructive) disabled:cursor-not-allowed disabled:opacity-(--disabled-opacity)"
                    disabled={stopping_task_id === selected_task.task_id}
                    onClick={handle_stop_task}
                    type="button"
                  >
                    {stopping_task_id === selected_task.task_id ? (
                      <Loader2 className="h-3.5 w-3.5 animate-spin" />
                    ) : (
                      <Square className="h-3.5 w-3.5" />
                    )}
                    停止
                  </button>
                ) : null}
                <button
                  className="inline-flex h-8 items-center gap-1.5 rounded-[7px] border border-(--divider-subtle-color) px-2.5 text-xs font-medium text-(--text-default) disabled:cursor-not-allowed disabled:opacity-(--disabled-opacity)"
                  disabled={is_loading_list}
                  onClick={() => void refresh_tasks()}
                  type="button"
                >
                  <RefreshCcw className={cn("h-3.5 w-3.5", is_loading_list && "animate-spin")} />
                  刷新
                </button>
              </div>
            }
          />
          <UiDialogBody
            class_name="grid min-h-0 flex-1 gap-3 p-3 sm:grid-cols-[320px_minmax(0,1fr)] sm:p-4"
            scrollable={false}
          >
            <aside className="soft-scrollbar flex min-h-[180px] flex-col gap-2 overflow-auto rounded-[8px] border border-(--divider-subtle-color) bg-(--surface-canvas-background) p-2">
              {list_error ? (
                <div className="rounded-[8px] border border-(--destructive) bg-[color:color-mix(in_srgb,var(--destructive)_8%,transparent)] p-3 text-xs text-(--destructive)">
                  {list_error}
                </div>
              ) : null}
              {tasks.length === 0 && !is_loading_list ? (
                <div className="flex min-h-[120px] items-center justify-center text-sm text-(--text-muted)">
                  暂无后台 Subagent。
                </div>
              ) : null}
              {tasks.map((task) => {
                const metrics = task_metrics(task);
                const selected = task.task_id === selected_task_id;
                return (
                  <button
                    key={task.task_id}
                    className={cn(
                      "grid min-w-0 grid-cols-[28px_minmax(0,1fr)] items-start gap-2 rounded-[8px] border px-2.5 py-2 text-left transition-colors",
                      selected
                        ? "border-(--primary) bg-[color:color-mix(in_srgb,var(--primary)_8%,transparent)]"
                        : "border-(--divider-subtle-color) bg-(--surface-elevated-background) hover:bg-(--surface-hover-background)",
                    )}
                    onClick={() => set_selected_task_id(task.task_id)}
                    type="button"
                  >
                    <span className={cn("mt-0.5 flex h-7 w-7 items-center justify-center rounded-[7px]", status_class_name(task.status))}>
                      {status_icon(task.status)}
                    </span>
                    <span className="min-w-0">
                      <span className="flex min-w-0 items-center gap-1.5 text-[11px] font-medium text-(--text-muted)">
                        <span className="truncate">{task_title(task)}</span>
                        <span className={cn("shrink-0 rounded-[6px] px-1.5 py-0.5 text-[10px] font-semibold", status_class_name(task.status))}>
                          {status_label(task.status)}
                        </span>
                      </span>
                      <span className="mt-1 line-clamp-2 block text-xs font-medium leading-5 text-(--text-strong)">
                        {task.description || "子 Agent 任务"}
                      </span>
                      {metrics ? (
                        <span className="mt-1 flex items-center gap-1 text-[11px] text-(--text-soft)">
                          <Clock3 className="h-3 w-3 shrink-0" />
                          <span className="truncate">{metrics}</span>
                        </span>
                      ) : null}
                    </span>
                  </button>
                );
              })}
            </aside>
            <section className="soft-scrollbar min-h-0 overflow-auto rounded-[8px] border border-(--divider-subtle-color) bg-(--surface-canvas-background) p-3">
              {detail_error ? (
                <div className="mb-3 rounded-[8px] border border-(--destructive) bg-[color:color-mix(in_srgb,var(--destructive)_8%,transparent)] p-3 text-xs text-(--destructive)">
                  {detail_error}
                </div>
              ) : null}
              {is_loading_detail ? (
                <div className="flex min-h-[180px] items-center justify-center gap-2 text-sm text-(--text-muted)">
                  <Loader2 className="h-4 w-4 animate-spin" />
                  正在读取 transcript...
                </div>
              ) : (
                <TaskTranscript detail={detail} />
              )}
              {selected_task && !is_terminal_status(selected_task.status) ? (
                <form className="mt-3 flex items-end gap-2 rounded-[8px] border border-(--divider-subtle-color) bg-(--surface-elevated-background) p-2" onSubmit={handle_send_task_message}>
                  <textarea
                    className="min-h-[64px] flex-1 resize-none rounded-[7px] border border-(--divider-subtle-color) bg-(--surface-canvas-background) px-3 py-2 text-sm leading-5 text-(--text-default) outline-none placeholder:text-(--text-muted) focus:border-(--primary)"
                    disabled={is_sending_message}
                    onChange={(event) => set_task_message(event.target.value)}
                    placeholder="给这个 subagent 发送后续消息"
                    value={task_message}
                  />
                  <button
                    className="inline-flex h-9 w-9 items-center justify-center rounded-[7px] bg-(--primary) text-(--primary-foreground) disabled:cursor-not-allowed disabled:opacity-(--disabled-opacity)"
                    disabled={is_sending_message || task_message.trim() === ""}
                    type="submit"
                    title="发送"
                  >
                    {is_sending_message ? (
                      <Loader2 className="h-4 w-4 animate-spin" />
                    ) : (
                      <Send className="h-4 w-4" />
                    )}
                  </button>
                </form>
              ) : null}
            </section>
          </UiDialogBody>
        </UiDialogShell>
      </UiDialogBackdrop>
    </UiDialogPortal>
  );
}
