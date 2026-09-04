// INPUT: 锚点 DOM、视口与调用方已解析的原始定位约束。
// OUTPUT: 被视口边界夹紧的浮层坐标、宽度、最大高度与最终上下方向。
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
            viewportHeight - rect.top + gap,
          ),
        }
      : {
          top: Math.min(
            rect.bottom + gap,
            viewportHeight - viewportMargin - resolvedMaxHeight,
          ),
        }),
  };
}
