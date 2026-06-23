import { useEffect, useMemo, useState } from "react";
import type { ReactNode } from "react";
import {
  ChevronDown,
  MousePointer2,
  Power,
} from "lucide-react";

import { cn } from "@/lib/utils";

import type { StageWindowState } from "../operation-desktop-types";
import {
  display_stage_event_target,
  display_stage_event_title,
} from "../operation-stage-labels";
import { resolve_operation_tool_profile } from "../operation-tool-catalog";
import type { NexusOperationEvent, OperationPhase } from "../operation-types";
import {
  icon_for_artifact_path,
  stage_app_label_for_window_kind,
} from "./operation-stage-window-meta";
import { build_stage_desktop_icon_items } from "./operation-stage-desktop-icons";
import { build_stage_menu_status } from "./operation-stage-menu-model";
import {
  agent_cursor_action_label,
  agent_cursor_anchor_class,
  agent_cursor_intent_for_window_kind,
} from "./operation-stage-agent-cursor";
import {
  event_sequence_label,
  icon_for_operation_kind,
} from "./operation-stage-helpers";

interface StageToolQueueItem {
  action_label: string;
  active: boolean;
  event: NexusOperationEvent;
  step_label: string;
  target: string;
  title: string;
}

export function StageMacMenuBar({
  active_event,
  active_window,
  events,
  header_action,
  on_focus_event,
  windows,
}: {
  active_event: NexusOperationEvent;
  active_window: StageWindowState | null;
  events: NexusOperationEvent[];
  header_action?: ReactNode;
  on_focus_event: (event: NexusOperationEvent) => void;
  windows: StageWindowState[];
}) {
  const app_name = active_window ? stage_app_label_for_window_kind(active_window.kind) : "Nexus";
  const menu_status = useMemo(() => (
    build_stage_menu_status(windows, active_window, (window) => stage_app_label_for_window_kind(window.kind))
  ), [active_window, windows]);
  const tool_queue_items = useMemo(() => (
    build_stage_tool_queue_items(events, active_event)
  ), [active_event, events]);
  const [current_time, set_current_time] = useState(() => new Date());
  const time_label = useMemo(() => (
    new Intl.DateTimeFormat("zh-CN", {
      hour: "2-digit",
      minute: "2-digit",
    }).format(current_time)
  ), [current_time]);

  useEffect(() => {
    const interval = window.setInterval(() => set_current_time(new Date()), 30_000);
    return () => window.clearInterval(interval);
  }, []);

  return (
    <div
      aria-label={menu_status.activity_label}
      className="absolute inset-x-4 top-3 z-[160] flex h-9 items-center justify-between rounded-[14px] border border-white/64 bg-[rgba(255,255,255,0.62)] px-3 text-[11px] font-semibold text-(--text-strong) shadow-[0_12px_30px_rgba(18,28,42,0.09),inset_0_1px_0_rgba(255,255,255,0.72)] backdrop-blur-2xl max-md:hidden"
      title={[
        menu_status.activity_label,
        menu_status.window_label,
        menu_status.dock_label,
      ].filter(Boolean).join(" · ")}
    >
      <div className="flex min-w-0 items-center gap-3">
        <span className="grid h-6 w-6 shrink-0 place-items-center rounded-[8px] bg-[rgba(20,28,38,0.88)] font-mono text-[9px] font-black text-white shadow-[0_8px_18px_rgba(18,28,42,0.16)]">
          NX
        </span>
        <span className="font-black">Nexus OS</span>
        <span className="h-4 w-px bg-[rgba(117,131,149,0.28)]" />
        <span className="max-w-[160px] truncate font-black">{app_name}</span>
      </div>
      <div className="flex shrink-0 items-center gap-2 text-(--text-soft)">
        <StageToolQueueMenu
          items={tool_queue_items}
          on_focus_event={on_focus_event}
        />
        <span className="hidden max-w-[220px] truncate rounded-full border border-white/66 bg-white/52 px-2 py-1 text-[9px] font-black text-(--text-strong) lg:inline">
          {menu_status.activity_label}
        </span>
        <span className="rounded-full border border-white/66 bg-white/44 px-2 py-1 text-[9px] font-bold">
          {menu_status.window_label}
        </span>
        {menu_status.dock_label ? (
          <span className="rounded-full border border-[rgba(223,157,46,0.24)] bg-[rgba(255,249,236,0.58)] px-2 py-1 text-[9px] font-bold text-[color:var(--warning)]">
            {menu_status.dock_label}
          </span>
        ) : null}
        <span className="font-mono text-[10px] text-(--text-strong)">{time_label}</span>
        {header_action ? (
          <StagePowerAction>{header_action}</StagePowerAction>
        ) : null}
      </div>
    </div>
  );
}

