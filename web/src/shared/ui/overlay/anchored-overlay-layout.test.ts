// INPUT: 八类锚定浮层 preset、锚点矩形、视口空间与可选内容估算高度。
// OUTPUT: 证明 gap、视口内边距、宽高边界及锚点宽度回退保持既有几何值。
// POS: 锚定浮层语义 geometry preset 合同测试；底层坐标算法由 model 负责。

import { describe, expect, it, vi } from "vitest";

import {
  getUiAnchoredOverlayMinimumWidth,
  getUiAnchoredOverlayViewportInset,
  resolveUiAnchoredOverlayPosition,
  resolveUiPointOverlayPosition,
  resolveUiSideOverlayPosition,
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
  it.each([
    { top: -120, placement: "bottom" as const },
    { top: 4, placement: "top" as const },
    { top: 1_004, placement: "bottom" as const },
  ])("keeps a $placement panel visible when its anchor moves beyond the viewport (top=$top)", ({ top, placement }) => {
    setViewport(600, 400);
    const position = resolveUiAnchoredOverlayPosition({
      anchor: createAnchor({ top, bottom: top + 32 }),
      estimatedContentHeight: 180,
      preset: "reference-list",
      placement,
    });
    const panelTop = position.top ?? window.innerHeight - position.bottom! - position.maxHeight;
    expect(panelTop).toBeGreaterThanOrEqual(12);
    expect(panelTop + position.maxHeight).toBeLessThanOrEqual(window.innerHeight - 12);
  });

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

describe("point and side overlays share preset boundaries", () => {
  it("constrains composed content width through the same anchored viewport solver", () => {
    setViewport(180, 300);
    const anchor = document.createElement("button");
    const rect = vi.spyOn(anchor, "getBoundingClientRect").mockReturnValue(new DOMRect(50, 200, 80, 28));
    const position = resolveUiAnchoredOverlayPosition({
      anchor, contentWidth: 486, estimatedContentHeight: 80, placement: "top", preset: "cascade-menu",
    });
    expect(position.width).toBe(156);
    expect(position.left).toBe(12);
    expect(position.maxHeight).toBe(80);
    rect.mockRestore();
  });

  it("moves a pointer menu back inside the viewport without changing its content height", () => {
    setViewport(800, 600);
    const position = resolveUiPointOverlayPosition({
      point: { x: 799, y: 599 }, preset: "cascade-menu", estimatedContentHeight: 256,
    });
    expect(position).toEqual({ left: 564, top: 332, width: 224, maxHeight: 256, placement: "top" });
  });

  it("uses one internal scroll limit when content or viewport is too small", () => {
    setViewport(180, 240);
    const position = resolveUiPointOverlayPosition({
      point: { x: -20, y: -30 }, preset: "cascade-menu", estimatedContentHeight: 900,
    });
    expect(position).toEqual({ left: 12, top: 12, width: 156, maxHeight: 216, placement: "bottom" });
  });

  it("aligns a side menu to its actual row and flips left when the right edge cannot fit", () => {
    setViewport(800, 600);
    const anchor = document.createElement("button");
    const rect = vi.spyOn(anchor, "getBoundingClientRect").mockReturnValue(new DOMRect(200, 100, 224, 36));
    expect(resolveUiSideOverlayPosition({ anchor, preset: "cascade-menu", estimatedContentHeight: 158 }))
      .toEqual({ left: 430, top: 100, width: 224, maxHeight: 158, placement: "bottom" });
    rect.mockReturnValue(new DOMRect(564, 500, 224, 36));
    expect(resolveUiSideOverlayPosition({ anchor, preset: "cascade-menu", estimatedContentHeight: 900 }))
      .toEqual({ left: 334, top: 268, width: 224, maxHeight: 320, placement: "bottom" });
    rect.mockRestore();
  });
});

describe("short viewport hard limits", () => {
  it.each(PRESET_EXPECTATIONS.flatMap(({preset, viewportInset}) =>
    (["auto", "top", "bottom"] as const).map((placement) => ({preset, viewportInset, placement})),
  ))("keeps $preset inside a short viewport with $placement placement", ({preset, viewportInset, placement}) => {
    setViewport(320, 120);
    const position = resolveUiAnchoredOverlayPosition({
      anchor: createAnchor({left: 240, top: 45, bottom: 77, width: 80}), preset, placement,
    });
    const top = position.top ?? window.innerHeight - position.bottom! - position.maxHeight;
    expect(position.maxHeight).toBeGreaterThanOrEqual(0);
    expect(top).toBeGreaterThanOrEqual(viewportInset);
    expect(top + position.maxHeight).toBeLessThanOrEqual(120 - viewportInset);
    expect(position.left).toBeGreaterThanOrEqual(viewportInset);
    expect(position.left + position.width).toBeLessThanOrEqual(320 - viewportInset);
  });
  it("never emits negative dimensions for a temporarily collapsed viewport", () => {
    setViewport(0, 0);
    const position = resolveUiAnchoredOverlayPosition({anchor: createAnchor(), preset: "form-picker", placement: "auto"});
    expect(position.width).toBe(0);
    expect(position.maxHeight).toBe(0);
  });
});
