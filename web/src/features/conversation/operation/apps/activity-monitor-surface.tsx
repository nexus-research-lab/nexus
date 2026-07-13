import {
  AlertTriangle,
  CheckCircle2,
  CircleHelp,
  Clock3,
  Cpu,
  HardDrive,
  Loader2,
  Search,
} from "lucide-react";
import type { LucideIcon } from "lucide-react";
import type { ReactNode } from "react";

import { cn } from "@/shared/ui/class-name";
import { useState } from "react";

import { formatOperationTime } from "../operation-preview";
import type {
  NexusOperationEvent,
  NexusOperationSnapshot,
  OperationPhase,
} from "../operation-types";
import { PHASE_LABELS } from "../operation-tool-catalog";
import {
  activityCpuLabel,
  activityCpuLoad,
  activityPidLabel,
} from "./activity-monitor-data";

const PHASE_LABEL: Record<OperationPhase, string> = {
  queued: "排队中",
  running: "执行中",
  waiting: "等待确认",
  done: "已完成",
  error: "失败",
  cancelled: "已中断",
};

export function ActivityMonitorSurface({
  event,
  snapshot,
  lines,
  onFocusEvent,
}: {
  event: NexusOperationEvent;
  snapshot: NexusOperationSnapshot | null;
  lines: string[];
  onFocusEvent?: (event: NexusOperationEvent) => void;
}) {
  const [selected_event_id, set_selected_event_id] = useState<string | null>(null);
  const task_events = collect_task_events(event, snapshot);
  const steps = task_events.map((item, index) => ({
    event: item,
    label: item.target ?? item.summary ?? item.tool_name ?? `进程 ${index + 1}`,
    status: PHASE_LABEL[item.phase],
  }));
  const active_index = Math.max(0, steps.findIndex((step) => step.event.id === event.id));
  const selected_index = selected_event_id
    ? steps.findIndex((step) => step.event.id === selected_event_id)
    : -1;
  const inspected_index = selected_index >= 0 ? selected_index : active_index;
  const preview_value = lines.join("\n") || event.result_preview || event.input_preview || event.summary;
  const finished_count = task_events.filter((item) => item.phase === "done").length;
  const running_count = task_events.filter((item) => item.phase === "running" || item.phase === "waiting").length;
  const cpu_load = activityCpuLoad(running_count, finished_count);
  const active_step = steps[inspected_index] ?? steps[0];

  return (
    <div className="flex h-full min-h-[320px] min-w-0 max-w-full overflow-hidden bg-[#f7f9fb] max-md:flex-col">
      <section className="flex min-h-0 w-[260px] shrink-0 flex-col overflow-hidden border-r border-(--divider-subtle-color) bg-white/62 max-md:w-full max-md:border-b max-md:border-r-0">
        <div className="border-b border-(--divider-subtle-color) bg-white/72 px-3 py-2.5">
          <div className="flex min-w-0 items-center justify-between gap-2">
            <div className="min-w-0">
              <p className="truncate text-[12px] font-black text-(--text-strong)">活动监视器</p>
              <p className="mt-0.5 truncate text-[10px] text-(--text-soft)">
                {running_count ? `${running_count} 个进程活跃` : `${finished_count}/${Math.max(task_events.length, 1)} 已完成`}
              </p>
            </div>
            <div className="flex shrink-0 items-center gap-1">
              <ActivityMonitorButton label="CPU" active>
                <Cpu className="h-3.5 w-3.5" />
              </ActivityMonitorButton>
              <ActivityMonitorButton label="Memory">
                <HardDrive className="h-3.5 w-3.5" />
              </ActivityMonitorButton>
            </div>
          </div>
          <div className="mt-2 flex min-w-0 items-center gap-1.5 rounded-[8px] border border-(--divider-subtle-color) bg-white/72 px-2 py-1 text-[10px] text-(--text-soft)">
            <Search className="h-3 w-3 shrink-0" />
            <span className="truncate">过滤进程</span>
          </div>
          <div className="mt-2 grid grid-cols-[minmax(0,1fr)_42px_38px] gap-2 px-1 text-[9px] font-bold uppercase tracking-[0.10em] text-(--text-soft)">
            <span>进程名称</span>
            <span>进程 ID</span>
            <span>%CPU</span>
          </div>
        </div>
        <div className="soft-scrollbar min-h-0 flex-1 overflow-auto p-1.5">
          {steps.map((step, index) => {
            const Icon = icon_for_task_phase(step.event.phase);
            const active = index === active_index;
            const pid_label = activityPidLabel(step.event.id);
            const cpu_label = activityCpuLabel(step.event.phase, index);
            return (
              <button
                aria-label={`查看进程 ${step.label}`}
                className={cn(
                  "mb-1 grid w-full min-w-0 grid-cols-[22px_minmax(0,1fr)_42px_38px] items-center gap-2 rounded-[9px] px-2 py-1.5 text-left text-[11px] transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[rgba(91,114,255,0.32)]",
                  active
                    ? "bg-[rgba(91,114,255,0.10)] text-(--text-strong)"
                    : "text-(--text-muted) hover:bg-white/64",
                )}
                key={step.event.id}
                onClick={() => set_selected_event_id(step.event.id)}
                type="button"
              >
                <span className={cn(
                  "grid h-5 w-5 shrink-0 place-items-center rounded-[7px]",
                  step.event.phase === "done" && "bg-[rgba(47,184,132,0.12)] text-[color:var(--success)]",
                  step.event.phase === "running" && "bg-[rgba(91,114,255,0.12)] text-[color:var(--primary)]",
                  step.event.phase === "waiting" && "bg-[rgba(223,157,46,0.12)] text-[color:var(--warning)]",
                  (step.event.phase === "error" || step.event.phase === "cancelled") && "bg-[rgba(223,93,98,0.12)] text-[color:var(--destructive)]",
                  step.event.phase === "queued" && "bg-white/70 text-(--icon-muted)",
                )}>
                  <Icon className={cn("h-3.5 w-3.5", step.event.phase === "running" && "animate-spin")} />
                </span>
                <div className="min-w-0 flex-1">
                  <p className="truncate font-bold">{step.label}</p>
                  <p className="mt-0.5 truncate text-[10px] text-(--text-soft)">
                    {step.status} · {formatOperationTime(step.event.updated_at)}
                  </p>
                </div>
                <span className="shrink-0 font-mono text-[9px] text-(--text-soft)">{pid_label}</span>
                <span className={cn(
                  "shrink-0 rounded px-1 py-px text-right font-mono text-[9px]",
                  step.event.phase === "running"
                    ? "bg-[rgba(91,114,255,0.10)] text-[color:var(--primary)]"
                    : "text-(--text-soft)",
                )}>
                  {cpu_label}
                </span>
              </button>
            );
          })}
        </div>
        <div className="border-t border-(--divider-subtle-color) bg-white/58 p-2 text-[10px] text-(--text-muted)">
          <div className="mb-1 flex items-center justify-between gap-2">
            <span className="font-black text-(--text-strong)">CPU 负载</span>
            <span className="font-mono">{running_count ? "活跃" : "空闲"}</span>
          </div>
          <div className="h-2 overflow-hidden rounded-full bg-[rgba(117,131,149,0.14)]">
            <span
              className="block h-full rounded-full bg-[linear-gradient(90deg,rgba(47,184,132,0.86),rgba(91,114,255,0.74))]"
              style={{ width: `${cpu_load.total}%` }}
            />
          </div>
          <div className="mt-2 grid grid-cols-3 gap-1">
            <ActivityMetric label="系统" value={`${cpu_load.system}%`} />
            <ActivityMetric label="用户" value={`${cpu_load.user}%`} />
            <ActivityMetric label="进程" value={task_events.length} />
          </div>
        </div>
      </section>
      <ActivityProcessInspector
        activeIndex={inspected_index}
        onFocusEvent={onFocusEvent}
        previewValue={preview_value}
        step={active_step}
      />
    </div>
  );
}

