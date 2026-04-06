/**
# !/usr/bin/env xx
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：dialog-styles.ts
# @Date   ：2026-04-05 17:09
# @Author ：leemysw
# 2026-04-05 17:09   Create
# =====================================================
*/

import { CSSProperties } from "react";

import { cn } from "@/lib/utils";

export const DIALOG_HEADER_LEADING_CLASS_NAME = "flex min-w-0 items-start gap-3.5";

export const DIALOG_HEADER_ICON_CLASS_NAME =
  "dialog-card flex h-10 w-10 shrink-0 items-center justify-center rounded-2xl text-slate-900/88";

export const DIALOG_EMPTY_CLASS_NAME =
  "surface-inset flex items-center justify-center rounded-[20px] px-4 py-4 text-[13px] text-slate-500/78";

export const DIALOG_TAG_CLASS_NAME =
  "chip-default inline-flex items-center gap-1 rounded-full px-[0.7rem] py-[0.3rem] text-[11px] font-semibold text-slate-600/86";

export function getDialogNoteClassName(tone: "default" | "danger", class_name?: string): string {
  return cn(
    "rounded-[18px] px-4 py-[0.95rem] text-[13px] leading-[1.65]",
    tone === "default"
      ? "surface-card text-slate-600/82"
      : "border border-rose-200/86 bg-rose-50/88 text-pink-700/88",
    class_name,
  );
}

export function getDialogChoiceClassName(is_active: boolean, class_name?: string): string {
  return cn(
    "inline-flex items-center justify-center gap-1.5 rounded-full px-[0.9rem] py-[0.45rem] text-[12px] font-semibold transition-[background,color,box-shadow,border-color] duration-180 ease-out",
    is_active
      ? "text-[var(--primary-foreground)]"
      : "chip-pill border border-[color:var(--chip-pill-border)] text-slate-600/84 hover:text-slate-900/94",
    class_name,
  );
}

export function getDialogChoiceStyle(is_active: boolean): CSSProperties | undefined {
  if (!is_active) {
    return undefined;
  }

  return {
    background: "color-mix(in srgb, var(--primary) 82%, white)",
    color: "var(--primary-foreground)",
  };
}
