/**
 * =====================================================
 * @File   : composer-styles.ts
 * @Date   : 2026-04-05 16:35
 * @Author : leemysw
 * 2026-04-05 16:35   Create
 * =====================================================
 */

import { cn } from "@/lib/utils";
import { get_ui_icon_button_class_name } from "@/shared/ui/button-styles";

export const COMPOSER_ATTACHMENT_CLASS_NAME =
  "chip-default group relative inline-flex items-center gap-2 rounded-[14px] px-3 py-[0.45rem]";

export const COMPOSER_ATTACHMENT_ROW_CLASS_NAME =
  "flex flex-wrap gap-2 border-b border-(--divider-subtle-color) px-2.5 py-2";

export const COMPOSER_ATTACHMENT_REMOVE_CLASS_NAME =
  "ml-1 rounded-full p-0.5 text-red-500/76 opacity-60 transition-[background,opacity] duration-(--motion-duration-fast) hover:bg-red-50/90 focus-visible:ring-2 focus-visible:ring-primary/50";

export const COMPOSER_ACTION_BUTTON_CLASS_NAME =
  get_ui_icon_button_class_name({ size: "lg", variant: "surface" }, "shrink-0");

export const COMPOSER_PRIMARY_ACTION_BUTTON_CLASS_NAME =
  get_ui_icon_button_class_name({ size: "lg", tone: "primary", variant: "solid" }, "shrink-0");

export const COMPOSER_DANGER_ACTION_BUTTON_CLASS_NAME =
  get_ui_icon_button_class_name({ size: "lg", tone: "danger", variant: "surface" }, "shrink-0");

export function get_composer_shell_class_name(disabled: boolean) {
  return cn(
    "input-shell overflow-hidden rounded-[18px]",
    disabled && "cursor-not-allowed opacity-(--disabled-opacity)",
  );
}

export function get_composer_shell_style(compact: boolean) {
  void compact;
  return undefined;
}

export const COMPOSER_FOOTER_CLASS_NAME =
  "flex items-center justify-between gap-3 border-t border-(--divider-subtle-color) px-2.5 py-1.5 text-(--text-soft)";
