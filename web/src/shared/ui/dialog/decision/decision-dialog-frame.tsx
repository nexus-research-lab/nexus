// INPUT: 决策弹窗内容、动作及可选异步执行状态。
// OUTPUT: 共享模态骨架，以及执行中防重复提交的确认/取消动作。
// POS: Decision Dialog 结构层；不解释业务失败或自行重试。
"use client";

import { LoaderCircle } from "lucide-react";
import type { ReactNode, RefObject } from "react";

import { UiButton } from "@/shared/ui/button/button";
import {
  UiDialogBackdrop,
  UiDialogFooter,
  UiDialogPortal,
  UiDialogShell,
  type UiDialogSize,
} from "@/shared/ui/dialog/dialog";
import { getUiSpinnerClassName } from "@/shared/ui/display/spinner-styles";

interface DecisionDialogFrameProps {
  children: ReactNode;
  describedBy?: string;
  initialFocusRef: RefObject<HTMLElement | null>;
  labelledBy: string;
  onClose: () => void;
  size?: Extract<UiDialogSize, "xs" | "sm">;
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
  size = "sm",
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
        <UiDialogShell size={size}>{children}</UiDialogShell>
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
    <UiDialogFooter appearance="plain" className="gap-2">
      <UiButton
        disabled={busy}
        onClick={onCancel}
        size="md"
        variant="surface"
      >
        {cancelText}
      </UiButton>
      <UiButton
        aria-busy={busy}
        className={confirmClassName}
        disabled={busy}
        onClick={onConfirm}
        ref={confirmButtonRef}
        size="md"
        tone={confirmTone}
        variant="solid"
      >
        {busy ? (
          <LoaderCircle
            aria-hidden
            className={getUiSpinnerClassName({ size: "md" })}
          />
        ) : null}
        {confirmText}
      </UiButton>
    </UiDialogFooter>
  );
}
