export type UiAnchoredOverlayPlacement = "auto" | "bottom" | "top";

export interface UiAnchoredOverlayPosition {
  bottom?: number;
  left: number;
  maxHeight: number;
  placement: "bottom" | "top";
  top?: number;
  width: number;
}

interface ResolveAnchoredOverlayPositionOptions {
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
  const left = Math.min(
    Math.max(viewportMargin, rect.left),
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
