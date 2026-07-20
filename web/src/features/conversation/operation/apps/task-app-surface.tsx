/**
 * INPUT: A task operation event and the real task events visible in its round.
 * OUTPUT: An interactive desktop Tasks app without fabricated process telemetry.
 * POS: Operation Stage Tasks presentation; task semantics live in task-app-model.
 */
import {
  AlertCircle,
  Ban,
  CheckCircle2,
  ChevronRight,
  Circle,
  CircleDashed,
  Clock3,
  Crosshair,
  ListChecks,
  ListTodo,
  Loader2,
  PauseCircle,
} from "lucide-react";
import type { LucideIcon } from "lucide-react";
import { useEffect, useMemo, useRef, useState } from "react";

import { cn } from "@/shared/ui/class-name";

import { formatOperationTime } from "../operation-preview";
import type { NexusOperationEvent } from "../operation-types";
import {
  buildTaskAppSession,
  type TaskAppItem,
  type TaskAppSection,
  type TaskAppState,
} from "./task-app-model";

export function TaskAppSurface({
  event,
  onFocusEvent,
  relatedEvents,
}: {
  event: NexusOperationEvent;
  onFocusEvent?: (event: NexusOperationEvent) => void;
  relatedEvents: NexusOperationEvent[];
}) {
  const root_ref = useRef<HTMLDivElement | null>(null);
  const session = useMemo(
    () => buildTaskAppSession(event, relatedEvents),
    [event, relatedEvents],
  );
  const [section, set_section] = useState<TaskAppSection>(session.active_section);
  const [selected_item_id, set_selected_item_id] = useState<string | null>(session.selected_item_id);
  const [container_width, set_container_width] = useState(0);
  const items = section === "plan" ? session.plan_items : session.task_items;
  const selected_item = items.find((item) => item.id === selected_item_id)
    ?? items.find((item) => item.state === "running" || item.state === "waiting")
    ?? items.at(-1)
    ?? null;
  const show_sidebar = container_width >= 620;
  const show_detail = container_width >= 760;

  useEffect(() => {
    set_section(session.active_section);
    set_selected_item_id(session.selected_item_id);
  }, [session.active_section, session.selected_item_id]);

  useEffect(() => {
    const element = root_ref.current;
    if (!element) return;
    const update_width = () => set_container_width(element.clientWidth);
    update_width();
    const observer = new ResizeObserver(update_width);
    observer.observe(element);
    return () => observer.disconnect();
  }, []);

  const select_section = (next: TaskAppSection) => {
    const next_items = next === "plan" ? session.plan_items : session.task_items;
    set_section(next);
    set_selected_item_id(next_items.find((item) => item.state === "running" || item.state === "waiting")?.id
      ?? next_items.at(-1)?.id
      ?? null);
  };

  return (
    <div ref={root_ref} className="flex h-full min-h-[300px] min-w-0 overflow-hidden bg-[#f7f8fa] text-(--text-default)">
      {show_sidebar ? (
        <TaskSidebar
          activeSection={section}
          modeLabel={session.mode_label}
          onSelect={select_section}
          planCount={session.plan_items.length}
          taskCount={session.task_items.length}
        />
      ) : null}
      <section className="flex min-h-0 min-w-0 flex-1 flex-col bg-white/80">
        <TaskToolbar
          activeSection={section}
          compact={!show_sidebar}
          count={items.length}
          onSelect={select_section}
          planCount={session.plan_items.length}
          taskCount={session.task_items.length}
        />
        <div className={cn(
          "grid min-h-0 flex-1",
          show_detail ? "grid-cols-[minmax(250px,0.92fr)_minmax(270px,1.08fr)]" : "grid-cols-1",
        )}>
          <TaskList
            items={items}
            onSelect={set_selected_item_id}
            selectedItemId={selected_item?.id ?? null}
          />
          {show_detail ? (
            <TaskInspector item={selected_item} onFocusEvent={onFocusEvent} />
          ) : null}
        </div>
      </section>
    </div>
  );
}

function TaskSidebar({
  activeSection,
  modeLabel,
  onSelect,
  planCount,
  taskCount,
}: {
  activeSection: TaskAppSection;
  modeLabel: string | null;
  onSelect: (section: TaskAppSection) => void;
  planCount: number;
  taskCount: number;
}) {
  return (
    <aside className="flex w-[168px] shrink-0 flex-col border-r border-(--divider-subtle-color) bg-[#edf1f5]/92 p-2.5">
      <div className="px-2 pb-2 pt-1">
        <p className="text-[12px] font-black text-(--text-strong)">任务</p>
        {modeLabel ? <p className="mt-0.5 text-[10px] text-(--text-soft)">{modeLabel}</p> : null}
      </div>
      <TaskSidebarButton
        active={activeSection === "plan"}
        count={planCount}
        icon={ListTodo}
        label="计划"
        onClick={() => onSelect("plan")}
      />
      <TaskSidebarButton
        active={activeSection === "tasks"}
        count={taskCount}
        icon={ListChecks}
        label="子任务"
        onClick={() => onSelect("tasks")}
      />
    </aside>
  );
}

