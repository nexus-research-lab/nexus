// INPUT: 决策弹窗内容、动作及可选异步执行状态。
// OUTPUT: 共享模态骨架，以及执行中防重复提交的确认/取消动作。
// POS: Decision Dialog 结构层；不解释业务失败或自行重试。
"use client";

import { LoaderCircle } from "lucide-react";
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
  busy?: boolean;
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
        describedBy={describedBy}
        initialFocusRef={initialFocusRef}
        labelledBy={labelledBy}
        layer="dialog"
        onClose={onClose}
      >
        <UiDialogShell size="sm">{children}</UiDialogShell>
      </UiDialogBackdrop>
    </UiDialogPortal>
  );
}

export function DecisionDialogActions({
  busy = false,
  cancelText,
  confirmButtonRef,
  confirmClassName,
  confirmText,
  confirmTone = "primary",
  onCancel,
  onConfirm,
}: DecisionDialogActionsProps) {
  return (
    <UiDialogFooter className="!border-t-0 !bg-transparent !px-5 !pb-5 !pt-0">
      <button
        className={getDialogActionClassName("default")}
        disabled={busy}
        onClick={onCancel}
        type="button"
      >
        {cancelText}
      </button>
      <button
        aria-busy={busy}
        className={getDialogActionClassName(confirmTone, confirmClassName)}
        disabled={busy}
        onClick={onConfirm}
        ref={confirmButtonRef}
        type="button"
      >
        {busy ? <LoaderCircle className="h-4 w-4 animate-spin" /> : null}
        {confirmText}
      </button>
    </UiDialogFooter>
  );
}
