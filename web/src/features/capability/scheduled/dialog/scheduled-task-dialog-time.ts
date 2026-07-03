/**
 * =====================================================
 * @File   : scheduled-task-dialog-time.ts
 * @Date   : 2026-04-16 14:28
 * @Author : leemysw
 * 2026-04-16 14:28   Create
 * =====================================================
 */

"use client";

import { buildRoomSharedSessionKey } from "@/lib/conversation/session-key";
import type { RoomContextAggregate, RoomSessionSelection } from "@/types/conversation/room";

import { type Weekday, WEEKDAY_OPTIONS } from "../pickers/picker-types";
import type { EveryUnit } from "./scheduled-task-dialog-types";

function formatZonedParts(date: Date, timezone: string): {
  year: string;
  month: string;
  day: string;
  hour: string;
  minute: string;
  second: string;
} {
  const formatter = new Intl.DateTimeFormat("en-CA", {
    timeZone: timezone,
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    hourCycle: "h23",
  });
  const partMap = new Map(
    formatter.formatToParts(date)
      .filter((part) => part.type !== "literal")
      .map((part) => [part.type, part.value]),
  );
  return {
    year: partMap.get("year") || "1970",
    month: partMap.get("month") || "01",
    day: partMap.get("day") || "01",
    hour: partMap.get("hour") || "00",
    minute: partMap.get("minute") || "00",
    second: partMap.get("second") || "00",
  };
}

function parseDatetimeLocalInput(value: string): {
  year: number;
  month: number;
  day: number;
  hour: number;
  minute: number;
  second: number;
} | null {
  const match = value.trim().match(/^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2})(?::(\d{2}))?$/);
  if (!match) {
    return null;
  }
  const [, yearText, monthText, dayText, hourText, minuteText, secondText] = match;
  return {
    year: Number(yearText),
    month: Number(monthText),
    day: Number(dayText),
    hour: Number(hourText),
    minute: Number(minuteText),
    second: Number(secondText || "00"),
  };
}

export function zonedDateTimeToEpochMs(value: string, timezone: string): number | null {
  const parsed = parseDatetimeLocalInput(value);
  if (!parsed) {
    return null;
  }
  let candidate = Date.UTC(
    parsed.year,
    parsed.month - 1,
    parsed.day,
    parsed.hour,
    parsed.minute,
    parsed.second,
  );
  for (let index = 0; index < 3; index += 1) {
    const zoned = formatZonedParts(new Date(candidate), timezone);
    const desiredUtc = Date.UTC(parsed.year, parsed.month - 1, parsed.day, parsed.hour, parsed.minute, parsed.second);
    const currentUtc = Date.UTC(
      Number(zoned.year),
      Number(zoned.month) - 1,
      Number(zoned.day),
      Number(zoned.hour),
      Number(zoned.minute),
      Number(zoned.second),
    );
    const diff = currentUtc - desiredUtc;
    if (diff === 0) {
      break;
    }
    candidate -= diff;
  }
  const verified = formatZonedParts(new Date(candidate), timezone);
  if (
    Number(verified.year) !== parsed.year
    || Number(verified.month) !== parsed.month
    || Number(verified.day) !== parsed.day
    || Number(verified.hour) !== parsed.hour
    || Number(verified.minute) !== parsed.minute
    || Number(verified.second) !== parsed.second
  ) {
    return null;
  }
  return candidate;
}

export function isoToZonedLocalInput(value: string, timezone: string): string | null {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return null;
  }
  const parts = formatZonedParts(date, timezone);
  return `${parts.year}-${parts.month}-${parts.day}T${parts.hour}:${parts.minute}:${parts.second}`;
}

export function buildDailyCronExpression(timeValue: string, weekdays: Weekday[]): string | null {
  const normalized = timeValue.trim();
  const match = normalized.match(/^(\d{2}):(\d{2})$/);
  if (!match) {
    return null;
  }
  const hour = Number(match[1]);
  const minute = Number(match[2]);
  if (hour < 0 || hour > 23 || minute < 0 || minute > 59) {
    return null;
  }
  if (weekdays.length === 0) {
    return null;
  }
  if (weekdays.length === WEEKDAY_OPTIONS.length) {
    return `${minute} ${hour} * * *`;
  }
  const weekdayExpression = WEEKDAY_OPTIONS
    .filter((option) => weekdays.includes(option.key))
    .map((option) => String(option.cronValue))
    .join(",");
  return `${minute} ${hour} * * ${weekdayExpression}`;
}

export function parseDailyCronExpression(
  cronExpression: string,
): { dailyTime: string; selectedWeekdays: Weekday[] } | null {
  const parts = cronExpression.trim().split(/\s+/);
  if (parts.length !== 5) {
    return null;
  }

  const [minuteText, hourText, dayOfMonth, month, dayOfWeek] = parts;
  if (dayOfMonth !== "*" || month !== "*") {
    return null;
  }

  const hour = Number(hourText);
  const minute = Number(minuteText);
  if (!Number.isInteger(hour) || !Number.isInteger(minute) || hour < 0 || hour > 23 || minute < 0 || minute > 59) {
    return null;
  }

  const cronValueToWeekday = new Map(WEEKDAY_OPTIONS.map((option) => [String(option.cronValue), option.key]));
  const selectedWeekdays = dayOfWeek === "*"
    ? WEEKDAY_OPTIONS.map((option) => option.key)
    : dayOfWeek
      .split(",")
      .map((value) => cronValueToWeekday.get(value.trim()))
      .filter((value): value is Weekday => Boolean(value));

  if (selectedWeekdays.length === 0) {
    return null;
  }

  return {
    dailyTime: `${String(hour).padStart(2, "0")}:${String(minute).padStart(2, "0")}`,
    selectedWeekdays: selectedWeekdays,
  };
}

export function toIntervalSeconds(value: string, unit: EveryUnit): number | null {
  const normalizedValue = value.trim();
  if (!/^\d+$/.test(normalizedValue)) {
    return null;
  }
  const numericValue = Number(normalizedValue);
  if (!Number.isInteger(numericValue) || numericValue <= 0) {
    return null;
  }
  if (unit === "hours") {
    return numericValue * 3600;
  }
  if (unit === "minutes") {
    return numericValue * 60;
  }
  return numericValue;
}

export function formatSessionLabel(title: string, agentName: string): string {
  return `${title} · ${agentName}`;
}

export function buildRoomSessionSelections(
  contexts: RoomContextAggregate[],
  agentNameById: Map<string, string>,
): RoomSessionSelection[] {
  return contexts.flatMap((context) => {
    const roomTitle = context.conversation.title?.trim() || context.room.name?.trim() || "未命名会话";
    const roomType = context.room.room_type;
    return context.sessions.map((session) => {
      const agentName = agentNameById.get(session.agent_id) || session.agent_id;
      const label = roomType === "group"
        ? `${roomTitle} · ${agentName}`
        : `${agentName} · ${roomTitle}`;
      const sharedSessionKey = buildRoomSharedSessionKey(context.conversation.id);
      return {
        value: buildRoomExecutorSelectionKey(sharedSessionKey, session.agent_id),
        session_key: sharedSessionKey,
        agent_id: session.agent_id,
        room_id: context.room.id,
        conversation_id: context.conversation.id,
        room_type: roomType,
        title: roomTitle,
        session,
        label,
      };
    });
  });
}

export function buildRoomExecutorSelectionKey(sharedSessionKey: string, agentId: string): string {
  return `${sharedSessionKey.trim()}::executor:${agentId.trim()}`;
}