function StageToolQueueMenu({
  items,
  on_focus_event,
}: {
  items: StageToolQueueItem[];
  on_focus_event: (event: NexusOperationEvent) => void;
}) {
  const [is_open, set_is_open] = useState(false);

  if (!items.length) {
    return null;
  }

  return (
    <div className="relative">
      <button
        aria-expanded={is_open}
        aria-label="查看完整工具执行记录"
        className={cn(
          "inline-flex h-7 items-center gap-1.5 rounded-full border px-2.5 text-[9px] font-black transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[rgba(91,114,255,0.34)]",
          is_open
            ? "border-[rgba(91,114,255,0.26)] bg-[rgba(91,114,255,0.12)] text-[color:var(--primary)]"
            : "border-white/66 bg-white/44 text-(--text-strong) hover:bg-white/64",
        )}
        onClick={() => set_is_open((value) => !value)}
        type="button"
      >
        工具执行
        <span className="rounded-full bg-white/62 px-1.5 py-px text-[8px] text-(--text-soft)">
          {items.length}
        </span>
        <ChevronDown className={cn("h-3 w-3 transition-transform", is_open && "rotate-180")} />
      </button>

      {is_open ? (
        <div className="absolute right-0 top-[calc(100%+8px)] z-50 w-[360px] overflow-hidden rounded-[16px] border border-white/68 bg-[rgba(248,250,252,0.86)] p-2 text-(--text-strong) shadow-[0_22px_60px_rgba(18,28,42,0.18),inset_0_1px_0_rgba(255,255,255,0.76)] backdrop-blur-2xl">
          <div className="flex items-center justify-between gap-3 border-b border-[rgba(117,131,149,0.16)] px-1 pb-2">
            <span className="text-[11px] font-black">完整工具执行</span>
            <span className="rounded-full bg-white/62 px-2 py-0.5 text-[8px] font-black text-(--text-soft)">
              {items.length} 步
            </span>
          </div>
          <div className="soft-scrollbar mt-2 grid max-h-[360px] gap-1 overflow-auto pr-1">
            {items.map((item) => {
              const Icon = icon_for_operation_kind(item.event.kind);
              return (
                <button
                  aria-label={`查看${item.step_label}：${item.title}`}
                  className={cn(
                    "grid min-w-0 grid-cols-[30px_minmax(0,1fr)_auto] items-center gap-2 rounded-[12px] border px-2 py-2 text-left transition hover:bg-white/72 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[rgba(91,114,255,0.34)]",
                    item.active
                      ? "border-[rgba(91,114,255,0.28)] bg-[rgba(91,114,255,0.12)]"
                      : "border-white/54 bg-white/34",
                  )}
                  key={item.event.id}
                  onClick={() => {
                    on_focus_event(item.event);
                    set_is_open(false);
                  }}
                  title={`${item.step_label} · ${item.action_label} · ${item.title} · ${item.target}`}
                  type="button"
                >
                  <span className={cn(
                    "relative grid h-[30px] w-[30px] place-items-center rounded-[10px] border border-white/68 bg-white/62 text-(--icon-muted) shadow-[inset_0_1px_0_rgba(255,255,255,0.78)]",
                    item.active && "text-[color:var(--primary)] ring-2 ring-[rgba(91,114,255,0.18)]",
                  )}>
                    <Icon className="h-3.5 w-3.5" />
                    <span className={cn(
                      "absolute -bottom-0.5 -right-0.5 h-2 w-2 rounded-full border border-white/82",
                      phase_dot_class(item.event.phase),
                    )} />
                  </span>
                  <span className="min-w-0">
                    <span className="flex min-w-0 items-center gap-1.5">
                      <span className="shrink-0 text-[9px] font-black text-(--text-soft)">{item.step_label}</span>
                      <span className="truncate text-[10px] font-black text-(--text-strong)">{item.title}</span>
                    </span>
                    <span className="mt-0.5 block truncate text-[9px] font-semibold text-(--text-soft)">
                      {item.action_label} · {item.target}
                    </span>
                  </span>
                  <span className={cn(
                    "rounded-full px-1.5 py-px text-[8px] font-black",
                    phase_badge_class(item.event.phase),
                  )}>
                    {phase_label(item.event.phase)}
                  </span>
                </button>
              );
            })}
          </div>
        </div>
      ) : null}
    </div>
  );
}

