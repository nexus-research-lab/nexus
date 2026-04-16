/**
 * =====================================================
 * @File   : picker-types.ts
 * @Date   : 2026-04-16 14:28
 * @Author : leemysw
 * 2026-04-16 14:28   Create
 * =====================================================
 */

export type Weekday = "mo" | "tu" | "we" | "th" | "fr" | "sa" | "su";
export type Meridiem = "am" | "pm";

export const HOUR_12_OPTIONS = Array.from({ length: 12 }, (_, index) => `${index + 1}`.padStart(2, "0"));
export const MINUTE_OPTIONS = Array.from({ length: 60 }, (_, index) => `${index}`.padStart(2, "0"));
export const SECOND_OPTIONS = Array.from({ length: 60 }, (_, index) => `${index}`.padStart(2, "0"));

export const WEEKDAY_OPTIONS: Array<{ key: Weekday; short_label: string; cron_value: number }> = [
  { key: "mo", short_label: "一", cron_value: 1 },
  { key: "tu", short_label: "二", cron_value: 2 },
  { key: "we", short_label: "三", cron_value: 3 },
  { key: "th", short_label: "四", cron_value: 4 },
  { key: "fr", short_label: "五", cron_value: 5 },
  { key: "sa", short_label: "六", cron_value: 6 },
  { key: "su", short_label: "日", cron_value: 0 },
];
