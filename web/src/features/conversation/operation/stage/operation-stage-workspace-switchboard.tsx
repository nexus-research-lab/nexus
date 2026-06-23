import { LayoutDashboard, RotateCcw } from "lucide-react";

import { cn } from "@/lib/utils";

import type { StageWindowState } from "../operation-desktop-types";
import {
  display_stage_event_target,
  display_stage_event_title,
} from "../operation-stage-labels";
import { resolve_operation_tool_profile } from "../operation-tool-catalog";
import type { NexusOperationEvent, NexusOperationSnapshot, OperationPhase } from "../operation-types";
import { basename } from "../operation-scene-planner-helpers";
import {
  event_sequence_label,
  icon_for_operation_kind,
  icon_for_window_kind,
  stage_app_label_for_window_kind,
} from "./operation-stage-helpers";

const MAX_EVENT_ITEMS = 8;
const MAX_WINDOW_ITEMS = 4;

export function StageWorkspaceSwitchboard({
  active_event,
  active_window_id,
  events,
  on_focus_event,
  on_restore_window,
  snapshot,
  windows,
}: {
  active_event: NexusOperationEvent;
  active_window_id: string | null;
  events: NexusOperationEvent[];
  on_focus_event: (event: NexusOperationEvent) => void;
  on_restore_window: (window_id: string) => void;
  snapshot: NexusOperationSnapshot | null;
  windows: StageWindowState[];
}) {
  const event_items = switchboard_event_items(events, active_event);
  const window_items = switchboard_window_items(windows, active_window_id);
  const workspace_label = switchboard_workspace_label(snapshot, active_event);

  if (!event_items.length && !window_items.length) {
    return null;
  }

  return (
    <div className="pointer-events-none absolute left-4 top-[58px] z-30 hidden w-[136px] md:block">
      <div className="pointer-events-auto rounded-[18px] border border-white/62 bg-[rgba(255,255,255,0.48)] p-2 shadow-[0_18px_46px_rgba(18,28,42,0.13),inset_0_1px_0_rgba(255,255,255,0.78)] backdrop-blur-2xl">
        <div className="flex items-center gap-2 rounded-[13px] border border-white/54 bg-white/44 px-2 py-1.5">
          <span className="grid h-7 w-7 shrink-0 place-items-center rounded-[10px] border border-white/64 bg-[linear-gradient(135deg,rgba(91,114,255,0.16),rgba(255,255,255,0.74),rgba(79,162,159,0.14))] text-[color:var(--primary)] shadow-[inset_0_1px_0_rgba(255,255,255,0.74)]">
            <LayoutDashboard className="h-3.5 w-3.5" />
          </span>
          <span className="min-w-0">
            <span className="block truncate text-[10px] font-black text-(--text-strong)">工作现场</span>
            <span className="block truncate text-[8px] font-bold text-(--text-soft)">{workspace_label}</span>
          </span>
        </div>

        {event_items.length ? (
          <div className="mt-2 rounded-[14px] border border-white/52 bg-white/34 px-2 py-2">
            <div className="flex items-center justify-between gap-2">
              <span className="text-[9px] font-black text-(--text-strong)">工具队列</span>
              <span className="rounded-full bg-white/54 px-1.5 py-px text-[7px] font-black text-(--text-soft)">
                {event_items.length} 步
              </span>
            </div>
            <div className="mt-2 grid grid-cols-3 gap-1.5">
              {event_items.map((item) => {
                const Icon = icon_for_operation_kind(item.event.kind);
                return (
                  <button
                    aria-label={`查看${item.step_label}：${item.title}`}
                    className={cn(
                      "group relative grid h-8 w-8 shrink-0 place-items-center rounded-[11px] border transition duration-200 ease-out hover:-translate-y-0.5 focus-visible:-translate-y-0.5 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[rgba(91,114,255,0.36)]",
                      item.active
                        ? "border-[rgba(91,114,255,0.34)] bg-[rgba(91,114,255,0.16)] text-[color:var(--primary)] shadow-[0_10px_22px_rgba(91,114,255,0.16)]"
                        : "border-white/54 bg-white/42 text-(--icon-muted) hover:bg-white/68 hover:text-(--text-strong)",
                    )}
                    key={item.event.id}
                    onClick={() => on_focus_event(item.event)}
                    title={`${item.step_label} · ${item.title} · ${item.target}`}
                    type="button"
                  >
                    <Icon className="h-3.5 w-3.5" />
                    <span className={cn(
                      "absolute -bottom-1 left-1/2 h-1.5 w-1.5 -translate-x-1/2 rounded-full border border-white/80",
                      phase_dot_class(item.event.phase),
                    )} />
                    <span className="pointer-events-none absolute left-0 top-[calc(100%+8px)] hidden w-[150px] rounded-[10px] border border-white/72 bg-[rgba(20,28,38,0.84)] px-2 py-1.5 text-left text-white shadow-[0_12px_30px_rgba(18,28,42,0.22)] backdrop-blur-xl group-hover:block group-focus-visible:block">
                      <span className="block text-[9px] font-black">{item.step_label} · {item.action_label}</span>
                      <span className="mt-0.5 block truncate text-[9px] font-semibold text-white/78">{item.title}</span>
                      <span className="mt-0.5 block truncate text-[8px] font-medium text-white/60">{item.target}</span>
                    </span>
                  </button>
                );
              })}
            </div>
          </div>
        ) : null}

        {window_items.length ? (
          <div className="mt-2 grid gap-1.5">
            {window_items.map((item) => {
              const Icon = icon_for_window_kind(item.window.kind);
              return (
                <button
                  aria-label={`${item.action_label}：${item.app_label} ${item.title}`}
                  className={cn(
                    "group relative grid min-w-0 grid-cols-[30px_minmax(0,1fr)] items-center gap-2 rounded-[13px] border px-1.5 py-1.5 text-left transition duration-200 ease-out hover:-translate-y-0.5 focus-visible:-translate-y-0.5 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[rgba(91,114,255,0.34)]",
                    item.active
                      ? "border-[rgba(91,114,255,0.30)] bg-[rgba(91,114,255,0.13)] shadow-[0_10px_22px_rgba(91,114,255,0.12)]"
                      : "border-white/48 bg-white/28 hover:bg-white/54",
                  )}
                  key={item.window.id}
                  onClick={() => on_restore_window(item.window.id)}
                  title={`${item.app_label} · ${item.title} · ${item.state_label}`}
                  type="button"
                >
                  <span className={cn(
                    "relative grid h-[30px] w-[30px] place-items-center rounded-[11px] border border-white/60 bg-white/54 text-(--icon-muted) shadow-[inset_0_1px_0_rgba(255,255,255,0.72)]",
                    item.active && "text-[color:var(--primary)] ring-2 ring-[rgba(91,114,255,0.20)]",
                  )}>
                    <Icon className="h-3.5 w-3.5" />
                    <span className={cn(
                      "absolute -bottom-0.5 -right-0.5 h-2 w-2 rounded-full border border-white/82",
                      window_dot_class(item.window),
                    )} />
                  </span>
                  <span className="min-w-0">
                    <span className="flex min-w-0 items-center gap-1">
                      <span className="truncate text-[9px] font-black text-(--text-strong)">{item.app_label}</span>
                      <span className="shrink-0 rounded-full bg-white/52 px-1 py-px text-[7px] font-black text-(--text-soft)">
                        {item.state_label}
                      </span>
                    </span>
                    <span className="mt-0.5 block truncate text-[8px] font-bold text-(--text-soft)">{item.title}</span>
                  </span>
                  <RotateCcw className="absolute right-2 top-2 h-3 w-3 text-(--icon-muted) opacity-0 transition group-hover:opacity-100 group-focus-visible:opacity-100" />
                </button>
              );
            })}
          </div>
        ) : null}
      </div>
    </div>
  );
}

