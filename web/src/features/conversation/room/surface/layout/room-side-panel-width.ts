// INPUT: 面板种类、保存的百分比与当前容器/视口宽度。
// OUTPUT: 同一边界定义生成的 CSS 限制和实际可调整像素范围。
// POS: Room 右栏尺寸投影；不持有偏好、不调用业务命令。

import { HOME_SIDE_PANEL_MIN_WIDTH_PERCENT, HOME_SIDE_PANEL_MAX_WIDTH_PERCENT } from "@/lib/layout/home-layout";

export type RoomSidePanelKind = "auxiliary" | "thread";
type WidthLimit = { pixels: number; viewportPercent?: number };
const LIMITS: Record<RoomSidePanelKind, { min: WidthLimit; max: WidthLimit }> = {
  auxiliary: { min: { pixels: 520, viewportPercent: 46 }, max: { pixels: 860, viewportPercent: 54 } },
  thread: { min: { pixels: 360 }, max: { pixels: 560 } },
};

function widthStyle(limit: WidthLimit): string {
  return limit.viewportPercent === undefined ? `${limit.pixels}px`
    : `min(${limit.pixels}px, ${limit.viewportPercent}vw)`;
}

export function getRoomSidePanelWidthStyle(kind: RoomSidePanelKind) {
  return { minWidth: widthStyle(LIMITS[kind].min), maxWidth: widthStyle(LIMITS[kind].max) };
}

export function getRoomSidePanelWidthRange(
  kind: RoomSidePanelKind, widthPercent: number, containerWidth: number, viewportWidth: number,
) {
  if (containerWidth <= 0 || viewportWidth <= 0) return null;
  const resolve = (limit: WidthLimit) => limit.viewportPercent === undefined ? limit.pixels
    : Math.min(limit.pixels, viewportWidth * limit.viewportPercent / 100);
  const cssMin = resolve(LIMITS[kind].min);
  const cssMax = resolve(LIMITS[kind].max);
  // CSS min-width 在与 max-width 冲突时优先；使用同一投影避免键盘落入隐形区间。
  const project = (percent: number) => Math.max(cssMin, Math.min(cssMax, containerWidth * percent / 100));
  const min = project(HOME_SIDE_PANEL_MIN_WIDTH_PERCENT);
  const max = project(HOME_SIDE_PANEL_MAX_WIDTH_PERCENT);
  return { min, max, value: Math.min(max, Math.max(min, project(widthPercent))) };
}
