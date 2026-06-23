"use client";

import { Bot, GaugeCircle, Loader2, Wrench } from "lucide-react";
import { useEffect, useMemo, useState } from "react";

import { cn, format_tokens } from "@/lib/utils";
import type {
  Message,
  SystemMessage,
  TaskProgressContent,
  Usage,
} from "@/types/conversation/message";

interface SubagentStatusStripProps {
  compact?: boolean;
  live_round_ids: string[];
  messages: Message[];
}

interface SubagentStatusItem {
  task_id: string;
  description: string;
  duration_ms?: number;
  last_tool_name?: string | null;
  output_file?: string | null;
  status?: string | null;
  task_type?: string | null;
  timestamp: number;
  total_tokens?: number;
  tool_uses?: number;
}

const TERMINAL_RETAIN_MS = 8000;

function is_live_round(round_id: string, live_round_ids: string[]): boolean {
  return live_round_ids.some((live_round_id) => (
    round_id === live_round_id ||
    round_id.startsWith(`${live_round_id}:`) ||
    live_round_id.startsWith(`${round_id}:`)
  ));
}

function number_from_usage(usage: Usage | Record<string, any> | undefined, key: string): number | undefined {
  const value = usage?.[key];
  return typeof value === "number" && value > 0 ? value : undefined;
}

function upsert_task(
  tasks: Map<string, SubagentStatusItem>,
  task_id: string,
  patch: Partial<SubagentStatusItem>,
): void {
  const current = tasks.get(task_id);
  tasks.set(task_id, {
    task_id,
    description: patch.description ?? current?.description ?? "子 Agent 正在执行",
    duration_ms: patch.duration_ms ?? current?.duration_ms,
    last_tool_name: patch.last_tool_name ?? current?.last_tool_name ?? null,
    output_file: patch.output_file ?? current?.output_file ?? null,
    status: patch.status ?? current?.status ?? "running",
    task_type: patch.task_type ?? current?.task_type ?? null,
    timestamp: Math.max(patch.timestamp ?? 0, current?.timestamp ?? 0),
    total_tokens: patch.total_tokens ?? current?.total_tokens,
    tool_uses: patch.tool_uses ?? current?.tool_uses,
  });
}

function extract_system_task_event(message: SystemMessage): Partial<SubagentStatusItem> | null {
  if (message.metadata?.subtype !== "task_started" && message.metadata?.subtype !== "task_notification") {
    return null;
  }
  const task_id =
    typeof message.metadata.task_id === "string" && message.metadata.task_id.trim()
      ? message.metadata.task_id.trim()
      : typeof message.metadata.tool_use_id === "string" && message.metadata.tool_use_id.trim()
        ? message.metadata.tool_use_id.trim()
        : message.message_id;
  const usage = typeof message.metadata.usage === "object" && message.metadata.usage
    ? message.metadata.usage as Record<string, any>
    : undefined;
  if (message.metadata.subtype === "task_notification") {
    return {
      task_id,
      description: message.content.trim() || "子 Agent 状态已更新",
      duration_ms: number_from_usage(usage, "duration_ms"),
      output_file:
        typeof message.metadata.output_file === "string"
          ? message.metadata.output_file
          : null,
      status:
        typeof message.metadata.status === "string"
          ? message.metadata.status
          : "completed",
      timestamp: message.timestamp,
      total_tokens: number_from_usage(usage, "total_tokens"),
      tool_uses: number_from_usage(usage, "tool_uses"),
    };
  }
  return {
    task_id,
    description: message.content.trim() || "子 Agent 已启动",
    status: "running",
    task_type:
      typeof message.metadata.task_type === "string"
        ? message.metadata.task_type
        : null,
    timestamp: message.timestamp,
  };
}

function extract_subagent_statuses(
  messages: Message[],
  live_round_ids: string[],
): SubagentStatusItem[] {
  if (live_round_ids.length === 0) {
    return [];
  }
  const tasks = new Map<string, SubagentStatusItem>();
  for (const message of messages) {
    if (!is_live_round(message.round_id, live_round_ids)) {
      continue;
    }
    if (message.role === "system") {
      const event = extract_system_task_event(message);
      if (event?.task_id) {
        upsert_task(tasks, event.task_id, event);
      }
      continue;
    }
    if (message.role !== "assistant") {
      continue;
    }
    for (const block of message.content) {
      if (block.type !== "task_progress") {
        continue;
      }
      const progress = block as TaskProgressContent;
      upsert_task(tasks, progress.task_id, {
        description: progress.description,
        duration_ms: number_from_usage(progress.usage, "duration_ms"),
        last_tool_name: progress.last_tool_name,
        status: "running",
        timestamp: message.timestamp,
        total_tokens: number_from_usage(progress.usage, "total_tokens"),
        tool_uses: number_from_usage(progress.usage, "tool_uses"),
      });
    }
  }
  return Array.from(tasks.values())
    .filter((task) => task.task_id.trim())
    .sort((left, right) => left.timestamp - right.timestamp);
}

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

