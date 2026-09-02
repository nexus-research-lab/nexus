// INPUT: Dialog 的动作语义、视口模式和调用方外部布局约束。
// OUTPUT: 由共享 Button 与 Dialog recipe 组成的稳定 className。
// POS: Dialog 视觉几何入口；不处理焦点、modal 栈或业务提交。

import { CSSProperties } from "react";

import { cn } from "@/shared/ui/class-name";
import {
  getUiButtonClassName,
  getUiIconButtonClassName,
} from "@/shared/ui/button/button-styles";

export const DIALOG_HEADER_LEADING_CLASS_NAME = "flex min-w-0 items-center gap-2.5";

/** 统一弹窗遮罩 */
export const DIALOG_BACKDROP_CLASS_NAME =
  "dialog-backdrop animate-in fade-in duration-(--motion-duration-fast)";

export const DIALOG_HEADER_ICON_CLASS_NAME =
  "flex h-8 w-8 shrink-0 items-center justify-center radius-control-sm bg-(--surface-interactive-hover-background) text-(--icon-default)";

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
    "radius-control-lg px-4 py-[0.95rem] text-sm leading-[1.65]",
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
