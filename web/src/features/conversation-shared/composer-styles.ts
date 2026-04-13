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
  "flex flex-wrap gap-2 border-b border-[var(--divider-subtle-color)] px-2.5 py-2";

export const COMPOSER_ATTACHMENT_REMOVE_CLASS_NAME =
  "ml-1 rounded-full p-0.5 text-red-500/76 opacity-60 transition-[background,opacity] duration-[var(--motion-duration-fast)] hover:bg-red-50/90 focus-visible:ring-2 focus-visible:ring-primary/50";

export const COMPOSER_ACTION_BUTTON_CLASS_NAME =
  "inline-flex h-9 w-9 shrink-0 items-center justify-center rounded-[12px] border border-[var(--divider-subtle-color)] bg-transparent text-[color:var(--icon-default)] transition-[background,border-color,color] duration-[var(--motion-duration-fast)] hover:border-[var(--surface-interactive-hover-border)] hover:bg-[var(--surface-interactive-hover-background)] hover:text-[color:var(--text-strong)] disabled:cursor-not-allowed disabled:opacity-[var(--disabled-opacity)]";

export const COMPOSER_PRIMARY_ACTION_BUTTON_CLASS_NAME =
  "inline-flex h-9 w-9 shrink-0 items-center justify-center rounded-[14px] border border-[var(--button-primary-border)] bg-[var(--button-primary-background)] text-[var(--button-primary-color)] transition-[background,border-color,color] duration-[var(--motion-duration-fast)] hover:bg-[var(--button-primary-hover-background)] hover:border-[var(--button-primary-hover-border)] disabled:cursor-not-allowed disabled:opacity-[var(--disabled-opacity)]";

export const COMPOSER_DANGER_ACTION_BUTTON_CLASS_NAME =
  "inline-flex h-9 w-9 shrink-0 items-center justify-center rounded-[14px] border border-[color-mix(in_srgb,var(--destructive)_28%,transparent)] bg-[color-mix(in_srgb,var(--destructive)_10%,transparent)] text-[color-mix(in_srgb,var(--destructive)_86%,var(--foreground)_14%)] transition-[background,border-color,color] duration-[var(--motion-duration-fast)] hover:bg-[color-mix(in_srgb,var(--destructive)_14%,transparent)] hover:border-[color-mix(in_srgb,var(--destructive)_36%,transparent)] disabled:cursor-not-allowed disabled:opacity-[var(--disabled-opacity)]";

export function getComposerShellClassName(disabled: boolean) {
  return cn(
    "rounded-[18px] transition-[border-color,background,box-shadow] duration-[var(--motion-duration-fast)] focus-within:shadow-[0_0_0_1px_var(--material-input-focus-border)]",
    disabled && "cursor-not-allowed opacity-[var(--disabled-opacity)]",
  );
}

export function getComposerShellStyle(compact: boolean) {
  return {
    background: "var(--material-input-background)",
    border: `1px solid ${"var(--material-input-border)"}`,
    boxShadow: compact
      ? "0 6px 18px rgba(24, 34, 48, 0.05), inset 0 1px 0 rgba(255, 255, 255, 0.16)"
      : "0 10px 28px rgba(24, 34, 48, 0.06), inset 0 1px 0 rgba(255, 255, 255, 0.16)",
  } as const;
}

export const COMPOSER_FOOTER_CLASS_NAME =
  "flex items-center justify-between gap-3 border-t border-[var(--divider-subtle-color)] px-2.5 py-1.5 text-[color:var(--text-soft)]";
