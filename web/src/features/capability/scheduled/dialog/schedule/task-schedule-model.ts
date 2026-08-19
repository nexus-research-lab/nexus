import type { I18nContextValue } from "@/shared/i18n/i18n-context";

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

type Translate = I18nContextValue["t"];

export function buildScheduleOptions(t: Translate): ChoiceDef<ScheduleKind>[] {
  return [
    { key: "at", label: t("capability.scheduled_dialog_schedule_once") },
    { key: "cron", label: t("capability.scheduled_dialog_schedule_daily") },
    { key: "monthly", label: t("capability.scheduled_dialog_schedule_monthly") },
    { key: "every", label: t("capability.scheduled_dialog_schedule_interval") },
    { key: "custom", label: t("capability.scheduled_dialog_schedule_custom") },
  ];
}

export function buildEveryUnitOptions(t: Translate): ChoiceDef<EveryUnit>[] {
  return [
    { key: "seconds", label: t("capability.scheduled_dialog_seconds") },
    { key: "minutes", label: t("capability.scheduled_dialog_minutes") },
    { key: "hours", label: t("capability.scheduled_dialog_hours") },
  ];
}

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
    cronExpression: "0 9 * * *",
    dailyTime: formatTimeLocalInput(nextHour),
    everyUnit: "minutes",
    everyValue: "30",
    kind: "every",
    monthlyDay: "1",
    runAt: formatDatetimeLocalInput(nextHour),
    selectedWeekdays: WEEKDAY_OPTIONS.map((option) => option.key),
    timezone,
  };
}
