"use client";

import { Clock3, History, Pencil, Play, Trash2 } from "lucide-react";

import { WorkspaceStatusBadge } from "@/shared/ui/workspace/workspace-status-badge";
import {
  WorkspaceCatalogAction,
  WorkspaceCatalogTextAction,
} from "@/shared/ui/workspace/workspace-catalog-card";
import type {
  ScheduledTaskDeliveryTarget,
  ScheduledTaskItem,
  ScheduledTaskSchedule,
  ScheduledTaskSource,
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
  if (target.kind === "main") {
    return "主会话";
  }
  if (target.kind === "bound") {
    return "使用现有会话";
  }
  if (target.kind === "named") {
    return `专用长期会话 · ${target.named_session_key}`;
  }
  return "每次新建临时会话";
}

function get_source_kind_label(source: ScheduledTaskSource | null | undefined): string {
  if (!source) {
    return "未知来源";
  }
  if (source.kind === "user_page") {
    return "页面创建";
  }
  if (source.kind === "agent") {
    return "智能体创建";
  }
  if (source.kind === "cli") {
    return "CLI 创建";
  }
  return "系统创建";
}

function get_delivery_summary(
  delivery: ScheduledTaskDeliveryTarget,
  source: ScheduledTaskSource | null | undefined,
): string {
  if (delivery.mode === "none") {
    return "不回传";
  }
  if (delivery.mode === "last") {
    return "回到最近会话";
  }
  if (delivery.channel === "websocket") {
    if (delivery.to && source?.session_key && delivery.to === source.session_key) {
      return "回到当前选择的会话";
    }
    return "回到指定会话";
  }
  return "回到指定位置";
}

function get_context_summary(task: ScheduledTaskItem): string {
  const source = task.source;
  if (source?.context_type === "room" && source.context_label) {
    return `Room：${source.context_label}`;
  }
  if (source?.context_type === "agent" && source.context_label) {
    return `智能体：${source.context_label}`;
  }
  return `智能体：${task.agent_id}`;
}

function get_session_summary(task: ScheduledTaskItem): string {
  const source = task.source;
  if (source?.session_label) {
    return source.session_label;
  }
  return get_session_target_summary(task.session_target);
}

function is_same_session_loop(task: ScheduledTaskItem): boolean {
  return Boolean(
    task.session_target.kind === "bound"
      && task.delivery.mode === "explicit"
      && task.delivery.channel === "websocket"
      && task.delivery.to
      && task.source?.session_key
      && task.delivery.to === task.source.session_key,
  );
}

