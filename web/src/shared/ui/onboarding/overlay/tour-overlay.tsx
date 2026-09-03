"use client";

import { useEffect } from "react";
import { createPortal } from "react-dom";

import type { OnboardingTourDefinition } from "../tour-contract";
import { TourOverlayCard } from "./tour-overlay-card";
import {
  getPopoverPosition,
  resolveTourPlacement,
} from "./tour-overlay-geometry";
import { useTourOverlayLayout } from "./use-tour-overlay-layout";

interface OnboardingTourOverlayProps {
  onClose: (options?: { completed?: boolean }) => void;
  onNext: () => void;
  onPrevious: () => void;
  stepIndex: number;
  tour: OnboardingTourDefinition;
}

export function OnboardingTourOverlay({
  onClose,
  onNext,
  onPrevious,
  stepIndex,
  tour,
}: OnboardingTourOverlayProps) {
  const step = tour.steps[stepIndex];
  const { cardRef, popoverSize, targetRect } = useTourOverlayLayout(step);

  useEffect(() => {
    const handleEscape = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        onClose();
      }
    };
    window.addEventListener("keydown", handleEscape);
    return () => window.removeEventListener("keydown", handleEscape);
  }, [onClose]);

  useEffect(() => {
    if (!step) {
      return undefined;
    }
    const handlePageInteraction = (event: PointerEvent) => {
      const target = event.target;
      if (
        target instanceof Element
        && target.closest("[data-onboarding-tour-card]")
      ) {
        return;
      }
      // 导览只解释当前界面，不应吞掉用户真正想执行的点击。
      onClose();
    };
    document.addEventListener("pointerdown", handlePageInteraction, true);
    return () => {
      document.removeEventListener("pointerdown", handlePageInteraction, true);
    };
  }, [onClose, step]);

  if (typeof document === "undefined" || !step) {
    return null;
  }

  const placement = resolveTourPlacement(step);
  const position = getPopoverPosition(
    placement,
    targetRect,
    window.innerWidth,
    window.innerHeight,
    popoverSize,
    16,
  );

  return createPortal(
    <div className="pointer-events-none fixed inset-0 ui-layer-tour">
      {!targetRect ? (
        <div
          className="absolute inset-0 bg-[rgba(11,16,24,0.42)]"
          role="presentation"
        />
      ) : null}
      {targetRect ? <TourTargetHighlight targetRect={targetRect} /> : null}
      <div
        className="pointer-events-auto absolute"
        data-onboarding-tour-card
        style={{ left: position.left, top: position.top }}
      >
        <TourOverlayCard
          isLastStep={stepIndex >= tour.steps.length - 1}
          onClose={onClose}
          onNext={onNext}
          onPrevious={onPrevious}
          placement={placement}
          ref={cardRef}
          step={step}
          stepCount={tour.steps.length}
          stepIndex={stepIndex}
        />
      </div>
    </div>,
    document.body,
  );
}

function TourTargetHighlight({ targetRect }: { targetRect: DOMRect }) {
  return (
    <div
      className="tour-target-highlight pointer-events-none absolute"
      style={{
        height: targetRect.height + 12,
        left: targetRect.left - 6,
        top: targetRect.top - 6,
        width: targetRect.width + 12,
      }}
    />
  );
}
