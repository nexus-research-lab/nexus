import { CSSProperties } from "react";

import { cn } from "@/lib/utils";
import {
  get_ui_button_class_name,
  get_ui_icon_button_class_name,
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

export const DIALOG_ICON_BUTTON_CLASS_NAME = get_ui_icon_button_class_name({
  size: "md",
  variant: "ghost",
});

export const DIALOG_EMPTY_CLASS_NAME =
  "flex items-center justify-center rounded-[12px] px-4 py-4 text-[13px] text-(--text-muted)";

export const DIALOG_TAG_CLASS_NAME =
  "inline-flex items-center gap-1 rounded-[6px] border border-(--divider-subtle-color) bg-transparent px-2 py-0.5 text-[11px] font-medium text-(--text-muted)";

export function get_dialog_action_class_name(
  tone: "default" | "primary" | "danger",
  size_or_class_name?: "default" | "compact" | string,
  class_name?: string,
): string {
  const size = size_or_class_name === "compact" || size_or_class_name === "default"
    ? size_or_class_name
    : "default";
  const resolved_class_name =
    typeof size_or_class_name === "string" &&
      size_or_class_name !== "compact" &&
      size_or_class_name !== "default"
      ? size_or_class_name
      : class_name;

  return get_ui_button_class_name(
    {
      size: size === "compact" ? "sm" : "md",
      tone,
      variant: "surface",
    },
    resolved_class_name,
  );
}

export function get_dialog_note_class_name(tone: "default" | "danger", class_name?: string): string {
  return cn(
    "rounded-[14px] px-4 py-[0.95rem] text-[13px] leading-[1.65]",
    tone === "default"
      ? "border border-[color:color-mix(in_srgb,var(--divider-subtle-color)_76%,transparent)] bg-transparent text-(--text-default)"
      : "border text-(--text-default)",
    class_name,
  );
}

export function get_dialog_note_style(tone: "default" | "danger"): CSSProperties | undefined {
  if (tone !== "danger") {
    return undefined;
  }

  return {
    background: "color-mix(in srgb, var(--destructive) 12%, var(--modal-dialog-body-background))",
    borderColor: "color-mix(in srgb, var(--destructive) 26%, var(--modal-card-border))",
    color: "var(--text-default)",
  };
}
