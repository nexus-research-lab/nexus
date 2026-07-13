import type { StageWindowState } from "../operation-desktop-types";

export interface StageHiddenWindowSummary {
  closed_count: number;
  hidden_count: number;
  label: string;
  minimized_count: number;
}

export function summarizeHiddenStageWindows(windows: StageWindowState[]): StageHiddenWindowSummary {
  const minimized_count = windows.filter((window) => window.phase === "minimized").length;
  const closed_count = windows.filter((window) => window.phase === "closed").length;
  const hidden_count = minimized_count + closed_count;
  const label = hidden_count <= 0
    ? "桌面空闲"
    : closed_count <= 0
      ? `${minimized_count} 个窗口在 Dock`
      : minimized_count > 0
        ? `${minimized_count} 个在 Dock · ${closed_count} 个已关闭`
        : `${closed_count} 个窗口已关闭`;

  return {
    closed_count,
    hidden_count,
    label,
    minimized_count,
  };
}
