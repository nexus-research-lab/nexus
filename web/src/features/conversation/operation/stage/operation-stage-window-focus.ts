import type { StageWindowState } from "../operation-desktop-types";

export type StageWindowFocusCycleDirection = "next" | "previous";

export function resolveNextWindowFocus({
  current_focus_id,
  hidden_window_id,
  windows,
}: {
  current_focus_id: string | null;
  hidden_window_id: string;
  windows: StageWindowState[];
}): string | null {
  const visible_windows = windows.filter((window) => (
    window.id !== hidden_window_id &&
    window.phase !== "closed" &&
    window.phase !== "minimized"
  ));

  if (current_focus_id && visible_windows.some((window) => window.id === current_focus_id)) {
    return current_focus_id;
  }

  const focused_window = visible_windows.find((window) => window.phase === "focused");
  if (focused_window) {
    return focused_window.id;
  }

  return [...visible_windows]
    .sort((left, right) => right.z - left.z)[0]?.id ?? null;
}

export function resolveCycledWindowFocus({
  current_focus_id,
  direction,
  windows,
}: {
  current_focus_id: string | null;
  direction: StageWindowFocusCycleDirection;
  windows: StageWindowState[];
}): string | null {
  const visible_windows = [...windows]
    .filter((window) => window.phase !== "closed" && window.phase !== "minimized")
    .sort((left, right) => right.z - left.z);
  if (visible_windows.length === 0) {
    return null;
  }
  if (!current_focus_id) {
    return visible_windows[0].id;
  }
  const current_index = visible_windows.findIndex((window) => window.id === current_focus_id);
  if (current_index < 0) {
    return visible_windows[0].id;
  }
  const offset = direction === "next" ? 1 : -1;
  const next_index = (current_index + offset + visible_windows.length) % visible_windows.length;
  return visible_windows[next_index].id;
}
