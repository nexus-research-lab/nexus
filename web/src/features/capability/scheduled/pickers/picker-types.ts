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

export const WEEKDAY_OPTIONS: Array<{ key: Weekday; shortLabel: string; cronValue: number }> = [
  { key: "mo", shortLabel: "一", cronValue: 1 },
  { key: "tu", shortLabel: "二", cronValue: 2 },
  { key: "we", shortLabel: "三", cronValue: 3 },
  { key: "th", shortLabel: "四", cronValue: 4 },
  { key: "fr", shortLabel: "五", cronValue: 5 },
  { key: "sa", shortLabel: "六", cronValue: 6 },
  { key: "su", shortLabel: "日", cronValue: 0 },
];