function ActivityMetric({ label, value }: { label: string; value: number | string }) {
  return (
    <span className="min-w-0 truncate rounded-[8px] bg-white/64 px-2 py-1.5">
      {label} <span className="font-mono font-black text-(--text-strong)">{value}</span>
    </span>
  );
}

function collect_task_events(
  event: NexusOperationEvent,
  snapshot: NexusOperationSnapshot | null,
): NexusOperationEvent[] {
  const events = (snapshot?.events ?? [])
    .filter((item) => item.surface === "task" || item.kind === "task_delegate" || item.kind === "task_progress")
    .filter((item) => item.round_id === event.round_id || item.id === event.id);
  if (!events.some((item) => item.id === event.id)) {
    events.push(event);
  }
  return events
    .sort((a, b) => (a.started_at ?? a.updated_at) - (b.started_at ?? b.updated_at))
    .slice(-8);
}

function ActivityMonitorButton({
  active = false,
  children,
  label,
}: {
  active?: boolean;
  children: ReactNode;
  label: string;
}) {
  return (
    <button
      aria-label={label}
      className={cn(
        "grid h-7 w-7 place-items-center rounded-[8px] border transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[rgba(91,114,255,0.32)]",
        active
          ? "border-(--divider-subtle-color) bg-white text-(--text-strong)"
          : "border-transparent bg-white/42 text-(--icon-muted) hover:bg-white/76 hover:text-(--text-strong)",
      )}
      title={label}
      type="button"
    >
      {children}
    </button>
  );
}

