// INPUT: Picker 锚点、展开状态、关闭命令、可访问名称与选择内容。
// OUTPUT: 复用共享 anchored-overlay 生命周期和表面的日期/时间浮层。
// POS: Scheduled Picker 浮层边界；不拥有字段触发器或选择状态。

"use client";

import { type ReactNode, type RefObject, useCallback } from "react";
import { createPortal } from "react-dom";

import { cn } from "@/shared/ui/class-name";
import { useAnchoredOverlayLayer } from "@/shared/ui/overlay/anchored-overlay-layer";
import { resolveUiAnchoredOverlayPosition } from "@/shared/ui/overlay/anchored-overlay-layout";
import { OPEN_OVERLAY_DATA_ATTRIBUTES } from "@/shared/ui/overlay/overlay-contract";
import {
  ANCHORED_OVERLAY_MOTION_CLASS_NAME,
  OVERLAY_SURFACE_CLASS_NAME,
} from "@/shared/ui/overlay/overlay-styles";

interface PickerPopoverProps {
  anchorRef: RefObject<HTMLElement | null>;
  ariaLabel: string;
  children: ReactNode;
  isOpen: boolean;
  onClose: () => void;
}

export function PickerPopover({
  anchorRef,
  ariaLabel,
  children,
  isOpen,
  onClose,
}: PickerPopoverProps) {
  const estimatePosition = useCallback(
    (anchor: HTMLElement) => resolveUiAnchoredOverlayPosition({
      anchor,
      estimatedContentHeight: 288,
      placement: "auto",
      preset: "form-picker",
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
      aria-label={ariaLabel}
      className={cn(
        "fixed left-0 top-0 ui-layer-dialog-interaction overflow-y-auto p-3",
        OVERLAY_SURFACE_CLASS_NAME,
        ANCHORED_OVERLAY_MOTION_CLASS_NAME,
      )}
      data-placement={overlayPosition?.placement ?? "bottom"}
      role="dialog"
      style={overlayStyle}
      {...OPEN_OVERLAY_DATA_ATTRIBUTES}
    >
      {children}
    </div>,
    portalContainer,
  );
}