function TaskSidebarButton({
  active,
  count,
  icon: Icon,
  label,
  onClick,
}: {
  active: boolean;
  count: number;
  icon: LucideIcon;
  label: string;
  onClick: () => void;
}) {
  return (
    <button
      className={cn(
        "mb-1 flex h-8 w-full items-center gap-2 rounded-[7px] px-2 text-left text-[11px] font-semibold transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[rgba(91,114,255,0.3)]",
        active ? "bg-white/88 text-(--text-strong) shadow-[0_1px_3px_rgba(15,23,42,0.08)]" : "text-(--text-muted) hover:bg-white/54",
      )}
      onClick={onClick}
      type="button"
    >
      <Icon className={cn("h-3.5 w-3.5", active ? "text-[color:var(--primary)]" : "text-(--icon-muted)")} />
      <span className="min-w-0 flex-1 truncate">{label}</span>
      <span className="font-mono text-[9px] text-(--text-soft)">{count}</span>
    </button>
  );
}

function TaskToolbar({
  activeSection,
  compact,
  count,
  onSelect,
  planCount,
  taskCount,
}: {
  activeSection: TaskAppSection;
  compact: boolean;
  count: number;
  onSelect: (section: TaskAppSection) => void;
  planCount: number;
  taskCount: number;
}) {
  return (
    <header className="flex min-h-12 items-center justify-between gap-3 border-b border-(--divider-subtle-color) bg-white/74 px-3 py-2">
      <div className="min-w-0">
        <h2 className="truncate text-[13px] font-black text-(--text-strong)">
          {activeSection === "plan" ? "计划" : "子任务"}
        </h2>
        <p className="mt-0.5 text-[9.5px] text-(--text-soft)">{count} 项</p>
      </div>
      {compact ? (
        <div aria-label="任务视图" className="flex rounded-[8px] bg-[#eef1f5] p-0.5" role="tablist">
          <TaskSegment active={activeSection === "plan"} count={planCount} label="计划" onClick={() => onSelect("plan")} />
          <TaskSegment active={activeSection === "tasks"} count={taskCount} label="子任务" onClick={() => onSelect("tasks")} />
        </div>
      ) : null}
    </header>
  );
}

function TaskSegment({ active, count, label, onClick }: {
  active: boolean;
  count: number;
  label: string;
  onClick: () => void;
}) {
  return (
    <button
      aria-selected={active}
      className={cn(
        "h-7 rounded-[6px] px-2.5 text-[10px] font-bold transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[rgba(91,114,255,0.3)]",
        active ? "bg-white text-(--text-strong) shadow-sm" : "text-(--text-soft)",
      )}
      onClick={onClick}
      role="tab"
      type="button"
    >
      {label} <span className="ml-1 font-mono text-[9px]">{count}</span>
    </button>
  );
}

function TaskList({
  items,
  onSelect,
  selectedItemId,
}: {
  items: TaskAppItem[];
  onSelect: (id: string) => void;
  selectedItemId: string | null;
}) {
  if (items.length === 0) {
    return (
      <div className="grid min-h-0 place-items-center px-6 text-center">
        <div>
          <ListTodo className="mx-auto h-7 w-7 text-(--icon-muted)" />
          <p className="mt-2 text-[11px] font-semibold text-(--text-muted)">当前没有任务条目</p>
        </div>
      </div>
    );
  }

  return (
    <div className="soft-scrollbar min-h-0 overflow-auto border-r border-(--divider-subtle-color) p-2">
      {items.map((item) => {
        const Icon = task_state_icon(item.state);
        return (
          <button
            aria-label={`查看任务 ${item.title}`}
            className={cn(
              "mb-1 flex w-full min-w-0 items-start gap-2.5 rounded-[8px] px-2.5 py-2 text-left transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[rgba(91,114,255,0.3)]",
              item.id === selectedItemId ? "bg-[rgba(91,114,255,0.1)]" : "hover:bg-[#f2f4f7]",
            )}
            key={item.id}
            onClick={() => onSelect(item.id)}
            type="button"
          >
            <Icon className={cn("mt-0.5 h-4 w-4 shrink-0", task_state_color(item.state), item.state === "running" && "animate-spin")} />
            <span className="min-w-0 flex-1 border-b border-(--divider-subtle-color) pb-2">
              <span className="flex min-w-0 items-center justify-between gap-2">
                <span className="line-clamp-2 text-[11px] font-bold leading-4 text-(--text-strong)">{item.title}</span>
                <ChevronRight className="h-3.5 w-3.5 shrink-0 text-(--icon-muted)" />
              </span>
              <span className="mt-1 flex min-w-0 items-center gap-1.5 text-[9.5px] text-(--text-soft)">
                <span className="shrink-0">{item.state_label}</span>
                {item.active_label && item.state === "running" ? <span className="truncate">{item.active_label}</span> : null}
                {item.task_id ? <span className="truncate font-mono">{short_task_id(item.task_id)}</span> : null}
              </span>
            </span>
          </button>
        );
      })}
    </div>
  );
}

