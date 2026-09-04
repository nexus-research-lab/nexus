// INPUT: 八类锚定浮层 preset、锚点矩形、视口空间与可选内容估算高度。
// OUTPUT: 证明 gap、视口内边距、宽高边界及锚点宽度回退保持既有几何值。
// POS: 锚定浮层语义 geometry preset 合同测试；底层坐标算法由 model 负责。

import { describe, expect, it, vi } from "vitest";

import {
  getUiAnchoredOverlayMinimumWidth,
  getUiAnchoredOverlayViewportInset,
  resolveUiAnchoredOverlayPosition,
  type UiAnchoredOverlayPreset,
} from "./anchored-overlay-layout";

interface PresetExpectation {
  gap: number;
  maxHeight: number;
  minHeight: number;
  minWidth: number;
  preset: UiAnchoredOverlayPreset;
  viewportInset: number;
}

const PRESET_EXPECTATIONS: readonly PresetExpectation[] = [
  {
    preset: "directory-list",
    gap: 6,
    viewportInset: 12,
    minWidth: 330,
    minHeight: 190,
    maxHeight: 280,
  },
  {
    preset: "reference-list",
    gap: 8,
    viewportInset: 12,
    minWidth: 360,
    minHeight: 96,
    maxHeight: 320,
  },
  {
    preset: "form-picker",
    gap: 10,
    viewportInset: 24,
    minWidth: 480,
    minHeight: 240,
    maxHeight: 320,
  },
  {
    preset: "status-summary",
    gap: 6,
    viewportInset: 12,
    minWidth: 192,
    minHeight: 72,
    maxHeight: 72,
  },
  {
    preset: "status-list",
    gap: 6,
    viewportInset: 12,
    minWidth: 232,
    minHeight: 64,
    maxHeight: 248,
  },
  {
    preset: "cascade-menu",
    gap: 6,
    viewportInset: 12,
    minWidth: 224,
    minHeight: 32,
    maxHeight: 320,
  },
  {
    preset: "command-list",
    gap: 6,
    viewportInset: 12,
    minWidth: 0,
    minHeight: 44,
    maxHeight: 296,
  },
  {
    preset: "command-picker",
    gap: 6,
    viewportInset: 12,
    minWidth: 0,
    minHeight: 44,
    maxHeight: 336,
  },
];

function setViewport(width: number, height: number) {
  Object.defineProperties(window, {
    innerHeight: { configurable: true, value: height },
    innerWidth: { configurable: true, value: width },
  });
}

function createAnchor({
  bottom = 120,
  left = 100,
  top = 100,
  width = 100,
}: {
  bottom?: number;
  left?: number;
  top?: number;
  width?: number;
} = {}): HTMLElement {
  const anchor = document.createElement("button");
  vi.spyOn(anchor, "getBoundingClientRect").mockReturnValue({
    bottom,
    height: bottom - top,
    left,
    right: left + width,
    top,
    width,
    x: left,
    y: top,
    toJSON: () => ({}),
  });
  return anchor;
}

describe("anchored overlay layout presets", () => {
  it.each(PRESET_EXPECTATIONS)(
    "keeps the $preset roomy viewport geometry",
    ({ gap, maxHeight, minWidth, preset, viewportInset }) => {
      setViewport(2_000, 2_000);
      const anchor = createAnchor();

      const position = resolveUiAnchoredOverlayPosition({
        anchor,
        placement: "bottom",
        preset,
      });

      expect(getUiAnchoredOverlayViewportInset(preset)).toBe(viewportInset);
      expect(getUiAnchoredOverlayMinimumWidth(preset)).toBe(minWidth);
      expect(position).toEqual({
        left: 100,
        maxHeight,
        placement: "bottom",
        top: 120 + gap,
        width: minWidth || 100,
      });
    },
  );

  it.each(PRESET_EXPECTATIONS)(
    "keeps the $preset minimum height when forced into a cramped side",
    ({ minHeight, preset }) => {
      setViewport(2_000, 1_000);
      const anchor = createAnchor({ bottom: 990, top: 970 });

      const position = resolveUiAnchoredOverlayPosition({
        anchor,
        placement: "bottom",
        preset,
      });

      expect(position.maxHeight).toBe(minHeight);
    },
  );

  it("defaults estimates to the preset maximum and caps larger estimates first", () => {
    setViewport(2_000, 2_000);
    const anchor = createAnchor();

    const defaultEstimate = resolveUiAnchoredOverlayPosition({
      anchor,
      placement: "bottom",
      preset: "status-list",
    });
    const boundedEstimate = resolveUiAnchoredOverlayPosition({
      anchor,
      estimatedContentHeight: 140,
      placement: "bottom",
      preset: "status-list",
    });
    const oversizedEstimate = resolveUiAnchoredOverlayPosition({
      anchor,
      estimatedContentHeight: 999,
      placement: "bottom",
      preset: "status-list",
    });

    expect(defaultEstimate.maxHeight).toBe(248);
    expect(boundedEstimate.maxHeight).toBe(140);
    expect(oversizedEstimate.maxHeight).toBe(248);
  });
});
