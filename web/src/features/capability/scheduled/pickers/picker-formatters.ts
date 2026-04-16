/**
 * =====================================================
 * @File   : picker-formatters.ts
 * @Date   : 2026-04-16 14:28
 * @Author : leemysw
 * 2026-04-16 14:28   Create
 * =====================================================
 */

import { type Meridiem } from "./picker-types";

export function format_time_local_input(date: Date): string {
  const hour = `${date.getHours()}`.padStart(2, "0");
  const minute = `${date.getMinutes()}`.padStart(2, "0");
  return `${hour}:${minute}`;
}

export function format_datetime_local_input(date: Date): string {
  const year = date.getFullYear();
  const month = `${date.getMonth() + 1}`.padStart(2, "0");
  const day = `${date.getDate()}`.padStart(2, "0");
  const hour = `${date.getHours()}`.padStart(2, "0");
  const minute = `${date.getMinutes()}`.padStart(2, "0");
  const second = `${date.getSeconds()}`.padStart(2, "0");
  return `${year}-${month}-${day}T${hour}:${minute}:${second}`;
}

export function split_time_value(time_value: string): { hour: string; minute: string } {
  const normalized = time_value.trim();
  const match = normalized.match(/^(\d{2}):(\d{2})$/);
  if (!match) {
    return { hour: "08", minute: "00" };
  }
  return { hour: match[1], minute: match[2] };
}

export function build_time_value(hour: string, minute: string): string {
  return `${hour}:${minute}`;
}

export function to_meridiem_parts(hour_24: string, minute: string, second: string = "00"): {
  meridiem: Meridiem;
  hour12: string;
  minute: string;
  second: string;
} {
  const hour = Number(hour_24);
  const normalized_hour = Number.isFinite(hour) ? hour : 0;
  const meridiem: Meridiem = normalized_hour >= 12 ? "pm" : "am";
  const hour12 = normalized_hour % 12 === 0 ? 12 : normalized_hour % 12;
  return {
    meridiem,
    hour12: `${hour12}`.padStart(2, "0"),
    minute,
    second,
  };
}

export function from_meridiem_parts(meridiem: Meridiem, hour12: string, minute: string, second: string = "00"): {
  hour24: string;
  minute: string;
  second: string;
} {
  const hour = Math.min(12, Math.max(1, Number(hour12) || 1));
  const normalized_minute = `${Math.min(59, Math.max(0, Number(minute) || 0))}`.padStart(2, "0");
  const normalized_second = `${Math.min(59, Math.max(0, Number(second) || 0))}`.padStart(2, "0");
  let hour24 = hour % 12;
  if (meridiem === "pm") {
    hour24 += 12;
  }
  return {
    hour24: `${hour24}`.padStart(2, "0"),
    minute: normalized_minute,
    second: normalized_second,
  };
}

export function format_time_display(hour24: string, minute: string): string {
  const parts = to_meridiem_parts(hour24, minute);
  return `${parts.meridiem === "am" ? "上午" : "下午"} ${parts.hour12}:${parts.minute}`;
}

export function format_datetime_display(date_value: string, hour24: string, minute: string, second: string = "00"): string {
  const [year, month, day] = date_value.split("-");
  const parts = to_meridiem_parts(hour24, minute, second);
  return `${day}/${month}/${year} ${parts.meridiem === "am" ? "上午" : "下午"} ${parts.hour12}:${parts.minute}:${parts.second}`;
}

export function split_datetime_local_input(value: string): { date: string; hour: string; minute: string; second: string } {
  const match = value.match(/^(\d{4}-\d{2}-\d{2})T(\d{2}):(\d{2})(?::(\d{2}))?$/);
  if (!match) {
    const fallback = new Date(Date.now() + 3600_000);
    return {
      date: `${fallback.getFullYear()}-${`${fallback.getMonth() + 1}`.padStart(2, "0")}-${`${fallback.getDate()}`.padStart(2, "0")}`,
      hour: `${fallback.getHours()}`.padStart(2, "0"),
      minute: `${fallback.getMinutes()}`.padStart(2, "0"),
      second: `${fallback.getSeconds()}`.padStart(2, "0"),
    };
  }
  return { date: match[1], hour: match[2], minute: match[3], second: match[4] ?? "00" };
}

export function build_datetime_local_input(date_value: string, hour: string, minute: string, second: string = "00"): string {
  return `${date_value}T${hour}:${minute}:${second}`;
}

export function build_calendar_days(month_key: string): Array<{ label: string; value: string; muted: boolean }> {
  const [year_text, month_text] = month_key.split("-");
  const year = Number(year_text);
  const month = Number(month_text);
  if (!Number.isFinite(year) || !Number.isFinite(month)) {
    return [];
  }

  const first_day = new Date(year, month - 1, 1);
  const start_weekday = first_day.getDay();
  const days_in_month = new Date(year, month, 0).getDate();
  const prev_month_days = new Date(year, month - 1, 0).getDate();
  const cells: Array<{ label: string; value: string; muted: boolean }> = [];

  for (let index = start_weekday - 1; index >= 0; index -= 1) {
    const day = prev_month_days - index;
    const prev_month = new Date(year, month - 2, day);
    cells.push({
      label: String(day),
      value: `${prev_month.getFullYear()}-${`${prev_month.getMonth() + 1}`.padStart(2, "0")}-${`${prev_month.getDate()}`.padStart(2, "0")}`,
      muted: true,
    });
  }

  for (let day = 1; day <= days_in_month; day += 1) {
    cells.push({
      label: String(day),
      value: `${year}-${`${month}`.padStart(2, "0")}-${`${day}`.padStart(2, "0")}`,
      muted: false,
    });
  }

  while (cells.length % 7 !== 0 || cells.length < 35) {
    const next_day = cells.length - (start_weekday + days_in_month) + 1;
    const next_month = new Date(year, month, next_day);
    cells.push({
      label: String(next_day),
      value: `${next_month.getFullYear()}-${`${next_month.getMonth() + 1}`.padStart(2, "0")}-${`${next_month.getDate()}`.padStart(2, "0")}`,
      muted: true,
    });
  }

  return cells;
}
