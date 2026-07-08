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

interface CalendarDay {
  label: string;
  muted: boolean;
  value: string;
}

export interface TaskSchedulePanelProps {
  closeDailyPicker: () => void;
  closeSinglePicker: () => void;
  dailyAnchorRef: RefObject<HTMLButtonElement | null>;
  dailyDisplay: string;
  dailyHour12: string;
  dailyMeridiem: Meridiem;
  dailyMinute: string;
  enabled: boolean;
  errorMessage: string | null;
  everyUnit: EveryUnit;
  everyUnitOptions: Array<{ key: EveryUnit; label: string }>;
  everyValue: string;
  instruction: string;
  instructionLabel: string;
  instructionPlaceholder: string;
  isDailyPickerOpen: boolean;
  isSinglePickerOpen: boolean;
  isSingleDateDisabled: (value: string) => boolean;
  isSingleHourDisabled: (value: string) => boolean;
  isSingleMeridiemDisabled: (value: Meridiem) => boolean;
  isSingleMinuteDisabled: (value: string) => boolean;
  isSingleSecondDisabled: (value: string) => boolean;
  onDailyHourSelect: (value: string) => void;
  onDailyMeridiemSelect: (value: Meridiem) => void;
  onDailyMinuteSelect: (value: string) => void;
  onDailyTriggerClick: () => void;
  onNextMonth: () => void;
  onPrevMonth: () => void;
  onSingleDateSelect: (value: string) => void;
  onSingleHourSelect: (value: string) => void;
  onSingleMeridiemSelect: (value: Meridiem) => void;
  onSingleMinuteSelect: (value: string) => void;
  onSingleSecondSelect: (value: string) => void;
  onSingleTriggerClick: () => void;
  onToggleWeekday: (value: Weekday) => void;
  runAtDisplay: string;
  scheduleKind: ScheduleKind;
  scheduleOptions: Array<{ key: ScheduleKind; label: string }>;
  selectedRunDate: string;
  selectedWeekdays: Weekday[];
  setEnabled: (value: boolean) => void;
  setEveryUnit: (value: EveryUnit) => void;
  setEveryValue: (value: string) => void;
  setInstruction: (value: string) => void;
  setScheduleKind: (value: ScheduleKind) => void;
  setTimezone: (value: string) => void;
  singleAnchorRef: RefObject<HTMLButtonElement | null>;
  singleHour12: string;
  singleMeridiem: Meridiem;
  singleMinute: string;
  singlePickerDays: CalendarDay[];
  singlePickerMonth: string;
  singleSecond: string;
  timezone: string;
  timezoneOptions: string[];
}
