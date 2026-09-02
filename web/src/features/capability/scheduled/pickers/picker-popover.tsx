"use client";

import { type ReactNode, type RefObject, useCallback } from "react";
import { createPortal } from "react-dom";

import { cn } from "@/shared/ui/class-name";
import { useAnchoredOverlayLayer } from "@/shared/ui/overlay/anchored-overlay-layer";
import { resolveAnchoredOverlayPosition } from "@/shared/ui/overlay/anchored-overlay-model";
import { OPEN_OVERLAY_DATA_ATTRIBUTES } from "@/shared/ui/overlay/overlay-contract";
import {
  ANCHORED_OVERLAY_MOTION_CLASS_NAME,
  OVERLAY_SURFACE_CLASS_NAME,
} from "@/shared/ui/overlay/overlay-styles";

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
    disabled: false,
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
      className={cn(
        "fixed left-0 top-0 ui-layer-dialog-interaction overflow-y-auto p-3",
        OVERLAY_SURFACE_CLASS_NAME,
        ANCHORED_OVERLAY_MOTION_CLASS_NAME,
      )}
      data-placement={overlayPosition?.placement ?? "bottom"}
      style={overlayStyle}
      {...OPEN_OVERLAY_DATA_ATTRIBUTES}
    >
      {children}
    </div>,
    portalContainer,
  );
}
