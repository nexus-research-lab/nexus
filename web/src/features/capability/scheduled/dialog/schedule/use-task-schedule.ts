import { useCallback, useMemo, useState } from "react";

import {
  buildCalendarDays,
  buildDatetimeLocalInput,
  buildTimeValue,
  formatDatetimeDisplay,
  formatDatetimeLocalInput,
  formatTimeDisplay,
  fromMeridiemParts,
  splitDatetimeLocalInput,
  splitTimeValue,
  toMeridiemParts,
} from "../../pickers/picker-formatters";
import type { Meridiem, Weekday } from "../../pickers/picker-types";
import type {
  EveryUnit,
  ScheduleKind,
  TaskScheduleDraft,
} from "../scheduled-task-dialog-types";
import { createDefaultTaskSchedule } from "./task-schedule-model";
import { zonedDateTimeToEpochMs } from "./task-schedule-time";

interface PickerState {
  dailyOpen: boolean;
  singleMonth: string;
  singleOpen: boolean;
}

function createPickerState(runAt: string): PickerState {
  return {
    dailyOpen: false,
    singleMonth: runAt.slice(0, 7),
    singleOpen: false,
  };
}

export function useTaskSchedule(
  initialDraft: TaskScheduleDraft,
  onChange: () => void,
) {
  const [draft, setDraft] = useState(initialDraft);
  const [picker, setPicker] = useState(() => createPickerState(
    initialDraft.runAt,
  ));

  const hydrate = useCallback((nextDraft: TaskScheduleDraft) => {
    setDraft(nextDraft);
    setPicker(createPickerState(nextDraft.runAt));
  }, []);

  const reset = useCallback(() => {
    const nextDraft = createDefaultTaskSchedule();
    setDraft(nextDraft);
    setPicker(createPickerState(nextDraft.runAt));
  }, []);

  const setValue = useCallback(<Key extends keyof TaskScheduleDraft>(
    key: Key,
    value: TaskScheduleDraft[Key],
  ) => {
    setDraft((current) => ({ ...current, [key]: value }));
    onChange();
  }, [onChange]);

  const timeParts = splitTimeValue(draft.dailyTime);
  const runAtParts = splitDatetimeLocalInput(draft.runAt);
  const dailyMeridiemParts = toMeridiemParts(timeParts.hour, timeParts.minute);
  const singleMeridiemParts = toMeridiemParts(
    runAtParts.hour,
    runAtParts.minute,
    runAtParts.second,
  );

  const updateDailyPicker = useCallback((next: {
    hour12?: string;
    meridiem?: Meridiem;
    minute?: string;
  }) => {
    const currentParts = splitTimeValue(draft.dailyTime);
    const currentMeridiem = toMeridiemParts(
      currentParts.hour,
      currentParts.minute,
    );
    const converted = fromMeridiemParts(
      next.meridiem ?? currentMeridiem.meridiem,
      next.hour12 ?? currentMeridiem.hour12,
      next.minute ?? currentMeridiem.minute,
    );
    setValue("dailyTime", buildTimeValue(converted.hour24, converted.minute));
  }, [draft.dailyTime, setValue]);

  const buildSingleCandidate = useCallback((next: {
    date?: string;
    hour12?: string;
    meridiem?: Meridiem;
    minute?: string;
    second?: string;
  }): string => {
    const parts = splitDatetimeLocalInput(draft.runAt);
    const meridiemParts = toMeridiemParts(
      parts.hour,
      parts.minute,
      parts.second,
    );
    const converted = fromMeridiemParts(
      next.meridiem ?? meridiemParts.meridiem,
      next.hour12 ?? meridiemParts.hour12,
      next.minute ?? meridiemParts.minute,
      next.second ?? meridiemParts.second,
    );
    return buildDatetimeLocalInput(
      next.date ?? parts.date,
      converted.hour24,
      converted.minute,
      converted.second,
    );
  }, [draft.runAt]);

  const updateSinglePicker = useCallback((next: Parameters<
    typeof buildSingleCandidate
  >[0]) => {
    setValue("runAt", buildSingleCandidate(next));
  }, [buildSingleCandidate, setValue]);

  const isSingleCandidateDisabled = useCallback((next: Parameters<
    typeof buildSingleCandidate
  >[0]): boolean => {
    const epochMs = zonedDateTimeToEpochMs(
      buildSingleCandidate(next),
      draft.timezone,
    );
    return epochMs !== null && epochMs <= Date.now();
  }, [buildSingleCandidate, draft.timezone]);

  const toggleWeekday = useCallback((weekday: Weekday) => {
    setDraft((current) => ({
      ...current,
      selectedWeekdays: current.selectedWeekdays.includes(weekday)
        ? current.selectedWeekdays.filter((item) => item !== weekday)
        : [...current.selectedWeekdays, weekday],
    }));
    onChange();
  }, [onChange]);

  const toggleDailyPicker = useCallback(() => {
    setPicker((current) => ({
      ...current,
      dailyOpen: !current.dailyOpen,
      singleOpen: false,
    }));
  }, []);

  const toggleSinglePicker = useCallback(() => {
    const opening = !picker.singleOpen;
    if (opening) {
      const epochMs = zonedDateTimeToEpochMs(draft.runAt, draft.timezone);
      if (epochMs === null || epochMs <= Date.now()) {
        const nextRunAt = formatDatetimeLocalInput(
          new Date(Date.now() + 3600_000),
        );
        setDraft((current) => ({ ...current, runAt: nextRunAt }));
        setPicker({
          dailyOpen: false,
          singleMonth: nextRunAt.slice(0, 7),
          singleOpen: true,
        });
        return;
      }
    }
    setPicker((current) => ({
      ...current,
      dailyOpen: false,
      singleOpen: opening,
    }));
  }, [draft.runAt, draft.timezone, picker.singleOpen]);

  const shiftMonth = useCallback((offset: number) => {
    setPicker((current) => {
      const [year, month] = current.singleMonth.split("-").map(Number);
      const next = new Date(year, month - 1 + offset, 1);
      return {
        ...current,
        singleMonth: `${next.getFullYear()}-${String(
          next.getMonth() + 1,
        ).padStart(2, "0")}`,
      };
    });
  }, []);

  const view = {
    dailyDisplay: formatTimeDisplay(timeParts.hour, timeParts.minute),
    dailyMeridiemParts,
    isDailyPickerOpen: picker.dailyOpen,
    isSinglePickerOpen: picker.singleOpen,
    runAtDisplay: formatDatetimeDisplay(
      runAtParts.date,
      runAtParts.hour,
      runAtParts.minute,
      runAtParts.second,
    ),
    runAtParts,
    singleMeridiemParts,
    singlePickerDays: buildCalendarDays(picker.singleMonth),
    singlePickerMonth: picker.singleMonth,
  };

  const actions = useMemo(() => ({
    closeDailyPicker: () => setPicker((current) => ({
      ...current,
      dailyOpen: false,
    })),
    closeSinglePicker: () => setPicker((current) => ({
      ...current,
      singleOpen: false,
    })),
    goToNextMonth: () => shiftMonth(1),
    goToPrevMonth: () => shiftMonth(-1),
    isSingleDateDisabled: (value: string) => isSingleCandidateDisabled({
      date: value,
    }),
    isSingleHourDisabled: (value: string) => isSingleCandidateDisabled({
      hour12: value,
    }),
    isSingleMeridiemDisabled: (value: Meridiem) => isSingleCandidateDisabled({
      meridiem: value,
    }),
    isSingleMinuteDisabled: (value: string) => isSingleCandidateDisabled({
      minute: value,
    }),
    isSingleSecondDisabled: (value: string) => isSingleCandidateDisabled({
      second: value,
    }),
    setEveryUnit: (value: EveryUnit) => setValue("everyUnit", value),
    setEveryValue: (value: string) => setValue("everyValue", value),
    setKind: (value: ScheduleKind) => setValue("kind", value),
    setTimezone: (value: string) => setValue("timezone", value),
    toggleDailyPicker,
    toggleSinglePicker,
    toggleWeekday,
    updateDailyPicker,
    updateSinglePicker,
  }), [
    isSingleCandidateDisabled,
    setValue,
    shiftMonth,
    toggleDailyPicker,
    toggleSinglePicker,
    toggleWeekday,
    updateDailyPicker,
    updateSinglePicker,
  ]);

  return { actions, draft, hydrate, reset, view };
}
