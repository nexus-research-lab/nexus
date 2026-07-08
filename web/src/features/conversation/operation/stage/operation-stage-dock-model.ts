import type { StageWindowKind, StageWindowState } from "../operation-desktop-types";

export interface DockAppGroup {
  app_label: string;
  count: number;
  is_active: boolean;
  is_running: boolean;
  window: StageWindowState;
}

export interface DockAppSlot {
  app_label: string;
  count: number;
  is_active: boolean;
  is_running: boolean;
  kind: StageWindowKind;
  window: StageWindowState | null;
}

export type DockSlotState = "active" | "background" | "minimized" | "recoverable" | "idle";

export interface DockSlotPresentation {
  is_disabled: boolean;
  state: DockSlotState;
  state_label: string;
  title: string;
}

export function build_dock_app_slots(
  app_groups: DockAppGroup[],
): DockAppSlot[] {
  return app_groups.map((group): DockAppSlot => (
    {
      app_label: group.app_label,
      count: group.count,
      is_active: group.is_active,
      is_running: group.is_running,
      kind: group.window.kind,
      window: group.window,
    }
  ));
}

export function resolve_dock_slot_presentation(
  slot: DockAppSlot,
  window_title: string,
): DockSlotPresentation {
  const state = resolve_dock_slot_state(slot);
  const state_label = dock_slot_state_label(state);
  const title = !slot.window
    ? `${slot.app_label} · ${state_label}`
    : slot.count > 1
      ? `${slot.app_label} · ${slot.count} 个窗口 · ${state_label}`
      : `${slot.app_label} · ${window_title} · ${state_label}`;

  return {
    is_disabled: state === "idle",
    state,
    state_label,
    title,
  };
}

export function group_dock_windows_by_app(
  windows: StageWindowState[],
  active_window_id: string | null,
  app_label_for_kind: (kind: StageWindowKind) => string,
): DockAppGroup[] {
  const groups = new Map<string, DockAppGroup>();

  for (const window of windows) {
    const app_label = app_label_for_kind(window.kind);
    const existing = groups.get(app_label);
    const is_active = window.id === active_window_id;
    const is_running = window.phase !== "closed";
    if (!existing) {
      groups.set(app_label, {
        app_label,
        count: is_running ? 1 : 0,
        is_active,
        is_running,
        window,
      });
      continue;
    }
    const had_running_window = existing.is_running;
    existing.count += is_running ? 1 : 0;
    existing.is_active = existing.is_active || is_active;
    existing.is_running = existing.is_running || is_running;
    if (is_active || (!had_running_window && is_running)) {
      existing.window = window;
    }
  }

  return [...groups.values()];
}

function resolve_dock_slot_state(slot: DockAppSlot): DockSlotState {
  if (!slot.window) {
    return "idle";
  }
  if (slot.is_active) {
    return "active";
  }
  if (slot.window.phase === "minimized") {
    return "minimized";
  }
  if (slot.is_running) {
    return "background";
  }
  return "recoverable";
}

function dock_slot_state_label(state: DockSlotState): string {
  if (state === "active") {
    return "当前";
  }
  if (state === "minimized") {
    return "已最小化";
  }
  if (state === "background") {
    return "后台";
  }
  if (state === "recoverable") {
    return "可重新打开";
  }
  return "未打开";
}
