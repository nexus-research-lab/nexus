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

/** 统一弹窗 shell（圆角 + 材质） */
export const DIALOG_SHELL_CLASS_NAME =
  "dialog-shell radius-shell-xl w-full max-w-lg animate-in fade-in zoom-in-95 duration-200";

/** 统一弹窗遮罩 */
export const DIALOG_BACKDROP_CLASS_NAME = "dialog-backdrop animate-in fade-in duration-200";

/** 统一 popover 面板 */
export const DIALOG_POPOVER_CLASS_NAME =
  "surface-popover radius-shell-lg overflow-hidden";

export const DIALOG_HEADER_ICON_CLASS_NAME =
  "dialog-card flex h-10 w-10 shrink-0 items-center justify-center rounded-2xl text-(--text-strong)";

export const DIALOG_ICON_BUTTON_CLASS_NAME =
  "inline-flex h-8 w-8 items-center justify-center rounded-[10px] text-(--icon-default) transition duration-(--motion-duration-fast) hover:bg-(--surface-interactive-hover-background) hover:text-(--icon-strong)";

export const DIALOG_EMPTY_CLASS_NAME =
  "surface-inset flex items-center justify-center rounded-[20px] px-4 py-4 text-[13px] text-(--text-muted)";

export const DIALOG_TAG_CLASS_NAME =
  "chip-default inline-flex items-center gap-1 rounded-full px-[0.7rem] py-[0.3rem] text-[11px] font-semibold text-(--text-muted)";

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

  return cn(
    "inline-flex items-center justify-center gap-1.5 font-semibold transition duration-(--motion-duration-fast) disabled:cursor-not-allowed disabled:opacity-(--disabled-opacity)",
    size === "default" && "rounded-[12px] px-4 py-2.5 text-sm",
    size === "compact" && "rounded-[10px] px-3 py-1.5 text-[12px]",
    tone === "default" && "text-(--text-default) hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)",
    tone === "primary" && "border border-[color:color-mix(in_srgb,var(--primary)_26%,var(--divider-subtle-color))] bg-[color:color-mix(in_srgb,var(--primary)_12%,transparent)] text-(--primary) hover:bg-[color:color-mix(in_srgb,var(--primary)_18%,transparent)]",
    tone === "danger" && "border border-[color:color-mix(in_srgb,var(--destructive)_24%,var(--divider-subtle-color))] bg-[color:color-mix(in_srgb,var(--destructive)_10%,transparent)] text-(--destructive) hover:bg-[color:color-mix(in_srgb,var(--destructive)_14%,transparent)]",
    resolved_class_name,
  );
}

export function get_dialog_note_class_name(tone: "default" | "danger", class_name?: string): string {
  return cn(
    "rounded-[18px] px-4 py-[0.95rem] text-[13px] leading-[1.65]",
    tone === "default"
      ? "surface-card text-(--text-default)"
      : "border text-(--text-default)",
    class_name,
  );
}

export function get_dialog_note_style(tone: "default" | "danger"): CSSProperties | undefined {
  if (tone !== "danger") {
    return undefined;
  }

  return {
    background: "color-mix(in srgb, var(--destructive) 12%, var(--modal-card-background))",
    borderColor: "color-mix(in srgb, var(--destructive) 26%, var(--modal-card-border))",
    color: "var(--text-default)",
  };
}

export function get_dialog_choice_class_name(is_active: boolean, class_name?: string): string {
  return cn(
    "inline-flex items-center justify-center gap-1.5 rounded-[12px] border px-3 py-2 text-[12px] font-semibold transition-[background,color,border-color] duration-(--motion-duration-normal) ease-out",
    is_active
      ? "text-(--primary)"
      : "border-(--divider-subtle-color) text-(--text-muted) hover:border-(--surface-interactive-hover-border) hover:text-(--text-strong)",
    class_name,
  );
}

export function get_dialog_choice_style(is_active: boolean): CSSProperties | undefined {
  if (!is_active) {
    return undefined;
  }

  return {
    background: "color-mix(in srgb, var(--primary) 10%, transparent)",
    border: "1px solid color-mix(in srgb, var(--primary) 28%, var(--divider-subtle-color))",
    color: "var(--primary)",
  };
}
