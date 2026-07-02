"use client";

import { useCallback, useState } from "react";

import {
  build_calendar_days,
  build_datetime_local_input,
  build_time_value,
  format_datetime_display,
  format_datetime_local_input,
  format_time_display,
  format_time_local_input,
  from_meridiem_parts,
  split_datetime_local_input,
  split_time_value,
  to_meridiem_parts,
} from "../pickers/picker-formatters";
import { type Meridiem, type Weekday } from "../pickers/picker-types";
import { zonedDateTimeToEpochMs } from "./scheduled-task-dialog-time";
import type { EveryUnit, ScheduleKind } from "./scheduled-task-dialog-types";

export function useScheduledTaskDialogScheduleState(timezone: string) {
  const now = new Date();
  const nowDate = `${now.getFullYear()}-${`${now.getMonth() + 1}`.padStart(2, "0")}-${`${now.getDate()}`.padStart(2, "0")}`;
  const [scheduleKind, setScheduleKind] = useState<ScheduleKind>("every");
  const [everyValue, setEveryValue] = useState("30");
  const [everyUnit, setEveryUnit] = useState<EveryUnit>("minutes");
  const [dailyTime, setDailyTime] = useState(format_time_local_input(new Date(Date.now() + 3600_000)));
  const [selectedWeekdays, setSelectedWeekdays] = useState<Weekday[]>(["mo", "tu", "we", "th", "fr", "sa", "su"]);
  const [runAt, setRunAt] = useState(format_datetime_local_input(new Date(Date.now() + 3600_000)));
  const [isDailyPickerOpen, setIsDailyPickerOpen] = useState(false);
  const [isSinglePickerOpen, setIsSinglePickerOpen] = useState(false);
  const [singlePickerMonth, setSinglePickerMonth] = useState(format_datetime_local_input(new Date(Date.now())).slice(0, 7));

  const dailyTimeParts = split_time_value(dailyTime);
  const runAtParts = split_datetime_local_input(runAt);
  const dailyMeridiemParts = to_meridiem_parts(dailyTimeParts.hour, dailyTimeParts.minute);
  const singleMeridiemParts = to_meridiem_parts(runAtParts.hour, runAtParts.minute, runAtParts.second);
  const singlePickerDays = build_calendar_days(singlePickerMonth);

  const reset = useCallback(() => {
    setScheduleKind("every");
    setEveryValue("30");
    setEveryUnit("minutes");
    setDailyTime(format_time_local_input(new Date(Date.now() + 3600_000)));
    setSelectedWeekdays(["mo", "tu", "we", "th", "fr", "sa", "su"]);
    setRunAt(format_datetime_local_input(new Date(Date.now() + 3600_000)));
    setIsDailyPickerOpen(false);
    setIsSinglePickerOpen(false);
    setSinglePickerMonth(format_datetime_local_input(new Date(Date.now() + 3600_000)).slice(0, 7));
  }, []);

  const hydrate = useCallback((params: {
    schedule_kind: ScheduleKind;
    every_value?: string;
    every_unit?: EveryUnit;
    daily_time?: string;
    selected_weekdays?: Weekday[];
    run_at?: string;
  }) => {
    setScheduleKind(params.schedule_kind);
    setEveryValue(params.every_value ?? "30");
    setEveryUnit(params.every_unit ?? "minutes");
    setDailyTime(params.daily_time ?? format_time_local_input(new Date(Date.now() + 3600_000)));
    setSelectedWeekdays(params.selected_weekdays ?? ["mo", "tu", "we", "th", "fr", "sa", "su"]);
    const nextRunAt = params.run_at ?? format_datetime_local_input(new Date(Date.now() + 3600_000));
    setRunAt(nextRunAt);
    setIsDailyPickerOpen(false);
    setIsSinglePickerOpen(false);
    setSinglePickerMonth(nextRunAt.slice(0, 7));
  }, []);

  function updateDailyPicker(next: { meridiem?: Meridiem; hour12?: string; minute?: string }) {
    const merged = {
      meridiem: next.meridiem ?? dailyMeridiemParts.meridiem,
      hour12: next.hour12 ?? dailyMeridiemParts.hour12,
      minute: next.minute ?? dailyMeridiemParts.minute,
    };
    const converted = from_meridiem_parts(merged.meridiem, merged.hour12, merged.minute);
    setDailyTime(build_time_value(converted.hour24, converted.minute));
  }

  function updateSinglePicker(next: { date?: string; meridiem?: Meridiem; hour12?: string; minute?: string; second?: string }) {
    const merged = {
      date: next.date ?? runAtParts.date,
      meridiem: next.meridiem ?? singleMeridiemParts.meridiem,
      hour12: next.hour12 ?? singleMeridiemParts.hour12,
      minute: next.minute ?? singleMeridiemParts.minute,
      second: next.second ?? singleMeridiemParts.second,
    };
    const converted = from_meridiem_parts(merged.meridiem, merged.hour12, merged.minute, merged.second);
    setRunAt(build_datetime_local_input(merged.date, converted.hour24, converted.minute, converted.second));
  }

  function toggleWeekday(weekday: Weekday) {
    setSelectedWeekdays((current) =>
      current.includes(weekday) ? current.filter((item) => item !== weekday) : [...current, weekday],
    );
  }

  function goToPrevMonth() {
    const [year, month] = singlePickerMonth.split("-").map(Number);
    const prev = new Date(year, month - 2, 1);
    setSinglePickerMonth(`${prev.getFullYear()}-${`${prev.getMonth() + 1}`.padStart(2, "0")}`);
  }

  function goToNextMonth() {
    const [year, month] = singlePickerMonth.split("-").map(Number);
    const next = new Date(year, month, 1);
    setSinglePickerMonth(`${next.getFullYear()}-${`${next.getMonth() + 1}`.padStart(2, "0")}`);
  }

  function syncSinglePickerToNow() {
    const nowValue = new Date();
    setRunAt(format_datetime_local_input(nowValue));
    setSinglePickerMonth(format_datetime_local_input(nowValue).slice(0, 7));
  }

  function buildSingleCandidateInput(params: {
    date?: string;
    meridiem?: Meridiem;
    hour12?: string;
    minute?: string;
    second?: string;
  }): string {
    const merged = {
      date: params.date ?? runAtParts.date,
      meridiem: params.meridiem ?? singleMeridiemParts.meridiem,
      hour12: params.hour12 ?? singleMeridiemParts.hour12,
      minute: params.minute ?? singleMeridiemParts.minute,
      second: params.second ?? singleMeridiemParts.second,
    };
    const converted = from_meridiem_parts(merged.meridiem, merged.hour12, merged.minute, merged.second);
    return build_datetime_local_input(merged.date, converted.hour24, converted.minute, converted.second);
  }

  function isSingleDateDisabled(dateValue: string): boolean {
    const epochMs = zonedDateTimeToEpochMs(buildSingleCandidateInput({ date: dateValue }), timezone);
    return epochMs !== null && epochMs <= Date.now();
  }

  function isSingleMeridiemDisabled(value: Meridiem): boolean {
    const epochMs = zonedDateTimeToEpochMs(buildSingleCandidateInput({ meridiem: value }), timezone);
    return epochMs !== null && epochMs <= Date.now();
  }

  function isSingleHourDisabled(value: string): boolean {
    const epochMs = zonedDateTimeToEpochMs(buildSingleCandidateInput({ hour12: value }), timezone);
    return epochMs !== null && epochMs <= Date.now();
  }

  function isSingleMinuteDisabled(value: string): boolean {
    const epochMs = zonedDateTimeToEpochMs(buildSingleCandidateInput({ minute: value }), timezone);
    return epochMs !== null && epochMs <= Date.now();
  }

  function isSingleSecondDisabled(value: string): boolean {
    const epochMs = zonedDateTimeToEpochMs(buildSingleCandidateInput({ second: value }), timezone);
    return epochMs !== null && epochMs <= Date.now();
  }

  return {
    schedule_kind: scheduleKind,
    set_schedule_kind: setScheduleKind,
    every_value: everyValue,
    set_every_value: setEveryValue,
    every_unit: everyUnit,
    set_every_unit: setEveryUnit,
    daily_time: dailyTime,
    selected_weekdays: selectedWeekdays,
    set_selected_weekdays: setSelectedWeekdays,
    run_at: runAt,
    set_run_at: setRunAt,
    is_daily_picker_open: isDailyPickerOpen,
    set_is_daily_picker_open: setIsDailyPickerOpen,
    is_single_picker_open: isSinglePickerOpen,
    set_is_single_picker_open: setIsSinglePickerOpen,
    single_picker_month: singlePickerMonth,
    set_single_picker_month: setSinglePickerMonth,
    daily_time_parts: dailyTimeParts,
    run_at_parts: runAtParts,
    daily_meridiem_parts: dailyMeridiemParts,
    single_meridiem_parts: singleMeridiemParts,
    single_picker_days: singlePickerDays,
    daily_display: format_time_display(dailyTimeParts.hour, dailyTimeParts.minute),
    run_at_display: format_datetime_display(runAtParts.date, runAtParts.hour, runAtParts.minute, runAtParts.second),
    update_daily_picker: updateDailyPicker,
    update_single_picker: updateSinglePicker,
    toggle_weekday: toggleWeekday,
    go_to_prev_month: goToPrevMonth,
    go_to_next_month: goToNextMonth,
    sync_single_picker_to_now: syncSinglePickerToNow,
    now_date: nowDate,
    is_single_date_disabled: isSingleDateDisabled,
    is_single_meridiem_disabled: isSingleMeridiemDisabled,
    is_single_hour_disabled: isSingleHourDisabled,
    is_single_minute_disabled: isSingleMinuteDisabled,
    is_single_second_disabled: isSingleSecondDisabled,
    reset,
    hydrate,
  };
}
