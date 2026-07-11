"use client";

import { type ReactNode, type RefObject, useCallback } from "react";
import { createPortal } from "react-dom";

import { useAnchoredOverlayLayer } from "@/shared/ui/overlay/anchored-overlay-layer";
import { resolveAnchoredOverlayPosition } from "@/shared/ui/overlay/anchored-overlay-model";

import { PICKER_POPOVER_CLASS_NAME } from "./picker-styles";

interface PickerPopoverProps {
  anchorRef: RefObject<HTMLElement | null>;
  children: ReactNode;
  isOpen: boolean;
  onClose: () => void;
}

export function PickerPopover({
  anchorRef,
  children,
  isOpen,
  onClose,
}: PickerPopoverProps) {
  const estimatePosition = useCallback(
    (anchor: HTMLElement) => resolveAnchoredOverlayPosition({
      anchor,
      estimatedHeight: 288,
      gap: 10,
      maxHeight: 320,
      minHeight: 240,
      minWidth: 480,
      placement: "auto",
      viewportMargin: 24,
    }),
    [],
  );
  const {
    overlayPosition,
    overlayRef,
    overlayStyle,
    portalContainer,
  } = useAnchoredOverlayLayer({
    anchorRef,
    estimatePosition,
    isOpen,
    onClose,
  });

  if (!isOpen || !anchorRef.current || !portalContainer) {
    return null;
  }

  return createPortal(
    <div
      ref={overlayRef}
      className={PICKER_POPOVER_CLASS_NAME}
      data-placement={overlayPosition?.placement ?? "bottom"}
      style={{
        ...overlayStyle,
        background: "rgba(252, 253, 255, 0.98)",
        borderColor: "rgba(214, 224, 237, 0.96)",
        backdropFilter: "blur(18px)",
        WebkitBackdropFilter: "blur(18px)",
      }}
    >
      {children}
    </div>,
    portalContainer,
  );
}
