/**
 * =====================================================
 * @File   : task-schedule-panel-model.ts
 * @Date   : 2026-04-16 13:44
 * @Author : leemysw
 * 2026-04-16 13:44   Create
 * =====================================================
 */

"use client";

import type { RefObject } from "react";

import type { Meridiem, Weekday } from "../pickers/picker-types";
import type { EveryUnit, ScheduleKind } from "./scheduled-task-dialog-types";

export interface CalendarDay {
  label: string;
  muted: boolean;
  value: string;
}

export interface TaskSchedulePanelProps {
  close_daily_picker: () => void;
  close_single_picker: () => void;
  daily_anchor_ref: RefObject<HTMLButtonElement | null>;
  daily_display: string;
  daily_hour12: string;
  daily_meridiem: Meridiem;
  daily_minute: string;
  enabled: boolean;
  error_message: string | null;
  every_unit: EveryUnit;
  every_unit_options: Array<{ key: EveryUnit; label: string }>;
  every_value: string;
  instruction: string;
  is_daily_picker_open: boolean;
  is_single_picker_open: boolean;
  is_single_date_disabled: (value: string) => boolean;
  is_single_hour_disabled: (value: string) => boolean;
  is_single_meridiem_disabled: (value: Meridiem) => boolean;
  is_single_minute_disabled: (value: string) => boolean;
  is_single_second_disabled: (value: string) => boolean;
  on_daily_hour_select: (value: string) => void;
  on_daily_meridiem_select: (value: Meridiem) => void;
  on_daily_minute_select: (value: string) => void;
  on_daily_trigger_click: () => void;
  on_next_month: () => void;
  on_prev_month: () => void;
  on_single_date_select: (value: string) => void;
  on_single_hour_select: (value: string) => void;
  on_single_meridiem_select: (value: Meridiem) => void;
  on_single_minute_select: (value: string) => void;
  on_single_second_select: (value: string) => void;
  on_single_trigger_click: () => void;
  on_toggle_weekday: (value: Weekday) => void;
  run_at_display: string;
  schedule_kind: ScheduleKind;
  schedule_options: Array<{ key: ScheduleKind; label: string }>;
  selected_run_date: string;
  selected_weekdays: Weekday[];
  set_enabled: (value: boolean) => void;
  set_every_unit: (value: EveryUnit) => void;
  set_every_value: (value: string) => void;
  set_instruction: (value: string) => void;
  set_schedule_kind: (value: ScheduleKind) => void;
  set_timezone: (value: string) => void;
  single_anchor_ref: RefObject<HTMLButtonElement | null>;
  single_hour12: string;
  single_meridiem: Meridiem;
  single_minute: string;
  single_picker_days: CalendarDay[];
  single_picker_month: string;
  single_second: string;
  timezone: string;
  timezone_options: string[];
}

export const COMPACT_STEPPER_CLASS_NAME =
  "dialog-input radius-shell-sm w-full px-5 py-3 text-[15px] text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/20";

export const COMPACT_SELECT_CLASS_NAME =
  "dialog-input radius-shell-sm w-full appearance-none px-5 py-3 text-[15px] text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/20";

export function get_schedule_tab_class_name(is_active: boolean): string {
  return [
    "inline-flex min-w-[64px] items-center justify-center rounded-[10px] border px-3 py-1.5 text-sm font-semibold transition-[background,color,border-color] duration-(--motion-duration-fast)",
    is_active
      ? "border-[color:color-mix(in_srgb,var(--primary)_30%,var(--divider-subtle-color))] bg-[color:color-mix(in_srgb,var(--primary)_12%,transparent)] text-(--primary)"
      : "border-transparent text-(--text-muted) hover:text-(--text-strong)",
  ].join(" ");
}

export function get_weekday_pill_class_name(is_active: boolean): string {
  return [
    "inline-flex h-9 min-w-9 items-center justify-center rounded-full border px-3 text-sm font-semibold transition-[background,color,border-color] duration-(--motion-duration-fast)",
    is_active
      ? "border-[color:color-mix(in_srgb,var(--primary)_34%,var(--divider-subtle-color))] bg-[color:color-mix(in_srgb,var(--primary)_12%,transparent)] text-(--primary)"
      : "border-(--divider-subtle-color) text-(--text-muted) hover:border-(--text-default) hover:text-(--text-strong)",
  ].join(" ");
}
