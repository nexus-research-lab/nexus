"use client";

import { type ReactNode, type RefObject, useEffect, useRef } from "react";
import { createPortal } from "react-dom";

import { PICKER_POPOVER_CLASS_NAME } from "./picker-utils";

interface PickerPopoverProps {
  anchor_ref: RefObject<HTMLElement | null>;
  children: ReactNode;
  is_open: boolean;
  on_close: () => void;
}

export function PickerPopover({ anchor_ref, children, is_open, on_close }: PickerPopoverProps) {
  const popover_ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!is_open) {
      return;
    }

    const handle_pointer_down = (event: MouseEvent) => {
      const anchor = anchor_ref.current;
      const popover = popover_ref.current;
      if (anchor?.contains(event.target as Node) || popover?.contains(event.target as Node)) {
        return;
      }
      on_close();
    };

    const handle_key_down = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        on_close();
      }
    };

    document.addEventListener("mousedown", handle_pointer_down, true);
    document.addEventListener("keydown", handle_key_down, true);
    return () => {
      document.removeEventListener("mousedown", handle_pointer_down, true);
      document.removeEventListener("keydown", handle_key_down, true);
    };
  }, [anchor_ref, is_open, on_close]);

  if (!is_open || !anchor_ref.current) {
    return null;
  }

  const rect = anchor_ref.current.getBoundingClientRect();
  const modal_root = document.querySelector("[data-modal-root='true']");
  return createPortal(
    <div
      ref={popover_ref}
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
    modal_root ?? document.body,
  );
}