function get_behavior_summary(task: ScheduledTaskItem): string {
  if (is_same_session_loop(task)) {
    return "在当前会话里持续执行，并直接回到这条会话。";
  }
  if (task.session_target.kind === "bound") {
    return "复用一个已有会话执行；回复位置可单独指定。";
  }
  if (task.session_target.kind === "named") {
    return "固定使用一条专用长期会话执行，便于持续积累上下文。";
  }
  if (task.session_target.kind === "main") {
    return "交给主会话处理，适合把任务继续接在主线对话里。";
  }
  return "每次执行都会新开一条临时会话，不会复用旧上下文。";
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

function get_toggle_action(task: ScheduledTaskItem): {
  label: string;
  pending_label: string;
  class_name: string;
} {
  if (task.enabled) {
    return {
      label: "暂停",
      pending_label: "暂停中",
      class_name: "border border-[color:color-mix(in_srgb,var(--destructive)_28%,var(--divider-subtle-color))] bg-[color:color-mix(in_srgb,var(--destructive)_8%,transparent)] text-(--destructive) hover:bg-[color:color-mix(in_srgb,var(--destructive)_12%,transparent)]",
    };
  }
  return {
      label: "恢复",
      pending_label: "恢复中",
      class_name: "border border-[color:color-mix(in_srgb,var(--primary)_28%,var(--divider-subtle-color))] bg-[color:color-mix(in_srgb,var(--primary)_10%,transparent)] text-(--primary) hover:bg-[color:color-mix(in_srgb,var(--primary)_14%,transparent)]",
  };
}

function sort_tasks(items: ScheduledTaskItem[]): ScheduledTaskItem[] {
  return [...items].sort((left, right) => {
    const left_rank = left.running ? 0 : left.enabled ? 1 : 2;
    const right_rank = right.running ? 0 : right.enabled ? 1 : 2;
    if (left_rank !== right_rank) {
      return left_rank - right_rank;
    }
    const left_next_run = left.next_run_at ?? Number.MAX_SAFE_INTEGER;
    const right_next_run = right.next_run_at ?? Number.MAX_SAFE_INTEGER;
    if (left_next_run !== right_next_run) {
      return left_next_run - right_next_run;
    }
    return left.name.localeCompare(right.name, "zh-CN");
  });
}

interface ScheduledTaskListProps {
  items: ScheduledTaskItem[];
  is_loading: boolean;
  error_message: string | null;
  run_pending_job_id?: string | null;
  toggle_pending_job_id?: string | null;
  delete_pending_job_id?: string | null;
  on_create: () => void;
  on_refresh?: () => void | Promise<void>;
  on_run_now?: (task: ScheduledTaskItem) => void | Promise<void>;
  on_toggle_enabled?: (task: ScheduledTaskItem) => void | Promise<void>;
  on_delete?: (task: ScheduledTaskItem) => void | Promise<void>;
  on_edit?: (task: ScheduledTaskItem) => void;
  on_open_history?: (task: ScheduledTaskItem) => void;
}

export function ScheduledTaskList({
  items,
  is_loading,
  error_message,
  run_pending_job_id = null,
  toggle_pending_job_id = null,
  delete_pending_job_id = null,
  on_create,
  on_refresh,
  on_run_now,
  on_toggle_enabled,
  on_delete,
  on_edit,
  on_open_history,
}: ScheduledTaskListProps) {
  const sorted_items = sort_tasks(items);

  return (
    <section className="surface-card flex min-h-[360px] flex-col rounded-[22px] px-4 py-4">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <div className="flex items-center gap-2">
            <Clock3 className="h-4 w-4 text-(--icon-default)" />
            <div>
              <h2 className="text-[15px] font-semibold tracking-[-0.03em] text-(--text-strong)">
                任务清单
              </h2>
              <p className="text-xs text-(--text-default)">
                共 {items.length} 个任务，可查看任务落在哪个会话里执行，以及结果回到哪里。
              </p>
            </div>
          </div>
        </div>
      </div>

      <div className="soft-scrollbar mt-4 min-h-0 flex-1 overflow-y-auto">
        {is_loading ? (
          <div className="space-y-3">
            {Array.from({ length: 3 }).map((_, index) => (
              <div
                key={index}
                className="h-[132px] animate-pulse rounded-[16px] border border-(--divider-subtle-color)"
              />
            ))}
          </div>
        ) : error_message ? (
          <div className="flex min-h-[240px] flex-col items-center justify-center rounded-[18px] border border-[color:color-mix(in_srgb,var(--destructive)_15%,transparent)] px-5 text-center">
            <p className="text-sm font-semibold text-(--destructive)">任务列表加载失败</p>
            <p className="mt-2 max-w-md text-sm leading-6 text-(--text-default)">
              {error_message}
            </p>
            <WorkspaceCatalogTextAction class_name="mt-4" onClick={() => void on_refresh?.()} tone="primary">
              重试
            </WorkspaceCatalogTextAction>
          </div>
        ) : items.length === 0 ? (
          <div className="flex min-h-[240px] flex-col items-center justify-center rounded-[18px] border border-dashed border-(--divider-subtle-color) px-5 text-center">
            <div className="chip-default flex h-14 w-14 items-center justify-center rounded-[20px]">
              <Clock3 className="h-6 w-6 text-(--icon-strong)" />
            </div>
            <h3 className="mt-5 text-lg font-bold tracking-[-0.03em] text-(--text-strong)">
              还没有定时任务
            </h3>
            <p className="mt-2 max-w-sm text-sm leading-6 text-(--text-default)">
              新建第一个自动化任务后，这里会显示任务在哪个会话里执行、结果回到哪里，以及最近运行情况。
            </p>
            <WorkspaceCatalogTextAction class_name="mt-4" onClick={on_create} tone="primary">
              新建任务
            </WorkspaceCatalogTextAction>
          </div>
        ) : (
          <div className="divide-y divide-(--divider-subtle-color)">
            {sorted_items.map((task) => {
              const status = get_primary_status(task);
              const toggle_action = get_toggle_action(task);
              const run_pending = run_pending_job_id === task.job_id;
              const toggle_pending = toggle_pending_job_id === task.job_id;
              const delete_pending = delete_pending_job_id === task.job_id;
              return (
                <article
                  key={task.job_id}
                  className="py-4 first:pt-0 last:pb-0"
                >
                  <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
                    <div className="min-w-0 flex-1">
                      <div className="flex flex-wrap items-center gap-2">
                        <h3 className="text-[15px] font-semibold text-(--text-strong)">
                          {task.name}
                        </h3>
                        <WorkspaceStatusBadge label={status.label} size="compact" tone={status.tone} />
                        {task.running ? (
                          <WorkspaceStatusBadge label="执行占用中" size="compact" tone="running" />
                        ) : null}
                      </div>
                      <div className="mt-3 grid gap-4 text-sm text-(--text-default) md:grid-cols-2">
                        <div className="min-w-0">
                          <p className="text-[11px] font-semibold uppercase tracking-[0.18em] text-(--text-muted)">
                            归属对象
                          </p>
                          <p className="mt-1.5 font-medium text-(--text-strong)">
                            {get_context_summary(task)}
                          </p>
                        </div>
                        <div className="min-w-0">
                          <p className="text-[11px] font-semibold uppercase tracking-[0.18em] text-(--text-muted)">
                            执行会话
                          </p>
                          <p className="mt-1.5 font-medium text-(--text-strong)">
                            {get_session_summary(task)}
                          </p>
                        </div>
                        <div className="min-w-0">
                          <p className="text-[11px] font-semibold uppercase tracking-[0.18em] text-(--text-muted)">
                            结果回传
                          </p>
                          <p className="mt-1.5 font-medium text-(--text-strong)">
                            {get_delivery_summary(task.delivery, task.source)}
                          </p>
                        </div>
                        <div className="min-w-0">
                          <p className="text-[11px] font-semibold uppercase tracking-[0.18em] text-(--text-muted)">
                            调度规则
                          </p>
                          <p className="mt-1.5 font-medium text-(--text-strong)">
                            {get_schedule_summary(task.schedule)}
                          </p>
                        </div>
                      </div>
                      <div className="mt-3 flex flex-wrap gap-x-5 gap-y-2 text-xs text-(--text-default)">
                        <span>下次运行 {format_datetime(task.next_run_at)}</span>
                        <span>最近执行 {format_datetime(task.last_run_at)}</span>
                        <span>Agent {task.agent_id}</span>
                        <span>来源 {get_source_kind_label(task.source)}</span>
                      </div>
                      <p className="mt-3 text-sm leading-6 text-(--text-default)">
                        {get_behavior_summary(task)}
                      </p>
                    </div>

                    <div className="flex shrink-0 flex-wrap items-center gap-3 lg:justify-end">
                      <div className="flex items-center justify-end gap-2">
                        <button
                          className={[
                            "inline-flex h-9 min-w-[92px] items-center justify-center rounded-[8px] px-3 text-sm font-semibold transition duration-(--motion-duration-fast) ease-out",
                            toggle_action.class_name,
                            toggle_pending ? "opacity-70" : "",
                          ].join(" ")}
                          disabled={toggle_pending}
                          onClick={() => void on_toggle_enabled?.(task)}
                          title={task.enabled ? "暂停后不会再按计划自动触发" : "恢复后会重新参与调度"}
                          type="button"
                        >
                          {toggle_pending ? toggle_action.pending_label : toggle_action.label}
                        </button>
                        <WorkspaceCatalogAction
                          aria-label="立即运行"
                          disabled={run_pending || task.running}
                          onClick={() => void on_run_now?.(task)}
                          size="md"
                          title={task.running ? "任务当前已经在运行中" : "立即触发一次执行"}
                        >
                          <Play className="h-3.5 w-3.5" />
                        </WorkspaceCatalogAction>
                        <WorkspaceCatalogAction
                          aria-label="运行历史"
                          onClick={() => on_open_history?.(task)}
                          size="md"
                          title="查看最近几次执行记录"
                        >
                          <History className="h-3.5 w-3.5" />
                        </WorkspaceCatalogAction>
                        <WorkspaceCatalogAction
                          aria-label="编辑任务"
                          onClick={() => on_edit?.(task)}
                          size="md"
                          title="编辑任务"
                        >
                          <Pencil className="h-3.5 w-3.5" />
                        </WorkspaceCatalogAction>
                        <WorkspaceCatalogAction
                          aria-label="删除任务"
                          disabled={delete_pending}
                          onClick={() => void on_delete?.(task)}
                          size="md"
                          title="删除后任务会从列表里移除"
                          tone="danger"
                        >
                          <Trash2 className="h-3.5 w-3.5" />
                        </WorkspaceCatalogAction>
                      </div>
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
