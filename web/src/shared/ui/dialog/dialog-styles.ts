import { CSSProperties } from "react";

import { cn } from "@/lib/utils";
import {
  getUiButtonClassName,
  getUiIconButtonClassName,
} from "@/shared/ui/button-styles";

export const DIALOG_HEADER_LEADING_CLASS_NAME = "flex min-w-0 items-center gap-2.5";

/** 统一弹窗遮罩 */
export const DIALOG_BACKDROP_CLASS_NAME =
  "dialog-backdrop animate-in fade-in duration-(--motion-duration-fast)";

/** 统一 popover 面板 */
export const DIALOG_POPOVER_CLASS_NAME =
  "surface-popover surface-radius-lg overflow-hidden";

export const DIALOG_HEADER_ICON_CLASS_NAME =
  "flex h-9 w-9 shrink-0 items-center justify-center rounded-[12px] border border-[color:color-mix(in_srgb,var(--primary)_16%,transparent)] bg-[color:color-mix(in_srgb,var(--primary)_6%,transparent)] text-(--text-strong)";

export const DIALOG_ICON_BUTTON_CLASS_NAME = getUiIconButtonClassName({
  size: "md",
  variant: "ghost",
});

export function getDialogActionClassName(
  tone: "default" | "primary" | "danger",
  sizeOrClassName?: "default" | "compact" | string,
  className?: string,
): string {
  const size = sizeOrClassName === "compact" || sizeOrClassName === "default"
    ? sizeOrClassName
    : "default";
  const resolvedClassName =
    typeof sizeOrClassName === "string" &&
      sizeOrClassName !== "compact" &&
      sizeOrClassName !== "default"
      ? sizeOrClassName
      : className;

  return getUiButtonClassName(
    {
      size: size === "compact" ? "sm" : "md",
      tone,
      variant: "surface",
    },
    resolvedClassName,
  );
}

export function getDialogNoteClassName(tone: "default" | "danger", className?: string): string {
  return cn(
    "rounded-[14px] px-4 py-[0.95rem] text-[13px] leading-[1.65]",
    tone === "default"
      ? "border border-[color:color-mix(in_srgb,var(--divider-subtle-color)_76%,transparent)] bg-transparent text-(--text-default)"
      : "border text-(--text-default)",
    className,
  );
}

export function getDialogNoteStyle(tone: "default" | "danger"): CSSProperties | undefined {
  if (tone !== "danger") {
    return undefined;
  }

  return {
    background: "color-mix(in srgb, var(--destructive) 12%, var(--modal-dialog-body-background))",
    borderColor: "color-mix(in srgb, var(--destructive) 26%, var(--modal-card-border))",
    color: "var(--text-default)",
  };
}
