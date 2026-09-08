// INPUT: 定时计划、可空时间戳与调用方指定的日期语言/缺省文案。
// OUTPUT: 有限合法日期的显示文本与既有计划摘要；无效日期退回缺省文案。
// POS: Scheduled 日期/计划纯格式化；历史消费者传入当前语言，既有计划摘要保留原协议。

import type { ScheduledTaskSchedule } from "@/types/capability/scheduled-task/task";

interface FormatScheduledDatetimeOptions {
  emptyLabel?: string;
  includeSeconds?: boolean;
  locale?: string;
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
    locale = "zh-CN",
  } = options;

  if (value === null || !Number.isFinite(value) || !Number.isFinite(new Date(value).getTime())) {
    return emptyLabel;
  }

  return new Intl.DateTimeFormat(locale, {
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
  const day = Number(dayOfMonth);
  const isFixedTime = Number.isInteger(hour)
    && hour >= 0
    && hour <= 23
    && Number.isInteger(minute)
    && minute >= 0
    && minute <= 59;
  if (
    isFixedTime
    && Number.isInteger(day)
    && day >= 1
    && day <= 31
    && month === "*"
    && weekdayText === "*"
  ) {
    return `每月 ${day} 日 ${String(hour).padStart(2, "0")}:${String(minute).padStart(2, "0")}`;
  }
  const weekdays = formatCronWeekdays(weekdayText);
  const isDailySchedule = dayOfMonth === "*"
    && month === "*"
    && isFixedTime
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
