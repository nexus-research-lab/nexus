"use client";

import {
  useLayoutEffect,
  useState,
  type CSSProperties,
  type RefObject,
} from "react";
import { createPortal } from "react-dom";

interface FocusRect {
  bottom: number;
  height: number;
  left: number;
  right: number;
  top: number;
  width: number;
}

interface HomeOnboardingFocusLayerProps {
  enabled: boolean;
  targetRef: RefObject<HTMLElement | null>;
}

function readFocusRect(element: HTMLElement): FocusRect {
  const rect = element.getBoundingClientRect();
  const top = Math.max(0, rect.top);
  const left = Math.max(0, rect.left);
  const right = Math.min(window.innerWidth, rect.right);
  const bottom = Math.min(window.innerHeight, rect.bottom);
  return {
    bottom,
    height: Math.max(0, bottom - top),
    left,
    right,
    top,
    width: Math.max(0, right - left),
  };
}

function FocusMask({
  className,
  style,
}: {
  className: string;
  style: CSSProperties;
}) {
  return (
    <div
      aria-hidden="true"
      className={`nexus-onboarding-focus-mask ${className}`}
      style={style}
    />
  );
}

export function HomeOnboardingFocusLayer({
  enabled,
  targetRef,
}: HomeOnboardingFocusLayerProps) {
  const [focusRect, setFocusRect] = useState<FocusRect | null>(null);

  useLayoutEffect(() => {
    if (!enabled || typeof document === "undefined") {
      return;
    }

    const element = targetRef.current;
    if (!element) {
      return;
    }

    const update = () => setFocusRect(readFocusRect(element));
    const resizeObserver = new ResizeObserver(update);
    resizeObserver.observe(element);
    window.addEventListener("resize", update);
    window.addEventListener("scroll", update, true);
    update();

    return () => {
      resizeObserver.disconnect();
      window.removeEventListener("resize", update);
      window.removeEventListener("scroll", update, true);
    };
  }, [enabled, targetRef]);

  if (!enabled || !focusRect || typeof document === "undefined") {
    return null;
  }

  const viewportWidth = window.innerWidth;
  const viewportHeight = window.innerHeight;
  const maskStyle = {
    top: 0,
    left: 0,
    width: viewportWidth,
    height: focusRect.top,
  };
  const bottomMaskStyle = {
    top: focusRect.bottom,
    left: 0,
    width: viewportWidth,
    height: Math.max(0, viewportHeight - focusRect.bottom),
  };
  const leftMaskStyle = {
    top: focusRect.top,
    left: 0,
    width: focusRect.left,
    height: focusRect.height,
  };
  const rightMaskStyle = {
    top: focusRect.top,
    left: focusRect.right,
    width: Math.max(0, viewportWidth - focusRect.right),
    height: focusRect.height,
  };

  return createPortal(
    <>
      <FocusMask className="nexus-onboarding-focus-mask-top" style={maskStyle} />
      <FocusMask
        className="nexus-onboarding-focus-mask-bottom"
        style={bottomMaskStyle}
      />
      <FocusMask
        className="nexus-onboarding-focus-mask-left"
        style={leftMaskStyle}
      />
      <FocusMask
        className="nexus-onboarding-focus-mask-right"
        style={rightMaskStyle}
      />
      <div
        aria-hidden="true"
        className="nexus-onboarding-focus-frame"
        style={{
          height: focusRect.height,
          left: focusRect.left,
          top: focusRect.top,
          width: focusRect.width,
        }}
      />
    </>,
    document.body,
  );
}
