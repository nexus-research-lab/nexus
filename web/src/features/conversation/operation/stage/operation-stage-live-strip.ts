import type { StageWindowKind, StageWindowState } from "../operation-desktop-types";
import type { NexusOperationEvent } from "../operation-types";
import { displayStageEventTarget, displayStageEventTitle } from "../operation-stage-labels";
import { resolveOperationToolProfile } from "../operation-tool-catalog";
import { eventSequenceLabel } from "./operation-stage-event-sequence";

export interface StageActivityItem {
  app_label: string;
  event: NexusOperationEvent;
  detail: string;
  key: string;
  step_label: string;
  title: string;
  tone: "active" | "done" | "waiting" | "error";
}

export interface StageActivityCenterState {
  app_label: string;
  active_app_label: string;
  active_title: string;
  detail: string;
  items: StageActivityItem[];
  running_label: string;
  step_label: string;
  title: string;
  tone: StageActivityItem["tone"];
}

const MAX_ACTIVITY_ITEMS = 5;

export function buildStageLiveStripState({
  active_event,
  active_window,
  events,
  windows = [],
}: {
  active_event: NexusOperationEvent;
  active_window: StageWindowState | null;
  events: NexusOperationEvent[];
  windows?: StageWindowState[];
}): StageActivityCenterState {
  const event_window_by_id = new Map(windows.map((window) => [window.payload.event.id, window]));
  const candidates = [active_event, ...events.slice().reverse()]
    .filter((event, index, list) => list.findIndex((item) => item.id === event.id) === index)
    .sort((left, right) => activity_priority(right, active_event.id) - activity_priority(left, active_event.id))
    .slice(0, MAX_ACTIVITY_ITEMS);
  const items = candidates.map((event): StageActivityItem => {
    const profile = resolveOperationToolProfile(event.tool_name, event.kind, event.surface);
    const window = event_window_by_id.get(event.id) ?? (event.id === active_event.id ? active_window : null);
    const app_label = window ? live_strip_app_label_for_kind(window.kind) : profile.title;
    const title = displayStageEventTitle(event, profile.action_label);
    const target = displayStageEventTarget(event, profile.action_label)
      || window?.target
      || window?.payload.target
      || profile.action_label;

    return {
      app_label,
      detail: `${profile.action_label} · ${target}`,
      event,
      key: event.id,
      step_label: eventSequenceLabel(event, events),
      title,
      tone: live_strip_tone_for_event(event),
    };
  });

  return {
    app_label: items[0]?.app_label ?? "Nexus",
    active_app_label: active_window ? live_strip_app_label_for_kind(active_window.kind) : "Nexus",
    active_title: items[0]?.title ?? "桌面待命",
    detail: items[0]?.detail ?? "等待工具调用",
    items,
    running_label: activity_running_label(windows),
    step_label: items[0]?.step_label ?? "待命",
    title: items[0]?.title ?? "桌面待命",
    tone: items[0]?.tone ?? "active",
  };
}

function live_strip_app_label_for_kind(kind: StageWindowKind): string {
  if (kind === "browser") {
    return "Navi";
  }
  if (kind === "terminal") {
    return "终端";
  }
  if (kind === "finder") {
    return "访达";
  }
  if (kind === "code_editor") {
    return "Code";
  }
  if (kind === "handoff") {
    return "交付台";
  }
  if (kind === "task_board") {
    return "活动监视器";
  }
  if (kind === "permission_wait") {
    return "系统设置";
  }
  return "Nexus";
}

function live_strip_tone_for_event(event: NexusOperationEvent): StageActivityItem["tone"] {
  if (event.phase === "waiting") {
    return "waiting";
  }
  if (event.phase === "error" || event.phase === "cancelled") {
    return "error";
  }
  if (event.phase === "done") {
    return "done";
  }
  return "active";
}

function activity_priority(event: NexusOperationEvent, active_event_id: string): number {
  if (event.id === active_event_id) {
    return 100;
  }
  if (event.phase === "waiting") {
    return 80;
  }
  if (event.phase === "error" || event.phase === "cancelled") {
    return 70;
  }
  if (event.phase === "running") {
    return 60;
  }
  return 10 + (event.updated_at ?? event.started_at ?? 0) / 1_000_000_000_000;
}

function activity_running_label(windows: StageWindowState[]): string {
  const visible_count = windows.filter((window) => window.phase !== "closed" && window.phase !== "minimized").length;
  const minimized_count = windows.filter((window) => window.phase === "minimized").length;
  if (minimized_count) {
    return `${visible_count} 个前台 · ${minimized_count} 个 Dock`;
  }
  return `${visible_count} 个窗口`;
}
