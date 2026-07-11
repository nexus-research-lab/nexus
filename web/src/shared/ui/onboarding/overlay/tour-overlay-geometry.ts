import type { OnboardingTourStep, TourPlacement } from "../tour-provider";

export type { TourPlacement } from "../tour-provider";

export interface PopoverPosition {
  left: number;
  top: number;
}

export interface PopoverSize {
  height: number;
  width: number;
}

interface PositionContext {
  cardHeight: number;
  cardWidth: number;
  targetRect: DOMRect;
  topClearance: number;
  viewportHeight: number;
  viewportWidth: number;
}

type PositionResolver = (context: PositionContext) => PopoverPosition;

const TARGET_POSITION_RESOLVERS: Record<
  Exclude<TourPlacement, "center">,
  PositionResolver
> = {
  bottom: ({
    cardHeight,
    cardWidth,
    targetRect,
    topClearance,
    viewportHeight,
    viewportWidth,
  }) => ({
    left: clampPopoverLeft(
      targetRect.left + targetRect.width / 2 - cardWidth / 2,
      cardWidth,
      viewportWidth,
    ),
    top: clampPopoverTop(
      targetRect.bottom + 16,
      cardHeight,
      viewportHeight,
      topClearance,
    ),
  }),
  left: ({
    cardHeight,
    cardWidth,
    targetRect,
    topClearance,
    viewportHeight,
    viewportWidth,
  }) => ({
    left: clampPopoverLeft(
      targetRect.left - cardWidth - 16,
      cardWidth,
      viewportWidth,
    ),
    top: clampPopoverTop(
      targetRect.top + targetRect.height / 2 - cardHeight / 2,
      cardHeight,
      viewportHeight,
      topClearance,
    ),
  }),
  right: ({
    cardHeight,
    cardWidth,
    targetRect,
    topClearance,
    viewportHeight,
    viewportWidth,
  }) => ({
    left: clampPopoverLeft(
      targetRect.right + 16,
      cardWidth,
      viewportWidth,
    ),
    top: clampPopoverTop(
      targetRect.top + targetRect.height / 2 - cardHeight / 2,
      cardHeight,
      viewportHeight,
      topClearance,
    ),
  }),
  top: ({
    cardHeight,
    cardWidth,
    targetRect,
    topClearance,
    viewportHeight,
    viewportWidth,
  }) => ({
    left: clampPopoverLeft(
      targetRect.left + targetRect.width / 2 - cardWidth / 2,
      cardWidth,
      viewportWidth,
    ),
    top: clampPopoverTop(
      targetRect.top - cardHeight - 16,
      cardHeight,
      viewportHeight,
      topClearance,
    ),
  }),
};

export function estimateTourCardHeight(step?: OnboardingTourStep): number {
  if (!step) {
    return 180;
  }

  const optionalHeights = [
    step.image ? 136 : 0,
    step.description ? 24 : 0,
    (step.items?.length ?? 0) * 34,
  ];
  return optionalHeights.reduce((height, value) => height + value, 104);
}

export function resolveTourPlacement(
  step: OnboardingTourStep,
): TourPlacement {
  return step.placement ?? (step.target ? "right" : "center");
}

export function getPopoverPosition(
  placement: TourPlacement,
  targetRect: DOMRect | null,
  viewportWidth: number,
  viewportHeight: number,
  popoverSize: PopoverSize,
  topClearance: number,
): PopoverPosition {
  const cardWidth = Math.min(popoverSize.width, Math.max(viewportWidth - 32, 1));
  const cardHeight = Math.min(popoverSize.height, Math.max(viewportHeight - 32, 1));
  if (!targetRect || placement === "center") {
    return {
      left: clampPopoverLeft(
        viewportWidth / 2 - cardWidth / 2,
        cardWidth,
        viewportWidth,
      ),
      top: clampPopoverTop(
        viewportHeight / 2 - cardHeight / 2,
        cardHeight,
        viewportHeight,
        topClearance,
      ),
    };
  }

  return TARGET_POSITION_RESOLVERS[placement]({
    cardHeight,
    cardWidth,
    targetRect,
    topClearance,
    viewportHeight,
    viewportWidth,
  });
}

function clampPopoverTop(
  top: number,
  cardHeight: number,
  viewportHeight: number,
  topClearance: number,
): number {
  const bottomLimit = viewportHeight - cardHeight - 16;
  return bottomLimit < topClearance
    ? Math.max(16, bottomLimit)
    : Math.max(topClearance, Math.min(top, bottomLimit));
}

function clampPopoverLeft(
  left: number,
  cardWidth: number,
  viewportWidth: number,
): number {
  return Math.max(16, Math.min(left, viewportWidth - cardWidth - 16));
}