function StagePowerAction({
  children,
}: {
  children: ReactNode;
}) {
  return (
    <div
      className="relative ml-1 grid h-7 w-7 place-items-center rounded-full border border-[rgba(223,93,98,0.24)] bg-[rgba(255,246,246,0.72)] text-[color:var(--destructive)] shadow-[inset_0_1px_0_rgba(255,255,255,0.78)] transition hover:bg-[rgba(255,235,235,0.9)] focus-within:ring-2 focus-within:ring-[rgba(223,93,98,0.22)]"
      title="退出操作舞台"
    >
      <Power className="pointer-events-none absolute h-3.5 w-3.5" />
      <div className="[&_button]:absolute [&_button]:inset-0 [&_button]:h-full [&_button]:w-full [&_button]:gap-0 [&_button]:rounded-full [&_button]:border-0 [&_button]:bg-transparent [&_button]:p-0 [&_button]:text-[0px] [&_button]:shadow-none [&_button]:outline-none [&_button]:ring-0 [&_svg]:opacity-0">
        {children}
      </div>
    </div>
  );
}

function build_stage_tool_queue_items(events: NexusOperationEvent[], active_event: NexusOperationEvent): StageToolQueueItem[] {
  const candidates = [...events, active_event].filter(is_tool_queue_event);
  const deduped = new Map<string, NexusOperationEvent>();
  for (const event of candidates) {
    deduped.set(event.id, event);
  }
  const ordered_events = [...deduped.values()].sort((left, right) => (
    (left.started_at ?? left.updated_at) - (right.started_at ?? right.updated_at)
  ));

  return ordered_events.map((event) => {
    const profile = resolve_operation_tool_profile(event.tool_name, event.kind, event.surface);
    return {
      action_label: profile.action_label,
      active: event.id === active_event.id,
      event,
      step_label: event_sequence_label(event, ordered_events),
      target: display_stage_event_target(event, profile.action_label),
      title: display_stage_event_title(event, profile.action_label),
    };
  });
}

function is_tool_queue_event(event: NexusOperationEvent): boolean {
  return event.surface !== "conversation" || Boolean(event.tool_name) || event.kind === "human_gate";
}

function phase_label(phase: OperationPhase): string {
  if (phase === "done") {
    return "完成";
  }
  if (phase === "waiting") {
    return "等待";
  }
  if (phase === "error") {
    return "异常";
  }
  if (phase === "cancelled") {
    return "取消";
  }
  if (phase === "queued") {
    return "排队";
  }
  return "运行";
}

function phase_dot_class(phase: OperationPhase): string {
  if (phase === "done") {
    return "bg-[rgba(47,184,132,0.78)]";
  }
  if (phase === "waiting") {
    return "bg-[rgba(223,157,46,0.84)]";
  }
  if (phase === "error" || phase === "cancelled") {
    return "bg-[rgba(223,93,98,0.82)]";
  }
  return "bg-[rgba(91,114,255,0.78)]";
}

function phase_badge_class(phase: OperationPhase): string {
  if (phase === "done") {
    return "bg-[rgba(47,184,132,0.12)] text-[color:var(--success)]";
  }
  if (phase === "waiting") {
    return "bg-[rgba(223,157,46,0.14)] text-[color:var(--warning)]";
  }
  if (phase === "error" || phase === "cancelled") {
    return "bg-[rgba(223,93,98,0.12)] text-[color:var(--destructive)]";
  }
  return "bg-[rgba(91,114,255,0.12)] text-[color:var(--primary)]";
}

