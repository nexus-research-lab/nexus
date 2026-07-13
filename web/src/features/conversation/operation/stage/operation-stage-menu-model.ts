import type { StageWindowState } from "../operation-desktop-types";

export interface StageMenuStatus {
  active_app_label: string;
  active_window_label: string | null;
  activity_label: string;
  dock_label: string | null;
  window_label: string;
}

export function buildStageMenuStatus(
  windows: StageWindowState[],
  active_window: StageWindowState | null,
  app_label_for_window: (window: StageWindowState) => string,
): StageMenuStatus {
  const open_windows = windows.filter((window) => window.phase !== "closed");
  const minimized_count = open_windows.filter((window) => window.phase === "minimized").length;
  const visible_count = open_windows.length - minimized_count;
  const active_app_label = active_window ? app_label_for_window(active_window) : "Nexus";
  const active_window_label = active_window ? normalize_stage_menu_window_title(active_window.title) : null;
  const activity_label = active_window_label
    ? `${active_app_label} · ${active_window_label}`
    : active_window ? active_app_label : "桌面待命";

  return {
    active_app_label,
    active_window_label,
    activity_label,
    dock_label: minimized_count > 0 ? `${minimized_count} 个在 Dock` : null,
    window_label: `${visible_count} 个窗口`,
  };
}

function normalize_stage_menu_window_title(title: string | null | undefined): string | null {
  const trimmed = title?.trim();
  if (!trimmed || trimmed === "Nexus" || trimmed === "桌面待命") {
    return null;
  }
  return trimmed.length > 28 ? `${trimmed.slice(0, 27)}…` : trimmed;
}