function elapsed_label(duration_ms?: number): string | null {
  if (!duration_ms || duration_ms <= 0) {
    return null;
  }
  const seconds = Math.max(1, Math.round(duration_ms / 1000));
  if (seconds < 60) {
    return `${seconds}s`;
  }
  return `${Math.floor(seconds / 60)}m ${seconds % 60}s`;
}

function metric_label(task: SubagentStatusItem): string | null {
  const parts = [
    task.total_tokens ? format_tokens(task.total_tokens) : null,
    task.tool_uses ? `${task.tool_uses} tools` : null,
    elapsed_label(task.duration_ms),
  ].filter(Boolean);
  return parts.length ? parts.join(" · ") : null;
}

export function SubagentStatusStrip({
  compact = false,
  live_round_ids,
  messages,
}: SubagentStatusStripProps) {
  const live_tasks = useMemo(
    () => extract_subagent_statuses(messages, live_round_ids),
    [live_round_ids, messages],
  );
  const [retained_tasks, set_retained_tasks] = useState<SubagentStatusItem[]>([]);
  const [retained_until, set_retained_until] = useState(0);

  useEffect(() => {
    if (live_tasks.length > 0) {
      set_retained_tasks(live_tasks);
      set_retained_until(
        live_tasks.every((task) => is_terminal_status(task.status))
          ? Date.now() + TERMINAL_RETAIN_MS
          : 0,
      );
      return;
    }
    if (retained_tasks.length === 0) {
      return;
    }
    const expires_at = retained_until || Date.now() + TERMINAL_RETAIN_MS;
    if (retained_until === 0) {
      set_retained_until(expires_at);
    }
    const delay = Math.max(0, expires_at - Date.now());
    const timeout_id = window.setTimeout(() => {
      set_retained_tasks([]);
      set_retained_until(0);
    }, delay);
    return () => window.clearTimeout(timeout_id);
  }, [live_tasks, retained_tasks.length, retained_until]);

  const tasks = live_tasks.length > 0 ? live_tasks : retained_tasks;

  if (tasks.length === 0) {
    return null;
  }

  return (
    <div
      className={cn(
        "shrink-0 border-t border-(--divider-subtle-color) px-2 py-2",
        compact ? "px-2" : "px-4 sm:px-6 xl:px-8",
      )}
    >
      <div className="mx-auto flex w-full max-w-[980px] flex-col gap-1.5">
        {tasks.map((task) => {
          const metrics = metric_label(task);
          const is_running = !is_terminal_status(task.status);
          return (
            <div
              key={task.task_id}
              className="grid min-w-0 grid-cols-[28px_minmax(0,1fr)] items-center gap-2 rounded-[8px] border border-(--divider-subtle-color) bg-(--surface-elevated-background) px-2.5 py-2"
            >
              <span className={cn("flex h-7 w-7 items-center justify-center rounded-[7px]", status_class_name(task.status))}>
                <Bot className="h-3.5 w-3.5" />
              </span>
              <div className="min-w-0">
                <div className="flex min-w-0 items-center gap-1.5 text-[11px] font-medium text-(--text-muted)">
                  {is_running ? (
                    <Loader2 className="h-3 w-3 shrink-0 animate-spin text-primary" />
                  ) : null}
                  <span className="shrink-0">Subagent</span>
                  <span className={cn("shrink-0 rounded-[6px] px-1.5 py-0.5 text-[10px] font-semibold", status_class_name(task.status))}>
                    {status_label(task.status)}
                  </span>
                  {task.task_type ? (
                    <span className="truncate text-(--text-soft)">{task.task_type}</span>
                  ) : null}
                  {task.last_tool_name ? (
                    <span className="inline-flex min-w-0 items-center gap-1 truncate text-(--text-soft)">
                      <Wrench className="h-3 w-3 shrink-0" />
                      <span className="truncate">{task.last_tool_name}</span>
                    </span>
                  ) : null}
                  {metrics ? (
                    <span className="ml-auto hidden max-w-[180px] shrink-0 items-center gap-1 truncate text-(--text-soft) sm:inline-flex">
                      <GaugeCircle className="h-3 w-3 shrink-0" />
                      <span className="truncate">{metrics}</span>
                    </span>
                  ) : null}
                </div>
                <div className="mt-0.5 truncate text-[13px] font-medium leading-5 text-(--text-strong)">
                  {task.description}
                </div>
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}
