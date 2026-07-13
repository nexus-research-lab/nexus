import {
  formatDatetimeLocalInput,
  formatTimeLocalInput,
} from "../../pickers/picker-formatters";
import { WEEKDAY_OPTIONS } from "../../pickers/picker-types";
import type {
  ChoiceDef,
  EveryUnit,
  ScheduleKind,
  TaskScheduleDraft,
} from "../scheduled-task-dialog-types";

export const SCHEDULE_OPTIONS: ChoiceDef<ScheduleKind>[] = [
  { key: "at", label: "单次" },
  { key: "cron", label: "每天" },
  { key: "every", label: "间隔" },
];

export const EVERY_UNIT_OPTIONS: ChoiceDef<EveryUnit>[] = [
  { key: "seconds", label: "秒" },
  { key: "minutes", label: "分钟" },
  { key: "hours", label: "小时" },
];

export const TIMEZONE_OPTIONS = [
  "Asia/Shanghai",
  "Asia/Tokyo",
  "UTC",
  "America/Los_Angeles",
  "America/New_York",
  "Europe/London",
];

export function getDefaultTimezone(): string {
  if (typeof Intl === "undefined") {
    return "Asia/Shanghai";
  }
  return Intl.DateTimeFormat().resolvedOptions().timeZone || "Asia/Shanghai";
}

export function createDefaultTaskSchedule(
  now = new Date(),
  timezone = getDefaultTimezone(),
): TaskScheduleDraft {
  const nextHour = new Date(now.getTime() + 3600_000);
  return {
    dailyTime: formatTimeLocalInput(nextHour),
    everyUnit: "minutes",
    everyValue: "30",
    kind: "every",
    runAt: formatDatetimeLocalInput(nextHour),
    selectedWeekdays: WEEKDAY_OPTIONS.map((option) => option.key),
    timezone,
  };
}
