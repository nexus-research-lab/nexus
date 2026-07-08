/**
 * =====================================================
 * @File   : picker-formatters.ts
 * @Date   : 2026-04-16 14:28
 * @Author : leemysw
 * 2026-04-16 14:28   Create
 * =====================================================
 */

import { type Meridiem } from "./picker-types";

export function formatTimeLocalInput(date: Date): string {
  const hour = `${date.getHours()}`.padStart(2, "0");
  const minute = `${date.getMinutes()}`.padStart(2, "0");
  return `${hour}:${minute}`;
}

export function formatDatetimeLocalInput(date: Date): string {
  const year = date.getFullYear();
  const month = `${date.getMonth() + 1}`.padStart(2, "0");
  const day = `${date.getDate()}`.padStart(2, "0");
  const hour = `${date.getHours()}`.padStart(2, "0");
  const minute = `${date.getMinutes()}`.padStart(2, "0");
  const second = `${date.getSeconds()}`.padStart(2, "0");
  return `${year}-${month}-${day}T${hour}:${minute}:${second}`;
}

export function splitTimeValue(timeValue: string): { hour: string; minute: string } {
  const normalized = timeValue.trim();
  const match = normalized.match(/^(\d{2}):(\d{2})$/);
  if (!match) {
    return { hour: "08", minute: "00" };
  }
  return { hour: match[1], minute: match[2] };
}

export function buildTimeValue(hour: string, minute: string): string {
  return `${hour}:${minute}`;
}

export function toMeridiemParts(hour24: string, minute: string, second: string = "00"): {
  meridiem: Meridiem;
  hour12: string;
  minute: string;
  second: string;
} {
  const hour = Number(hour24);
  const normalizedHour = Number.isFinite(hour) ? hour : 0;
  const meridiem: Meridiem = normalizedHour >= 12 ? "pm" : "am";
  const hour12 = normalizedHour % 12 === 0 ? 12 : normalizedHour % 12;
  return {
    meridiem,
    hour12: `${hour12}`.padStart(2, "0"),
    minute,
    second,
  };
}

export function fromMeridiemParts(meridiem: Meridiem, hour12: string, minute: string, second: string = "00"): {
  hour24: string;
  minute: string;
  second: string;
} {
  const hour = Math.min(12, Math.max(1, Number(hour12) || 1));
  const normalizedMinute = `${Math.min(59, Math.max(0, Number(minute) || 0))}`.padStart(2, "0");
  const normalizedSecond = `${Math.min(59, Math.max(0, Number(second) || 0))}`.padStart(2, "0");
  let hour24 = hour % 12;
  if (meridiem === "pm") {
    hour24 += 12;
  }
  return {
    hour24: `${hour24}`.padStart(2, "0"),
    minute: normalizedMinute,
    second: normalizedSecond,
  };
}

export function formatTimeDisplay(hour24: string, minute: string): string {
  const parts = toMeridiemParts(hour24, minute);
  return `${parts.meridiem === "am" ? "上午" : "下午"} ${parts.hour12}:${parts.minute}`;
}

export function formatDatetimeDisplay(dateValue: string, hour24: string, minute: string, second: string = "00"): string {
  const [year, month, day] = dateValue.split("-");
  const parts = toMeridiemParts(hour24, minute, second);
  return `${day}/${month}/${year} ${parts.meridiem === "am" ? "上午" : "下午"} ${parts.hour12}:${parts.minute}:${parts.second}`;
}

export function splitDatetimeLocalInput(value: string): { date: string; hour: string; minute: string; second: string } {
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

export function buildDatetimeLocalInput(dateValue: string, hour: string, minute: string, second: string = "00"): string {
  return `${dateValue}T${hour}:${minute}:${second}`;
}

export function buildCalendarDays(monthKey: string): Array<{ label: string; value: string; muted: boolean }> {
  const [yearText, monthText] = monthKey.split("-");
  const year = Number(yearText);
  const month = Number(monthText);
  if (!Number.isFinite(year) || !Number.isFinite(month)) {
    return [];
  }

  const firstDay = new Date(year, month - 1, 1);
  const startWeekday = firstDay.getDay();
  const daysInMonth = new Date(year, month, 0).getDate();
  const prevMonthDays = new Date(year, month - 1, 0).getDate();
  const cells: Array<{ label: string; value: string; muted: boolean }> = [];

  for (let index = startWeekday - 1; index >= 0; index -= 1) {
    const day = prevMonthDays - index;
    const prevMonth = new Date(year, month - 2, day);
    cells.push({
      label: String(day),
      value: `${prevMonth.getFullYear()}-${`${prevMonth.getMonth() + 1}`.padStart(2, "0")}-${`${prevMonth.getDate()}`.padStart(2, "0")}`,
      muted: true,
    });
  }

  for (let day = 1; day <= daysInMonth; day += 1) {
    cells.push({
      label: String(day),
      value: `${year}-${`${month}`.padStart(2, "0")}-${`${day}`.padStart(2, "0")}`,
      muted: false,
    });
  }

  while (cells.length % 7 !== 0 || cells.length < 35) {
    const nextDay = cells.length - (startWeekday + daysInMonth) + 1;
    const nextMonth = new Date(year, month, nextDay);
    cells.push({
      label: String(nextDay),
      value: `${nextMonth.getFullYear()}-${`${nextMonth.getMonth() + 1}`.padStart(2, "0")}-${`${nextMonth.getDate()}`.padStart(2, "0")}`,
      muted: true,
    });
  }

  return cells;
}
