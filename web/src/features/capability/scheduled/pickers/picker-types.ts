export type Weekday = "mo" | "tu" | "we" | "th" | "fr" | "sa" | "su";
export type Meridiem = "am" | "pm";

export const MERIDIEM_OPTIONS: readonly Meridiem[] = ["am", "pm"];
export const HOUR_12_OPTIONS = Array.from({ length: 12 }, (_, index) => `${index + 1}`.padStart(2, "0"));
export const MINUTE_OPTIONS = Array.from({ length: 60 }, (_, index) => `${index}`.padStart(2, "0"));
export const SECOND_OPTIONS = Array.from({ length: 60 }, (_, index) => `${index}`.padStart(2, "0"));

export const WEEKDAY_OPTIONS: Array<{ key: Weekday; cronValue: number }> = [
  { key: "mo", cronValue: 1 },
  { key: "tu", cronValue: 2 },
  { key: "we", cronValue: 3 },
  { key: "th", cronValue: 4 },
  { key: "fr", cronValue: 5 },
  { key: "sa", cronValue: 6 },
  { key: "su", cronValue: 0 },
];
