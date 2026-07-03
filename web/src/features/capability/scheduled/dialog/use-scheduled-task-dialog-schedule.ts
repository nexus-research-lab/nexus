"use client";

import { useCallback, useState } from "react";

import {
  buildCalendarDays,
  buildDatetimeLocalInput,
  buildTimeValue,
  formatDatetimeDisplay,
  formatDatetimeLocalInput,
  formatTimeDisplay,
  formatTimeLocalInput,
  fromMeridiemParts,
  splitDatetimeLocalInput,
  splitTimeValue,
  toMeridiemParts,
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
  const [dailyTime, setDailyTime] = useState(formatTimeLocalInput(new Date(Date.now() + 3600_000)));
  const [selectedWeekdays, setSelectedWeekdays] = useState<Weekday[]>(["mo", "tu", "we", "th", "fr", "sa", "su"]);
  const [runAt, setRunAt] = useState(formatDatetimeLocalInput(new Date(Date.now() + 3600_000)));
  const [isDailyPickerOpen, setIsDailyPickerOpen] = useState(false);
  const [isSinglePickerOpen, setIsSinglePickerOpen] = useState(false);
  const [singlePickerMonth, setSinglePickerMonth] = useState(formatDatetimeLocalInput(new Date(Date.now())).slice(0, 7));

  const dailyTimeParts = splitTimeValue(dailyTime);
  const runAtParts = splitDatetimeLocalInput(runAt);
  const dailyMeridiemParts = toMeridiemParts(dailyTimeParts.hour, dailyTimeParts.minute);
  const singleMeridiemParts = toMeridiemParts(runAtParts.hour, runAtParts.minute, runAtParts.second);
  const singlePickerDays = buildCalendarDays(singlePickerMonth);

  const reset = useCallback(() => {
    setScheduleKind("every");
    setEveryValue("30");
    setEveryUnit("minutes");
    setDailyTime(formatTimeLocalInput(new Date(Date.now() + 3600_000)));
    setSelectedWeekdays(["mo", "tu", "we", "th", "fr", "sa", "su"]);
    setRunAt(formatDatetimeLocalInput(new Date(Date.now() + 3600_000)));
    setIsDailyPickerOpen(false);
    setIsSinglePickerOpen(false);
    setSinglePickerMonth(formatDatetimeLocalInput(new Date(Date.now() + 3600_000)).slice(0, 7));
  }, []);

  const hydrate = useCallback((params: {
    scheduleKind: ScheduleKind;
    everyValue?: string;
    everyUnit?: EveryUnit;
    dailyTime?: string;
    selectedWeekdays?: Weekday[];
    runAt?: string;
  }) => {
    setScheduleKind(params.scheduleKind);
    setEveryValue(params.everyValue ?? "30");
    setEveryUnit(params.everyUnit ?? "minutes");
    setDailyTime(params.dailyTime ?? formatTimeLocalInput(new Date(Date.now() + 3600_000)));
    setSelectedWeekdays(params.selectedWeekdays ?? ["mo", "tu", "we", "th", "fr", "sa", "su"]);
    const nextRunAt = params.runAt ?? formatDatetimeLocalInput(new Date(Date.now() + 3600_000));
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
    const converted = fromMeridiemParts(merged.meridiem, merged.hour12, merged.minute);
    setDailyTime(buildTimeValue(converted.hour24, converted.minute));
  }

  function updateSinglePicker(next: { date?: string; meridiem?: Meridiem; hour12?: string; minute?: string; second?: string }) {
    const merged = {
      date: next.date ?? runAtParts.date,
      meridiem: next.meridiem ?? singleMeridiemParts.meridiem,
      hour12: next.hour12 ?? singleMeridiemParts.hour12,
      minute: next.minute ?? singleMeridiemParts.minute,
      second: next.second ?? singleMeridiemParts.second,
    };
    const converted = fromMeridiemParts(merged.meridiem, merged.hour12, merged.minute, merged.second);
    setRunAt(buildDatetimeLocalInput(merged.date, converted.hour24, converted.minute, converted.second));
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
    setRunAt(formatDatetimeLocalInput(nowValue));
    setSinglePickerMonth(formatDatetimeLocalInput(nowValue).slice(0, 7));
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
    const converted = fromMeridiemParts(merged.meridiem, merged.hour12, merged.minute, merged.second);
    return buildDatetimeLocalInput(merged.date, converted.hour24, converted.minute, converted.second);
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
    scheduleKind: scheduleKind,
    setScheduleKind: setScheduleKind,
    everyValue: everyValue,
    setEveryValue: setEveryValue,
    everyUnit: everyUnit,
    setEveryUnit: setEveryUnit,
    dailyTime: dailyTime,
    selectedWeekdays: selectedWeekdays,
    setSelectedWeekdays: setSelectedWeekdays,
    runAt: runAt,
    setRunAt: setRunAt,
    isDailyPickerOpen: isDailyPickerOpen,
    setIsDailyPickerOpen: setIsDailyPickerOpen,
    isSinglePickerOpen: isSinglePickerOpen,
    setIsSinglePickerOpen: setIsSinglePickerOpen,
    singlePickerMonth: singlePickerMonth,
    setSinglePickerMonth: setSinglePickerMonth,
    dailyTimeParts: dailyTimeParts,
    runAtParts: runAtParts,
    dailyMeridiemParts: dailyMeridiemParts,
    singleMeridiemParts: singleMeridiemParts,
    singlePickerDays: singlePickerDays,
    dailyDisplay: formatTimeDisplay(dailyTimeParts.hour, dailyTimeParts.minute),
    runAtDisplay: formatDatetimeDisplay(runAtParts.date, runAtParts.hour, runAtParts.minute, runAtParts.second),
    updateDailyPicker: updateDailyPicker,
    updateSinglePicker: updateSinglePicker,
    toggleWeekday: toggleWeekday,
    goToPrevMonth: goToPrevMonth,
    goToNextMonth: goToNextMonth,
    syncSinglePickerToNow: syncSinglePickerToNow,
    nowDate: nowDate,
    isSingleDateDisabled: isSingleDateDisabled,
    isSingleMeridiemDisabled: isSingleMeridiemDisabled,
    isSingleHourDisabled: isSingleHourDisabled,
    isSingleMinuteDisabled: isSingleMinuteDisabled,
    isSingleSecondDisabled: isSingleSecondDisabled,
    reset,
    hydrate,
  };
}
