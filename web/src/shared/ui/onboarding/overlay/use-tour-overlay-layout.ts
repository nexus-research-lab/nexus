// INPUT: 当前 Tour 步骤、锚点 DOM、卡片尺寸及浏览器视口变化。
// OUTPUT: 同步后的目标矩形、卡片尺寸、视口尺寸与卡片测量 ref。
// POS: Onboarding Tour 测量生命周期；不决定摆放策略或渲染内容。

import { useEffect, useLayoutEffect, useRef, useState } from "react";

import type { OnboardingTourStep } from "../tour-contract";
import {
  estimateTourCardHeight,
  type PopoverSize,
} from "./tour-overlay-geometry";

function createInitialPopoverSize(step?: OnboardingTourStep): PopoverSize {
  const viewportWidth = typeof window === "undefined" ? 344 : window.innerWidth;
  return {
    height: estimateTourCardHeight(step),
    width: Math.min(344, Math.max(viewportWidth - 32, 1)),
  };
}

function readViewportSize(): PopoverSize {
  if (typeof window === "undefined") {
    return { height: 768, width: 344 };
  }
  return { height: window.innerHeight, width: window.innerWidth };
}

export function useTourOverlayLayout(step?: OnboardingTourStep) {
  const cardRef = useRef<HTMLDivElement | null>(null);
  const [targetRect, setTargetRect] = useState<DOMRect | null>(null);
  const [popoverSize, setPopoverSize] = useState<PopoverSize>(() => (
    createInitialPopoverSize(step)
  ));
  const [viewportSize, setViewportSize] = useState<PopoverSize>(readViewportSize);

  useEffect(() => {
    if (typeof window === "undefined") {
      return;
    }

    let targetElement: HTMLElement | null = null;
    let targetObserver: ResizeObserver | null = null;
    function bindTarget(nextTarget: HTMLElement | null): void {
      if (nextTarget === targetElement) {
        return;
      }
      targetObserver?.disconnect();
      targetElement = nextTarget;
      targetObserver = null;
      if (nextTarget) {
        targetObserver = new ResizeObserver(updateTargetRect);
        targetObserver.observe(nextTarget);
        nextTarget.scrollIntoView({ block: "nearest", inline: "nearest" });
      }
    }
    function updateTargetRect(): void {
      const nextViewportSize = readViewportSize();
      setViewportSize((current) => (
        current.width === nextViewportSize.width
        && current.height === nextViewportSize.height
          ? current
          : nextViewportSize
      ));
      const nextTarget = step?.target
        ? document.querySelector<HTMLElement>(`[data-tour-anchor="${step.target}"]`)
        : null;
      bindTarget(nextTarget);
      setTargetRect(targetElement?.getBoundingClientRect() ?? null);
    }

    updateTargetRect();
    const frameId = requestAnimationFrame(updateTargetRect);
    window.addEventListener("resize", updateTargetRect);
    window.addEventListener("scroll", updateTargetRect, true);

    return () => {
      cancelAnimationFrame(frameId);
      targetObserver?.disconnect();
      window.removeEventListener("resize", updateTargetRect);
      window.removeEventListener("scroll", updateTargetRect, true);
    };
  }, [step?.target]);

  useLayoutEffect(() => {
    const element = cardRef.current;
    if (!element || typeof window === "undefined") {
      return;
    }

    const updateSize = () => {
      const rect = element.getBoundingClientRect();
      if (rect.width <= 0 || rect.height <= 0) {
        return;
      }
      setPopoverSize((current) => (
        Math.abs(current.width - rect.width) < 1
        && Math.abs(current.height - rect.height) < 1
          ? current
          : { height: rect.height, width: rect.width }
      ));
    };

    updateSize();
    const observer = new ResizeObserver(updateSize);
    observer.observe(element);
    window.addEventListener("resize", updateSize);

    return () => {
      observer.disconnect();
      window.removeEventListener("resize", updateSize);
    };
  }, [step]);

  return { cardRef, popoverSize, targetRect, viewportSize };
}