function icon_for_task_phase(phase: OperationPhase): LucideIcon {
  if (phase === "done") {
    return CheckCircle2;
  }
  if (phase === "running") {
    return Loader2;
  }
  if (phase === "waiting") {
    return CircleHelp;
  }
  if (phase === "error" || phase === "cancelled") {
    return AlertTriangle;
  }
  return Clock3;
}

function ActivityProcessInspector({
  activeIndex,
  onFocusEvent,
  previewValue,
  step,
}: {
  activeIndex: number;
  onFocusEvent?: (event: NexusOperationEvent) => void;
  previewValue: unknown;
  step?: { event: NexusOperationEvent; label: string; status: string };
}) {
  if (!step) {
    return (
      <section className="grid min-h-0 min-w-0 flex-1 place-items-center bg-white/74 text-[12px] text-(--text-soft)">
        没有活动进程
      </section>
    );
  }

  const output_lines = format_process_output(previewValue);
  const cpu_label = activityCpuLabel(step.event.phase, activeIndex);
  const pid_label = activityPidLabel(step.event.id);

  return (
    <section className="soft-scrollbar min-h-0 min-w-0 flex-1 overflow-auto bg-white/74 p-4">
      <div className="mb-4 flex items-start justify-between gap-3 border-b border-(--divider-subtle-color) pb-3">
        <div className="min-w-0">
          <p className="text-[10px] font-black uppercase tracking-[0.14em] text-(--text-soft)">进程检查器</p>
          <h3 className="mt-1 truncate text-[15px] font-black tracking-[-0.03em] text-(--text-strong)">
            {step.label}
          </h3>
          <p className="mt-1 truncate text-[11px] text-(--text-soft)">
            PID {pid_label} · {step.status}
          </p>
        </div>
        <div className="flex shrink-0 items-center gap-2">
          <span className={cn(
            "rounded-[9px] px-2.5 py-1 text-[10px] font-black",
            step.event.phase === "running" || step.event.phase === "waiting"
              ? "bg-[rgba(91,114,255,0.10)] text-[color:var(--primary)]"
              : "bg-white/70 text-(--text-soft)",
          )}>
            {cpu_label}% CPU
          </span>
          {onFocusEvent ? (
            <button
              className="h-7 rounded-[9px] border border-(--divider-subtle-color) bg-white/76 px-2.5 text-[10px] font-black text-(--text-strong) transition hover:bg-white focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[rgba(91,114,255,0.32)]"
              onClick={() => onFocusEvent(step.event)}
              type="button"
            >
              跳到窗口
            </button>
          ) : null}
        </div>
      </div>

      <div className="grid grid-cols-3 gap-2 text-[10px] max-sm:grid-cols-1">
        <ActivityDetailTile label="状态" value={PHASE_LABELS[step.event.phase]} />
        <ActivityDetailTile label="更新" value={formatOperationTime(step.event.updated_at)} />
        <ActivityDetailTile label="来源" value={step.event.tool_name ?? step.event.surface} />
      </div>

      <div className="mt-4 overflow-hidden rounded-[13px] border border-(--divider-subtle-color) bg-[#101820] p-3 font-mono text-[11px] leading-5 text-[#dce8ee]">
        <div className="mb-2 flex items-center justify-between border-b border-white/10 pb-2 text-[10px] text-[#8aa0ad]">
          <span>输出</span>
          <span>{output_lines.length} 行</span>
        </div>
        {(output_lines.length ? output_lines : ["等待进程输出..."]).map((line, index) => (
          <div className="flex min-w-0 gap-2" key={`${index}:${line}`}>
            <span className="w-6 shrink-0 select-none text-right text-[#6f8190]">{index + 1}</span>
            <span className="min-w-0 whitespace-pre-wrap break-words">{line}</span>
          </div>
        ))}
      </div>
    </section>
  );
}

function ActivityDetailTile({ label, value }: { label: string; value: string }) {
  return (
    <div className="min-w-0 rounded-[11px] border border-(--divider-subtle-color) bg-white/68 px-2.5 py-2">
      <p className="text-[9px] font-black uppercase tracking-[0.12em] text-(--text-soft)">{label}</p>
      <p className="mt-1 truncate font-mono text-[10px] font-black text-(--text-strong)" title={value}>{value}</p>
    </div>
  );
}

function format_process_output(value: unknown): string[] {
  if (value == null) {
    return [];
  }
  const text = typeof value === "string" ? value : JSON.stringify(value, null, 2);
  return text
    .replace(/\r\n/g, "\n")
    .replace(/\r/g, "\n")
    .split("\n")
    .map((line) => line.trimEnd())
    .filter((line) => line.trim())
    .slice(0, 12);
}
