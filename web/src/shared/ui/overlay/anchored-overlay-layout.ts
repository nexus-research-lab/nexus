// INPUT: 锚点、上下方向、对齐方式、语义 geometry preset 与可选内容估算高度。
// OUTPUT: 保持既有八类浮层尺寸的视口安全位置，以及复合布局需要复用的 preset 边界。
// POS: 锚定浮层语义几何唯一 owner；不解释菜单、选择器或业务内容。

import {
  resolveAnchoredOverlayPosition,
  type UiAnchoredOverlayAlignment,
  type UiAnchoredOverlayPlacement,
  type UiAnchoredOverlayPosition,
} from "./anchored-overlay-model";

export type { UiAnchoredOverlayPosition } from "./anchored-overlay-model";

export type UiAnchoredOverlayPreset =
  | "cascade-menu"
  | "command-list"
  | "command-picker"
  | "directory-list"
  | "form-picker"
  | "reference-list"
  | "status-list"
  | "status-summary";

interface UiAnchoredOverlayGeometry {
  gap: number;
  maxHeight: number;
  minHeight: number;
  minWidth: number;
  viewportInset: number;
}

const UI_ANCHORED_OVERLAY_GEOMETRY: Readonly<
  Record<UiAnchoredOverlayPreset, UiAnchoredOverlayGeometry>
> = {
  "directory-list": {
    gap: 6,
    viewportInset: 12,
    minWidth: 330,
    minHeight: 190,
    maxHeight: 280,
  },
  "reference-list": {
    gap: 8,
    viewportInset: 12,
    minWidth: 360,
    minHeight: 96,
    maxHeight: 320,
  },
  "form-picker": {
    gap: 10,
    viewportInset: 24,
    minWidth: 480,
    minHeight: 240,
    maxHeight: 320,
  },
  "status-summary": {
    gap: 6,
    viewportInset: 12,
    minWidth: 192,
    minHeight: 72,
    maxHeight: 72,
  },
  "status-list": {
    gap: 6,
    viewportInset: 12,
    minWidth: 232,
    minHeight: 64,
    maxHeight: 248,
  },
  "cascade-menu": {
    gap: 6,
    viewportInset: 12,
    minWidth: 224,
    minHeight: 32,
    maxHeight: 320,
  },
  "command-list": {
    gap: 6,
    viewportInset: 12,
    minWidth: 0,
    minHeight: 44,
    maxHeight: 296,
  },
  "command-picker": {
    gap: 6,
    viewportInset: 12,
    minWidth: 0,
    minHeight: 44,
    maxHeight: 336,
  },
};

export function getUiAnchoredOverlayViewportInset(
  preset: UiAnchoredOverlayPreset,
): number {
  return UI_ANCHORED_OVERLAY_GEOMETRY[preset].viewportInset;
}

export function getUiAnchoredOverlayMinimumWidth(
  preset: UiAnchoredOverlayPreset,
): number {
  return UI_ANCHORED_OVERLAY_GEOMETRY[preset].minWidth;
}

export function resolveUiAnchoredOverlayPosition({
  align,
  anchor,
  estimatedContentHeight,
  placement,
  preset,
}: {
  align?: UiAnchoredOverlayAlignment;
  anchor: HTMLElement;
  estimatedContentHeight?: number;
  placement: UiAnchoredOverlayPlacement;
  preset: UiAnchoredOverlayPreset;
}): UiAnchoredOverlayPosition {
  const geometry = UI_ANCHORED_OVERLAY_GEOMETRY[preset];
  const estimatedHeight = Math.min(
    estimatedContentHeight ?? geometry.maxHeight,
    geometry.maxHeight,
  );
  return resolveAnchoredOverlayPosition({
    align,
    anchor,
    estimatedHeight,
    gap: geometry.gap,
    maxHeight: geometry.maxHeight,
    minHeight: geometry.minHeight,
    minWidth: geometry.minWidth,
    placement,
    viewportMargin: geometry.viewportInset,
  });
}
