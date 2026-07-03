"use client";

import { type ReactNode, type RefObject, useEffect, useRef } from "react";
import { createPortal } from "react-dom";

import { closeOnEscape } from "@/shared/ui/dialog/dialog-keyboard";

import { PICKER_POPOVER_CLASS_NAME } from "./picker-styles";

interface PickerPopoverProps {
  anchorRef: RefObject<HTMLElement | null>;
  children: ReactNode;
  isOpen: boolean;
  onClose: () => void;
}

export function PickerPopover({ anchorRef: anchorRef, children, isOpen: isOpen, onClose: onClose }: PickerPopoverProps) {
  const popoverRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!isOpen) {
      return;
    }

    const handlePointerDown = (event: MouseEvent) => {
      const anchor = anchorRef.current;
      const popover = popoverRef.current;
      if (anchor?.contains(event.target as Node) || popover?.contains(event.target as Node)) {
        return;
      }
      onClose();
    };

    const onKeyDown = (event: KeyboardEvent) => closeOnEscape(event, onClose);

    document.addEventListener("mousedown", handlePointerDown, true);
    document.addEventListener("keydown", onKeyDown, true);
    return () => {
      document.removeEventListener("mousedown", handlePointerDown, true);
      document.removeEventListener("keydown", onKeyDown, true);
    };
  }, [anchorRef, isOpen, onClose]);

  if (!isOpen || !anchorRef.current) {
    return null;
  }

  const rect = anchorRef.current.getBoundingClientRect();
  const modalRoot = document.querySelector("[data-modal-root='true']");
  return createPortal(
    <div
      ref={popoverRef}
      className={PICKER_POPOVER_CLASS_NAME}
      style={{
        top: rect.bottom + 10,
        left: Math.max(24, rect.left),
        background: "rgba(252, 253, 255, 0.98)",
        borderColor: "rgba(214, 224, 237, 0.96)",
        backdropFilter: "blur(18px)",
        WebkitBackdropFilter: "blur(18px)",
      }}
    >
      {children}
    </div>,
    modalRoot ?? document.body,
  );
}
