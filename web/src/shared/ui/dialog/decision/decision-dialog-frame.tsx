"use client";

import type { ReactNode, RefObject } from "react";

import {
  UiDialogBackdrop,
  UiDialogFooter,
  UiDialogPortal,
  UiDialogShell,
} from "@/shared/ui/dialog/dialog";
import { getDialogActionClassName } from "@/shared/ui/dialog/dialog-styles";

interface DecisionDialogFrameProps {
  children: ReactNode;
  describedBy?: string;
  initialFocusRef: RefObject<HTMLElement | null>;
  labelledBy: string;
  onClose: () => void;
}

interface DecisionDialogActionsProps {
  cancelText: string;
  confirmButtonRef?: RefObject<HTMLButtonElement | null>;
  confirmClassName?: string;
  confirmText: string;
  confirmTone?: "danger" | "primary";
  onCancel: () => void;
  onConfirm: () => void;
}

export function DecisionDialogFrame({
  children,
  describedBy,
  initialFocusRef,
  labelledBy,
  onClose,
}: DecisionDialogFrameProps) {
  return (
    <UiDialogPortal>
      <UiDialogBackdrop
        className="z-[9999]"
        describedBy={describedBy}
        initialFocusRef={initialFocusRef}
        labelledBy={labelledBy}
        onClose={onClose}
      >
        <UiDialogShell size="sm">{children}</UiDialogShell>
      </UiDialogBackdrop>
    </UiDialogPortal>
  );
}

export function DecisionDialogActions({
  cancelText,
  confirmButtonRef,
  confirmClassName,
  confirmText,
  confirmTone = "primary",
  onCancel,
  onConfirm,
}: DecisionDialogActionsProps) {
  return (
    <UiDialogFooter>
      <button
        className={getDialogActionClassName("default")}
        onClick={onCancel}
        type="button"
      >
        {cancelText}
      </button>
      <button
        className={getDialogActionClassName(confirmTone, confirmClassName)}
        onClick={onConfirm}
        ref={confirmButtonRef}
        type="button"
      >
        {confirmText}
      </button>
    </UiDialogFooter>
  );
}
