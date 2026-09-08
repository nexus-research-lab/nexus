// INPUT: 锚点 DOM/指针位置、视口与调用方已解析的原始定位约束。
// OUTPUT: 上下锚定、指针或侧向浮层的视口安全坐标、宽度、最大高度与动画方向。
// POS: 锚定浮层底层几何求解器；语义 preset 及其数值归 anchored-overlay-layout 所有。

export type UiAnchoredOverlayPlacement = "auto" | "bottom" | "top";
export type UiAnchoredOverlayAlignment = "center" | "end" | "start";

export interface UiAnchoredOverlayPosition {
  bottom?: number;
  left: number;
  maxHeight: number;
  placement: "bottom" | "top";
  top?: number;
  width: number;
}

// areAnchoredOverlayPositionsEqual 判断两次定位是否产生相同可见几何。
export function areAnchoredOverlayPositionsEqual(
  current: UiAnchoredOverlayPosition | null,
  next: UiAnchoredOverlayPosition | null,
): boolean {
  if (current === next) {
    return true;
  }
  if (!current || !next) {
    return false;
  }
  return current.bottom === next.bottom
    && current.left === next.left
    && current.maxHeight === next.maxHeight
    && current.placement === next.placement
    && current.top === next.top
    && current.width === next.width;
}

interface ResolveAnchoredOverlayPositionOptions {
  align?: UiAnchoredOverlayAlignment;
  anchor: HTMLElement;
  estimatedHeight: number;
  gap?: number;
  maxHeight: number;
  minHeight: number;
  minWidth?: number;
  placement: UiAnchoredOverlayPlacement;
  viewportMargin?: number;
}

const DEFAULT_OVERLAY_GAP = 6;
const DEFAULT_VIEWPORT_MARGIN = 12;

export function resolveAnchoredOverlayPosition({
  align = "start",
  anchor,
  estimatedHeight,
  gap = DEFAULT_OVERLAY_GAP,
  maxHeight,
  minHeight,
  minWidth = 0,
  placement,
  viewportMargin = DEFAULT_VIEWPORT_MARGIN,
}: ResolveAnchoredOverlayPositionOptions): UiAnchoredOverlayPosition {
  const rect = anchor.getBoundingClientRect();
  const viewportWidth = window.innerWidth;
  const viewportHeight = window.innerHeight;
  const availableAbove = Math.max(0, rect.top - viewportMargin);
  const availableBelow = Math.max(
    0,
    viewportHeight - rect.bottom - viewportMargin,
  );
  const placeAbove = placement === "top"
    || (placement === "auto"
      && availableBelow < estimatedHeight
      && availableAbove > availableBelow);
  const availableSpace = placeAbove ? availableAbove : availableBelow;
  const resolvedMaxHeight = Math.min(
    maxHeight,
    estimatedHeight,
    Math.max(minHeight, availableSpace - gap),
  );
  const width = Math.min(
    Math.max(rect.width, minWidth),
    viewportWidth - viewportMargin * 2,
  );
  const preferredLeft = align === "end"
    ? rect.right - width
    : align === "center"
      ? rect.left + (rect.width - width) / 2
      : rect.left;
  const left = Math.min(
    Math.max(viewportMargin, preferredLeft),
    Math.max(viewportMargin, viewportWidth - width - viewportMargin),
  );

  return {
    left,
    maxHeight: resolvedMaxHeight,
    placement: placeAbove ? "top" : "bottom",
    width,
    ...(placeAbove
      ? {
          bottom: Math.max(
            viewportMargin,
            Math.min(viewportHeight - rect.top + gap, viewportHeight - viewportMargin - resolvedMaxHeight),
          ),
        }
      : {
          top: Math.max(
            viewportMargin,
            Math.min(rect.bottom + gap, viewportHeight - viewportMargin - resolvedMaxHeight),
          ),
        }),
  };
}

interface FreeOverlayBounds {
  estimatedHeight: number;
  maxHeight: number;
  minHeight: number;
  minWidth: number;
  viewportMargin: number;
}

function getFreeOverlayDimensions(bounds: FreeOverlayBounds) {
  return {
    width: Math.max(0, Math.min(bounds.minWidth, window.innerWidth - bounds.viewportMargin * 2)),
    maxHeight: Math.max(0, Math.min(
      Math.max(bounds.minHeight, bounds.estimatedHeight),
      bounds.maxHeight,
      window.innerHeight - bounds.viewportMargin * 2,
    )),
  };
}

function clampOverlayOffset(preferred: number, size: number, viewport: number, margin: number): number {
  return Math.max(margin, Math.min(preferred, viewport - margin - size));
}

/** 指针菜单保持调用点，空间不足时整体挪回视口，而不是按文件类型猜测高度。 */
export function resolvePointOverlayPosition({ point, ...bounds }: FreeOverlayBounds & {
  point: { x: number; y: number };
}): UiAnchoredOverlayPosition {
  const dimensions = getFreeOverlayDimensions(bounds);
  const top = clampOverlayOffset(point.y, dimensions.maxHeight, window.innerHeight, bounds.viewportMargin);
  return {
    ...dimensions,
    left: clampOverlayOffset(point.x, dimensions.width, window.innerWidth, bounds.viewportMargin),
    top,
    placement: top < point.y ? "top" : "bottom",
  };
}

/** 侧向子层沿真实行对齐，右侧不足则向左；超高内容只在层内滚动。 */
export function resolveSideOverlayPosition({ anchor, gap, ...bounds }: FreeOverlayBounds & {
  anchor: HTMLElement;
  gap: number;
}): UiAnchoredOverlayPosition {
  const rect = anchor.getBoundingClientRect();
  const dimensions = getFreeOverlayDimensions(bounds);
  const preferredLeft = rect.right + gap + dimensions.width <= window.innerWidth - bounds.viewportMargin
    ? rect.right + gap : rect.left - gap - dimensions.width;
  return {
    ...dimensions,
    left: clampOverlayOffset(preferredLeft, dimensions.width, window.innerWidth, bounds.viewportMargin),
    top: clampOverlayOffset(rect.top, dimensions.maxHeight, window.innerHeight, bounds.viewportMargin),
    placement: "bottom",
  };
}