function TaskInspector({
  item,
  onFocusEvent,
}: {
  item: TaskAppItem | null;
  onFocusEvent?: (event: NexusOperationEvent) => void;
}) {
  if (!item) {
    return <div className="grid min-h-0 place-items-center text-[11px] text-(--text-soft)">选择一个任务</div>;
  }
  const Icon = task_state_icon(item.state);
  return (
    <aside className="soft-scrollbar min-h-0 overflow-auto bg-[#fbfcfd] p-4">
      <div className="flex items-start gap-3 border-b border-(--divider-subtle-color) pb-3">
        <Icon className={cn("mt-0.5 h-5 w-5 shrink-0", task_state_color(item.state), item.state === "running" && "animate-spin")} />
        <div className="min-w-0 flex-1">
          <h3 className="break-words text-[13px] font-black leading-5 text-(--text-strong)">{item.title}</h3>
          <p className="mt-1 text-[10px] text-(--text-soft)">
            {item.state_label} · {formatOperationTime(item.updated_at)}
          </p>
        </div>
        {onFocusEvent ? (
          <button
            aria-label="定位到任务事件"
            className="grid h-7 w-7 shrink-0 place-items-center rounded-[7px] border border-(--divider-subtle-color) bg-white text-(--icon-default) transition hover:bg-[#f5f7fa] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[rgba(91,114,255,0.3)]"
            onClick={() => onFocusEvent(item.event)}
            title="定位到任务事件"
            type="button"
          >
            <Crosshair className="h-3.5 w-3.5" />
          </button>
        ) : null}
      </div>

      <div className="mt-3 grid gap-2 text-[10px]">
        {item.task_id ? <TaskDetail label="任务 ID" mono value={item.task_id} /> : null}
        {item.tool_name ? <TaskDetail label="来源" value={item.tool_name} /> : null}
        {item.last_tool_name ? <TaskDetail label="最近工具" value={item.last_tool_name} /> : null}
      </div>
      {item.prompt ? <TaskTextSection label="任务内容" value={item.prompt} /> : null}
      {item.output ? <TaskTextSection label="输出" value={item.output} /> : null}
      {item.usage.length > 0 ? (
        <section className="mt-4">
          <p className="mb-2 text-[9px] font-black text-(--text-soft)">用量</p>
          <div className="grid grid-cols-3 gap-1.5">
            {item.usage.map((usage) => (
              <div className="min-w-0 rounded-[8px] border border-(--divider-subtle-color) bg-white px-2 py-1.5" key={usage.label}>
                <p className="truncate text-[8.5px] text-(--text-soft)">{usage.label}</p>
                <p className="mt-0.5 truncate font-mono text-[10px] font-bold text-(--text-strong)">{usage.value}</p>
              </div>
            ))}
          </div>
        </section>
      ) : null}
    </aside>
  );
}

function TaskDetail({ label, mono = false, value }: { label: string; mono?: boolean; value: string }) {
  return (
    <div className="grid grid-cols-[70px_minmax(0,1fr)] gap-2">
      <span className="text-(--text-soft)">{label}</span>
      <span className={cn("min-w-0 break-all text-(--text-strong)", mono && "font-mono")}>{value}</span>
    </div>
  );
}

function TaskTextSection({ label, value }: { label: string; value: string }) {
  return (
    <section className="mt-4">
      <p className="mb-1.5 text-[9px] font-black text-(--text-soft)">{label}</p>
      <div className="max-h-40 overflow-auto whitespace-pre-wrap break-words rounded-[8px] border border-(--divider-subtle-color) bg-white p-2.5 text-[10.5px] leading-[1.55] text-(--text-muted)">
        {value}
      </div>
    </section>
  );
}

function task_state_icon(state: TaskAppState): LucideIcon {
  if (state === "completed") return CheckCircle2;
  if (state === "running") return Loader2;
  if (state === "waiting") return Clock3;
  if (state === "paused") return PauseCircle;
  if (state === "failed") return AlertCircle;
  if (state === "stopped") return Ban;
  if (state === "observed") return CircleDashed;
  return Circle;
}

function task_state_color(state: TaskAppState): string {
  if (state === "completed") return "text-[color:var(--success)]";
  if (state === "running") return "text-[color:var(--primary)]";
  if (state === "waiting" || state === "paused") return "text-[color:var(--warning)]";
  if (state === "failed" || state === "stopped") return "text-[color:var(--destructive)]";
  return "text-(--icon-muted)";
}

function short_task_id(task_id: string): string {
  return task_id.length > 12 ? `${task_id.slice(0, 8)}…` : task_id;
}
