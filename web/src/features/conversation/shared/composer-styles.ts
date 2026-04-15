/**
 * =====================================================
 * @File   : composer-styles.ts
 * @Date   : 2026-04-05 16:35
 * @Author : leemysw
 * 2026-04-05 16:35   Create
 * =====================================================
 */

import { cn } from "@/lib/utils";

export const COMPOSER_ATTACHMENT_CLASS_NAME =
  "chip-default group relative inline-flex items-center gap-2 rounded-[14px] px-3 py-[0.45rem]";

export const COMPOSER_ATTACHMENT_ROW_CLASS_NAME =
  "flex flex-wrap gap-2 border-b border-(--divider-subtle-color) px-2.5 py-2";

export const COMPOSER_ATTACHMENT_REMOVE_CLASS_NAME =
  "ml-1 rounded-full p-0.5 text-red-500/76 opacity-60 transition-[background,opacity] duration-(--motion-duration-fast) hover:bg-red-50/90 focus-visible:ring-2 focus-visible:ring-primary/50";

export const COMPOSER_ACTION_BUTTON_CLASS_NAME =
  "inline-flex h-9 w-9 shrink-0 items-center justify-center rounded-[12px] border border-(--divider-subtle-color) bg-transparent text-(--icon-default) transition-[background,border-color,color] duration-(--motion-duration-fast) hover:border-(--surface-interactive-hover-border) hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong) disabled:cursor-not-allowed disabled:opacity-(--disabled-opacity)";

export const COMPOSER_PRIMARY_ACTION_BUTTON_CLASS_NAME =
  "inline-flex h-9 w-9 shrink-0 items-center justify-center rounded-[14px] border border-(--button-primary-border) bg-(--button-primary-background) text-(--button-primary-color) transition-[background,border-color,color] duration-(--motion-duration-fast) hover:bg-(--button-primary-hover-background) hover:border-(--button-primary-hover-border) disabled:cursor-not-allowed disabled:opacity-(--disabled-opacity)";

export const COMPOSER_DANGER_ACTION_BUTTON_CLASS_NAME =
  "inline-flex h-9 w-9 shrink-0 items-center justify-center rounded-[14px] border border-[color-mix(in_srgb,var(--destructive)_28%,transparent)] bg-[color-mix(in_srgb,var(--destructive)_10%,transparent)] text-[color-mix(in_srgb,var(--destructive)_86%,var(--foreground)_14%)] transition-[background,border-color,color] duration-(--motion-duration-fast) hover:bg-[color-mix(in_srgb,var(--destructive)_14%,transparent)] hover:border-[color-mix(in_srgb,var(--destructive)_36%,transparent)] disabled:cursor-not-allowed disabled:opacity-(--disabled-opacity)";

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
