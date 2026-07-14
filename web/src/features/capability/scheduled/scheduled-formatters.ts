/**
 * =====================================================
 * @File   : scheduled-formatters.ts
 * @Date   : 2026-04-16 14:00
 * @Author : leemysw
 * 2026-04-16 14:00   Create
 * =====================================================
 */

import type { ScheduledTaskSchedule } from "@/types/capability/scheduled-task/task";

interface FormatScheduledDatetimeOptions {
  emptyLabel?: string;
  includeSeconds?: boolean;
}

const WEEKDAY_LABELS: Record<string, string> = {
  "0": "日",
  "1": "一",
  "2": "二",
  "3": "三",
  "4": "四",
  "5": "五",
  "6": "六",
};

export function formatScheduledDatetime(
  value: number | null,
  options: FormatScheduledDatetimeOptions = {},
): string {
  const {
    emptyLabel = "未记录",
    includeSeconds = false,
  } = options;

  if (!value) {
    return emptyLabel;
  }

  return new Intl.DateTimeFormat("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    ...(includeSeconds ? { second: "2-digit" as const } : {}),
  }).format(value);
}

function formatInterval(seconds: number): string {
  const units = [
    { label: "天", seconds: 86_400 },
    { label: "小时", seconds: 3_600 },
    { label: "分钟", seconds: 60 },
  ];
  const unit = units.find((candidate) => seconds % candidate.seconds === 0);
  return unit
    ? `${seconds / unit.seconds} ${unit.label}`
    : `${seconds} 秒`;
}

function formatCronWeekdays(value: string): string | null {
  if (value === "*") {
    return "每天";
  }
  const values = value.split(",").map((item) => item.trim());
  if (values.join(",") === "1,2,3,4,5") {
    return "工作日";
  }
  if (values.join(",") === "0,6" || values.join(",") === "6,0") {
    return "周末";
  }
  const labels = values.map((item) => WEEKDAY_LABELS[item]);
  return labels.every(Boolean) ? labels.map((label) => `周${label}`).join("、") : null;
}

function formatCronSchedule(expression: string): string {
  const fields = expression.trim().split(/\s+/);
  if (fields.length !== 5) {
    return "自定义计划";
  }
  const [minuteText, hourText, dayOfMonth, month, weekdayText] = fields;
  const minute = Number(minuteText);
  const hour = Number(hourText);
  const weekdays = formatCronWeekdays(weekdayText);
  const isDailySchedule = dayOfMonth === "*"
    && month === "*"
    && Number.isInteger(hour)
    && hour >= 0
    && hour <= 23
    && Number.isInteger(minute)
    && minute >= 0
    && minute <= 59
    && weekdays;
  if (!isDailySchedule) {
    return "自定义计划";
  }
  return `${weekdays} ${String(hour).padStart(2, "0")}:${String(minute).padStart(2, "0")}`;
}

export function formatScheduledTaskSchedule(schedule: ScheduledTaskSchedule): string {
  if (schedule.kind === "every") {
    return `每 ${formatInterval(schedule.interval_seconds)}`;
  }
  if (schedule.kind === "cron") {
    return formatCronSchedule(schedule.cron_expression);
  }
  return `单次 · ${formatScheduledDatetime(new Date(schedule.run_at).getTime(), {
    emptyLabel: "未安排",
  })}`;
}
