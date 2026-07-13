import type { StageWindowState } from "../operation-desktop-types";

export interface StageWindowLaunchState {
  delay_ms: number;
  origin: "active" | "desktop" | "dock";
}

export function buildStageWindowLaunchState({
  index,
  is_active,
  window,
}: {
  index: number;
  is_active: boolean;
  window: StageWindowState;
}): StageWindowLaunchState {
  if (window.layout === "artifact" || window.kind === "finder") {
    return {
      delay_ms: is_active || window.phase === "focused"
        ? 0
        : Math.min(420, 220 + index * 90),
      origin: "desktop",
    };
  }
  if (is_active || window.phase === "focused") {
    return {
      delay_ms: 0,
      origin: "dock",
    };
  }
  return {
    delay_ms: Math.min(520, 140 + index * 110),
    origin: "dock",
  };
}
