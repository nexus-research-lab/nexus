import { useEffect, useMemo, useState } from "react";
import {
  Battery,
  Bell,
  CheckCircle2,
  Command,
  Loader2,
  MousePointer2,
  Search,
  AlertTriangle,
  Activity,
  Wifi,
} from "lucide-react";

import { cn } from "@/lib/utils";

import type { StageWindowState } from "../operation-desktop-types";
import {
  icon_for_artifact_path,
  stage_app_label_for_window_kind,
} from "./operation-stage-window-meta";
import { stage_menu_items_for_window_kind } from "./operation-stage-app-identity";
import { build_stage_desktop_icon_items } from "./operation-stage-desktop-icons";
import { build_stage_menu_status } from "./operation-stage-menu-model";
import {
  agent_cursor_action_label,
  agent_cursor_anchor_class,
  agent_cursor_intent_for_window_kind,
} from "./operation-stage-agent-cursor";
import type { StageActivityCenterState, StageActivityItem } from "./operation-stage-live-strip";
import type { NexusOperationEvent } from "../operation-types";

export function StageMacMenuBar({
  active_window,
  windows,
}: {
  active_window: StageWindowState | null;
  windows: StageWindowState[];
}) {
  const app_name = active_window ? stage_app_label_for_window_kind(active_window.kind) : "Nexus";
  const menu_status = useMemo(() => (
    build_stage_menu_status(windows, active_window, (window) => stage_app_label_for_window_kind(window.kind))
  ), [active_window, windows]);
  const [current_time, set_current_time] = useState(() => new Date());
  const menu_items = useMemo(
    () => stage_menu_items_for_window_kind(active_window?.kind ?? null),
    [active_window?.kind],
  );
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
      className="absolute inset-x-4 top-3 z-40 flex h-9 items-center justify-between rounded-[14px] border border-white/64 bg-[rgba(255,255,255,0.62)] px-3 text-[11px] font-semibold text-(--text-strong) shadow-[0_12px_30px_rgba(18,28,42,0.09),inset_0_1px_0_rgba(255,255,255,0.72)] backdrop-blur-2xl max-md:hidden"
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
        {menu_items.map((item) => (
          <span className="text-(--text-soft)" key={item}>{item}</span>
        ))}
      </div>
      <div className="flex shrink-0 items-center gap-2 text-(--text-soft)">
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
        <Search className="h-3 w-3" />
        <Command className="h-3 w-3" />
        <Wifi className="h-3 w-3" />
        <Battery className="h-3 w-3" />
        <span className="font-mono text-[10px] text-(--text-strong)">{time_label}</span>
      </div>
    </div>
  );
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

export function StageActivityCenter({
  on_focus_event,
  state,
}: {
  on_focus_event: (event: NexusOperationEvent) => void;
  state: StageActivityCenterState;
}) {
  return (
    <div className="pointer-events-none absolute right-5 top-[58px] z-30 hidden w-[268px] md:block">
      <div className="operation-stage-live-strip pointer-events-auto rounded-[18px] border border-white/60 bg-[rgba(248,250,252,0.74)] p-2.5 text-(--text-strong) shadow-[0_18px_46px_rgba(18,28,42,0.14),inset_0_1px_0_rgba(255,255,255,0.72)] backdrop-blur-2xl">
        <div className="flex items-center justify-between gap-3 border-b border-[rgba(117,131,149,0.16)] pb-2">
          <span className="flex min-w-0 items-center gap-2">
            <span className="grid h-8 w-8 shrink-0 place-items-center rounded-[10px] border border-white/70 bg-white/66 text-(--icon-default) shadow-[inset_0_1px_0_rgba(255,255,255,0.78)]">
              <Activity className="h-4 w-4" />
            </span>
            <span className="min-w-0">
              <span className="block truncate text-[11px] font-black">Agent Activity</span>
              <span className="block truncate text-[9px] font-bold text-(--text-soft)">{state.active_app_label} · {state.running_label}</span>
            </span>
          </span>
          <span className="rounded-full border border-white/70 bg-white/58 px-2 py-0.5 text-[8px] font-black text-(--text-soft)">
            LIVE
          </span>
        </div>
        <div className="mt-2 grid gap-1.5">
          {state.items.map((item) => (
            <ActivityCenterItem item={item} key={item.key} on_focus_event={on_focus_event} />
          ))}
        </div>
      </div>
    </div>
  );
}

function ActivityCenterItem({
  item,
  on_focus_event,
}: {
  item: StageActivityItem;
  on_focus_event: (event: NexusOperationEvent) => void;
}) {
  const Icon = item.tone === "done"
    ? CheckCircle2
    : item.tone === "error"
      ? AlertTriangle
      : item.tone === "waiting"
        ? Bell
        : Loader2;

  return (
    <button
      aria-label={`查看 ${item.app_label}：${item.title}`}
      className={cn(
        "group grid min-w-0 grid-cols-[26px_minmax(0,1fr)] gap-2 rounded-[12px] border px-2 py-2 text-left transition hover:bg-white/70 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[rgba(91,114,255,0.36)]",
        item.tone === "waiting" && "border-[rgba(223,157,46,0.30)] bg-[rgba(255,249,236,0.68)]",
        item.tone === "error" && "border-[rgba(223,93,98,0.26)] bg-[rgba(255,246,246,0.68)]",
        item.tone === "active" && "border-[rgba(91,114,255,0.22)] bg-[rgba(247,249,255,0.62)]",
        item.tone === "done" && "border-[rgba(47,184,132,0.18)] bg-white/36",
      )}
      onClick={() => on_focus_event(item.event)}
      title={`${item.step_label} · ${item.title} · ${item.detail}`}
      type="button"
    >
      <span className={cn(
        "relative grid h-[26px] w-[26px] place-items-center rounded-[9px] border bg-white/72 text-(--icon-default) shadow-[inset_0_1px_0_rgba(255,255,255,0.78)]",
        item.tone === "waiting" && "text-[color:var(--warning)]",
        item.tone === "error" && "text-[color:var(--destructive)]",
        item.tone === "done" && "text-[color:var(--success)]",
      )}>
        <Icon className={cn("h-3.5 w-3.5", item.tone === "active" && "animate-spin")} />
      </span>
      <span className="min-w-0">
        <span className="flex min-w-0 items-center gap-1.5">
          <span className="truncate text-[10px] font-black text-(--text-strong)">{item.app_label}</span>
          <span className="shrink-0 rounded-full bg-white/58 px-1.5 py-px text-[7px] font-black text-(--text-soft)">
            {item.step_label}
          </span>
        </span>
        <span className="mt-0.5 block truncate text-[9px] font-bold text-(--text-strong)">{item.title}</span>
        <span className="mt-0.5 block truncate text-[8px] font-semibold text-(--text-soft)">{item.detail}</span>
      </span>
    </button>
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
