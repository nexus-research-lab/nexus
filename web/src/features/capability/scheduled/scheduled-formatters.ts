// INPUT: 定时计划、可空时间戳与调用方指定的日期语言/缺省文案。
// OUTPUT: 有限合法日期的显示文本与既有计划摘要；无效日期退回缺省文案。
// POS: Scheduled 日期/计划纯格式化；调用方传入当前语言，计划协议与时区不变。

import type { I18nContextValue } from "@/shared/i18n/i18n-context";
import type { TranslationKey } from "@/shared/i18n/messages";
import type { ScheduledTaskSchedule } from "@/types/capability/scheduled-task/task";

interface FormatScheduledDatetimeOptions {
  emptyLabel?: string;
  includeSeconds?: boolean;
  locale?: string;
}

type Translate = I18nContextValue["t"];

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

function formatInterval(seconds: number, t: Translate): string {
  const units = [
    { key: "capability.scheduled_board_interval_days", seconds: 86_400 },
    { key: "capability.scheduled_board_interval_hours", seconds: 3_600 },
    { key: "capability.scheduled_board_interval_minutes", seconds: 60 },
  ] as const;
  const unit = units.find((candidate) => seconds % candidate.seconds === 0);
  return unit
    ? t(unit.key, { count: seconds / unit.seconds })
    : t("capability.scheduled_board_interval_seconds", { count: seconds });
}

function formatCronWeekdays(value: string, t: Translate): string | null {
  if (value === "*") {
    return t("capability.scheduled_board_daily");
  }
  const values = value.split(",").map((item) => item.trim());
  if (values.join(",") === "1,2,3,4,5") {
    return t("capability.scheduled_board_weekdays");
  }
  if (values.join(",") === "0,6" || values.join(",") === "6,0") {
    return t("capability.scheduled_board_weekends");
  }
  return values.every((item) => /^[0-6]$/.test(item))
    ? values.map((item) => t(`capability.scheduled_board_weekday_${item}` as TranslationKey)).join(" / ")
    : null;
}

function formatCronSchedule(expression: string, t: Translate): string {
  const fields = expression.trim().split(/\s+/);
  if (fields.length !== 5) {
    return t("capability.scheduled_board_custom_schedule");
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
    return t("capability.scheduled_board_monthly", { day, time: `${String(hour).padStart(2, "0")}:${String(minute).padStart(2, "0")}` });
  }
  const weekdays = formatCronWeekdays(weekdayText, t);
  const isDailySchedule = dayOfMonth === "*"
    && month === "*"
    && isFixedTime
    && weekdays;
  if (!isDailySchedule) {
    return t("capability.scheduled_board_custom_schedule");
  }
  return `${weekdays} ${String(hour).padStart(2, "0")}:${String(minute).padStart(2, "0")}`;
}

export function formatScheduledTaskSchedule(schedule: ScheduledTaskSchedule, t: Translate, locale = "zh"): string {
  if (schedule.kind === "every") {
    return t("capability.scheduled_board_every", { interval: formatInterval(schedule.interval_seconds, t) });
  }
  if (schedule.kind === "cron") {
    return formatCronSchedule(schedule.cron_expression, t);
  }
  return t("capability.scheduled_board_once", { time: formatScheduledDatetime(new Date(schedule.run_at).getTime(), {
    emptyLabel: t("capability.scheduled_board_not_scheduled"), locale,
  }) });
}