export function StageDesktopIcons({
  on_restore,
  windows,
}: {
  on_restore: (window_id: string) => void;
  windows: StageWindowState[];
}) {
  const desktop_items = build_stage_desktop_icon_items(windows);

  if (!desktop_items.length) {
    return null;
  }

  return (
    <div className="absolute right-5 top-28 z-10 hidden grid-cols-1 gap-3 md:grid">
      {desktop_items.map((window) => {
        const Icon = icon_for_artifact_path(window.target);
        return (
          <button
            aria-label={window.aria_label}
            className="group flex w-[72px] flex-col items-center gap-1 text-center outline-none"
            key={window.window.id}
            onClick={() => on_restore(window.window.id)}
            title={window.title}
            type="button"
          >
            <div className={cn(
              "relative grid h-12 w-10 place-items-center rounded-[9px] border border-white/78 bg-[linear-gradient(180deg,rgba(255,255,255,0.96),rgba(244,248,252,0.86))] text-(--icon-default) shadow-[0_12px_26px_rgba(18,28,42,0.10),inset_0_1px_0_rgba(255,255,255,0.92)] transition group-hover:-translate-y-0.5 group-hover:bg-white group-focus-visible:ring-2 group-focus-visible:ring-[rgba(91,114,255,0.38)]",
              window.window.phase === "focused" && "text-[color:var(--primary)]",
              window.window.phase === "minimized" && "opacity-82",
              window.window.phase === "closed" && "opacity-64 grayscale-[0.22]",
            )}>
              <span className="absolute right-0 top-0 h-3.5 w-3.5 rounded-bl-[7px] border-b border-l border-[rgba(160,174,192,0.34)] bg-[linear-gradient(135deg,rgba(226,233,243,0.82),rgba(255,255,255,0.72))]" />
              <Icon className="h-[18px] w-[18px]" />
              <span className="absolute bottom-1.5 rounded-[5px] bg-[rgba(32,43,58,0.72)] px-1.5 py-px text-[7px] font-black leading-none text-white">
                {window.extension_label}
              </span>
              <span className={cn(
                "absolute -bottom-1.5 left-1/2 h-1.5 w-1.5 -translate-x-1/2 rounded-full border border-white/72",
                window.window.phase === "closed"
                  ? "bg-[rgba(117,131,149,0.55)]"
                  : window.window.phase === "minimized"
                    ? "bg-[rgba(223,157,46,0.82)]"
                    : "bg-[rgba(47,184,132,0.72)]",
              )} />
            </div>
            <p className="line-clamp-2 rounded-[6px] px-1 text-[9px] font-semibold leading-3 text-(--text-strong) group-hover:bg-white/48">
              {window.label}
            </p>
            <span className="text-[8px] font-semibold leading-none text-(--text-soft) opacity-0 transition group-hover:opacity-100 group-focus-visible:opacity-100">
              {window.file_kind_label}
            </span>
            <span className="sr-only">{window.state_label}</span>
          </button>
        );
      })}
    </div>
  );
}

export function StageAgentCursor({
  active_window,
}: {
  active_window: StageWindowState | null;
}) {
  if (!active_window) {
    return null;
  }
  const intent = agent_cursor_intent_for_window_kind(active_window.kind);
  const action_label = agent_cursor_action_label(intent);
  const app_label = stage_app_label_for_window_kind(active_window.kind);

  return (
    <div
      aria-label={`Nexus ${action_label} ${app_label}`}
      className={cn("operation-stage-agent-cursor pointer-events-none absolute z-50 hidden -translate-x-2 -translate-y-2 md:block", agent_cursor_anchor_class(active_window))}
      data-agent-cursor-intent={intent}
    >
      <span className="operation-stage-agent-cursor-target" />
      <MousePointer2 className="h-5 w-5 fill-[rgba(32,43,58,0.88)] text-[rgba(32,43,58,0.88)] drop-shadow-[0_8px_14px_rgba(18,28,42,0.22)]" />
    </div>
  );
}
