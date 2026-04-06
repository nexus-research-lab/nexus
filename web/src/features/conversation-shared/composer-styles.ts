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

export const COMPOSER_ATTACHMENT_REMOVE_CLASS_NAME =
  "ml-1 rounded-full p-0.5 text-red-500/76 opacity-60 transition-[background,opacity] duration-150 hover:bg-red-50/90 focus-visible:ring-2 focus-visible:ring-primary/50";

export function getComposerShellClassName(disabled: boolean) {
  return cn(
    "relative overflow-hidden rounded-[16px] transition-[box-shadow,border-color,background] duration-150",
    disabled && "cursor-not-allowed opacity-50",
  );
}

export function getComposerShellStyle(compact: boolean) {
  return {
    background: "var(--surface-shell-background)",
    border: `1px solid ${"color-mix(in srgb, var(--surface-shell-border) 94%, transparent)"}`,
    boxShadow: compact ? "none" : "0 8px 18px rgb(71 85 105 / 0.05)",
  } as const;
}

export const COMPOSER_FOOTER_CLASS_NAME =
  "flex items-center justify-between border-t border-[var(--divider-subtle-color)] px-2.5 py-1 text-slate-400/80";
