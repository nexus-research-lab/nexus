"use client";

import {
  type CSSProperties,
  type RefObject,
  useCallback,
  useEffect,
  useId,
  useLayoutEffect,
  useRef,
  useState,
} from "react";

import type { UiAnchoredOverlayPosition } from "./anchored-overlay-model";

interface AnchoredOverlayLayerOptions<T extends HTMLElement> {
  anchorRef: RefObject<T | null>;
  disabled?: boolean;
  estimatePosition: (anchor: T) => UiAnchoredOverlayPosition;
  isOpen: boolean;
  onClose: () => void;
}

/** 统一锚定浮层的浏览器生命周期，消费者只负责交互语义和内容。 */
export function useAnchoredOverlayLayer<T extends HTMLElement>({
  anchorRef,
  disabled = false,
  estimatePosition,
  isOpen,
  onClose,
}: AnchoredOverlayLayerOptions<T>) {
  const overlayId = useId();
  const overlayRef = useRef<HTMLDivElement>(null);
  const [position, setPosition] = useState<UiAnchoredOverlayPosition | null>(null);

  const updatePosition = useCallback(() => {
    const anchor = anchorRef.current;
    if (anchor) {
      setPosition(estimatePosition(anchor));
    }
  }, [anchorRef, estimatePosition]);

  useEffect(() => {
    if (!isOpen || disabled) {
      return;
    }

    const handlePointerDown = (event: PointerEvent) => {
      const target = event.target as Node;
      if (
        anchorRef.current?.contains(target)
        || overlayRef.current?.contains(target)
      ) {
        return;
      }
      onClose();
    };
    const handleKeyDown = (event: globalThis.KeyboardEvent) => {
      if (event.key !== "Escape") {
        return;
      }
      onClose();
      anchorRef.current?.focus();
    };

    document.addEventListener("pointerdown", handlePointerDown, true);
    document.addEventListener("keydown", handleKeyDown);
    window.addEventListener("resize", updatePosition);
    window.addEventListener("scroll", updatePosition, true);
    return () => {
      document.removeEventListener("pointerdown", handlePointerDown, true);
      document.removeEventListener("keydown", handleKeyDown);
      window.removeEventListener("resize", updatePosition);
      window.removeEventListener("scroll", updatePosition, true);
    };
  }, [anchorRef, disabled, isOpen, onClose, updatePosition]);

  useLayoutEffect(() => {
    if (isOpen && !disabled) {
      updatePosition();
    }
  }, [disabled, isOpen, updatePosition]);

  const overlayStyle: CSSProperties = {
    bottom: position?.bottom,
    left: position?.left,
    maxHeight: position?.maxHeight,
    top: position?.top,
    visibility: position ? "visible" : "hidden",
    width: position?.width,
  };
  const portalContainer = typeof document === "undefined"
    ? null
    : anchorRef.current?.closest("[data-modal-root='true']") ?? document.body;

  return {
    overlayId,
    overlayPosition: position,
    overlayRef,
    overlayStyle,
    portalContainer,
    updateOverlayPosition: updatePosition,
  };
}
