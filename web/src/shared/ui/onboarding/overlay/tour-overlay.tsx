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
    <div className="fixed inset-0 z-[11000]">
      <div
        className="absolute inset-0 bg-[rgba(11,16,24,0.42)]"
        onClick={() => onClose()}
        role="presentation"
      />
      {targetRect ? <TourTargetHighlight targetRect={targetRect} /> : null}
      <div
        className="absolute"
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
      className="pointer-events-none absolute rounded-[10px] border border-[color:color-mix(in_srgb,var(--primary)_38%,white)] shadow-[0_0_0_9999px_rgba(11,16,24,0.16)] transition-[top,left,width,height] duration-(--motion-duration-fast)"
      style={{
        height: targetRect.height + 12,
        left: targetRect.left - 6,
        top: targetRect.top - 6,
        width: targetRect.width + 12,
      }}
    />
  );
}