function switchboard_event_items(events: NexusOperationEvent[], active_event: NexusOperationEvent) {
  const recent = [...events.slice(-MAX_EVENT_ITEMS), active_event];
  const deduped = new Map<string, NexusOperationEvent>();
  for (const item of recent) {
    deduped.set(item.id, item);
  }
  return [...deduped.values()]
    .sort((left, right) => (left.started_at ?? left.updated_at) - (right.started_at ?? right.updated_at))
    .slice(-MAX_EVENT_ITEMS)
    .map((event) => {
      const profile = resolve_operation_tool_profile(event.tool_name, event.kind, event.surface);
      return {
        action_label: profile.action_label,
        active: event.id === active_event.id,
        event,
        step_label: event_sequence_label(event, events),
        target: display_stage_event_target(event, profile.action_label),
        title: display_stage_event_title(event, profile.action_label),
      };
    });
}

function switchboard_window_items(windows: StageWindowState[], active_window_id: string | null) {
  return windows
    .slice()
    .sort((left, right) => {
      const left_active = left.id === active_window_id ? 1 : 0;
      const right_active = right.id === active_window_id ? 1 : 0;
      if (left_active !== right_active) {
        return right_active - left_active;
      }
      return right.z - left.z;
    })
    .slice(0, MAX_WINDOW_ITEMS)
    .map((window) => {
      const app_label = stage_app_label_for_window_kind(window.kind);
      return {
        action_label: window.phase === "closed" || window.phase === "minimized" ? "恢复窗口" : "切换窗口",
        active: window.id === active_window_id && window.phase !== "closed" && window.phase !== "minimized",
        app_label,
        state_label: window_state_label(window),
        title: compact_window_title(window),
        window,
      };
    });
}

function switchboard_workspace_label(snapshot: NexusOperationSnapshot | null, active_event: NexusOperationEvent): string {
  const key = snapshot?.key ?? active_event.session_key;
  if (!key) {
    return active_event.agent_id || "Nexus session";
  }
  return key
    .replace(/^session:/, "")
    .replace(/^room-session:/, "room:")
    .slice(0, 34);
}

function compact_window_title(window: StageWindowState): string {
  const target = window.target ?? window.payload.target ?? "";
  const name = basename(target);
  if (name && name !== "preview") {
    return name;
  }
  return window.title;
}

function window_state_label(window: StageWindowState): string {
  if (window.phase === "focused") {
    return "前台";
  }
  if (window.phase === "background" || window.phase === "opening") {
    return "运行";
  }
  if (window.phase === "minimized") {
    return "Dock";
  }
  if (window.phase === "closed") {
    return "隐藏";
  }
  if (window.phase === "error") {
    return "异常";
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

function window_dot_class(window: StageWindowState): string {
  if (window.phase === "closed") {
    return "bg-[rgba(117,131,149,0.58)]";
  }
  if (window.phase === "minimized") {
    return "bg-[rgba(223,157,46,0.82)]";
  }
  if (window.phase === "error") {
    return "bg-[rgba(223,93,98,0.82)]";
  }
  return "bg-[rgba(47,184,132,0.74)]";
}
