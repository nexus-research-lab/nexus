"use client";

import { build_room_agent_session_key } from "@/lib/conversation/session-key";
import type { RoomContextAggregate, RoomSessionSelection } from "@/types/conversation/room";
import type { ScheduledTaskSchedule } from "@/types/capability/scheduled-task";

import { type Weekday, WEEKDAY_OPTIONS } from "../pickers/picker-utils";

export type ScheduleKind = ScheduledTaskSchedule["kind"];
export type EveryUnit = "seconds" | "minutes" | "hours";
export type TargetType = "agent" | "room";
export type ExecutionMode = "main" | "existing" | "temporary" | "dedicated";
export type ReplyMode = "none" | "execution" | "selected";

interface ChoiceDef<TValue extends string> {
  key: TValue;
  label: string;
}

export const TARGET_TYPE_OPTIONS: ChoiceDef<TargetType>[] = [
  { key: "agent", label: "智能体" },
  { key: "room", label: "Room" },
];

export const SCHEDULE_OPTIONS: ChoiceDef<ScheduleKind>[] = [
  { key: "at", label: "单次" },
  { key: "cron", label: "每天" },
  { key: "every", label: "间隔" },
];

export const EVERY_UNIT_OPTIONS: ChoiceDef<EveryUnit>[] = [
  { key: "seconds", label: "秒" },
  { key: "minutes", label: "分钟" },
  { key: "hours", label: "小时" },
];

export const EXECUTION_MODE_OPTIONS: ChoiceDef<ExecutionMode>[] = [
  { key: "main", label: "使用主会话" },
  { key: "existing", label: "使用现有会话" },
  { key: "temporary", label: "每次新建临时会话" },
  { key: "dedicated", label: "使用专用长期会话" },
];

export const REPLY_MODE_OPTIONS: ChoiceDef<ReplyMode>[] = [
  { key: "none", label: "不回传" },
  { key: "execution", label: "回到执行会话" },
  { key: "selected", label: "回到指定会话" },
];

export const TIMEZONE_OPTIONS = [
  "Asia/Shanghai",
  "Asia/Tokyo",
  "UTC",
  "America/Los_Angeles",
  "America/New_York",
  "Europe/London",
];

export function get_default_timezone(): string {
  if (typeof Intl === "undefined") {
    return "Asia/Shanghai";
  }
  return Intl.DateTimeFormat().resolvedOptions().timeZone || "Asia/Shanghai";
}

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
  const part_map = new Map(
    formatter.formatToParts(date)
      .filter((part) => part.type !== "literal")
      .map((part) => [part.type, part.value]),
  );
  return {
    year: part_map.get("year") || "1970",
    month: part_map.get("month") || "01",
    day: part_map.get("day") || "01",
    hour: part_map.get("hour") || "00",
    minute: part_map.get("minute") || "00",
    second: part_map.get("second") || "00",
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
  const [, year_text, month_text, day_text, hour_text, minute_text, second_text] = match;
  return {
    year: Number(year_text),
    month: Number(month_text),
    day: Number(day_text),
    hour: Number(hour_text),
    minute: Number(minute_text),
    second: Number(second_text || "00"),
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
    const desired_utc = Date.UTC(parsed.year, parsed.month - 1, parsed.day, parsed.hour, parsed.minute, parsed.second);
    const current_utc = Date.UTC(
      Number(zoned.year),
      Number(zoned.month) - 1,
      Number(zoned.day),
      Number(zoned.hour),
      Number(zoned.minute),
      Number(zoned.second),
    );
    const diff = current_utc - desired_utc;
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

export function build_daily_cron_expression(time_value: string, weekdays: Weekday[]): string | null {
  const normalized = time_value.trim();
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
  const weekday_expression = WEEKDAY_OPTIONS
    .filter((option) => weekdays.includes(option.key))
    .map((option) => String(option.cron_value))
    .join(",");
  return `${minute} ${hour} * * ${weekday_expression}`;
}

export function parse_daily_cron_expression(
  cron_expression: string,
): { daily_time: string; selected_weekdays: Weekday[] } | null {
  const parts = cron_expression.trim().split(/\s+/);
  if (parts.length !== 5) {
    return null;
  }

  const [minute_text, hour_text, day_of_month, month, day_of_week] = parts;
  if (day_of_month !== "*" || month !== "*") {
    return null;
  }

  const hour = Number(hour_text);
  const minute = Number(minute_text);
  if (!Number.isInteger(hour) || !Number.isInteger(minute) || hour < 0 || hour > 23 || minute < 0 || minute > 59) {
    return null;
  }

  const cron_value_to_weekday = new Map(WEEKDAY_OPTIONS.map((option) => [String(option.cron_value), option.key]));
  const selected_weekdays = day_of_week === "*"
    ? WEEKDAY_OPTIONS.map((option) => option.key)
    : day_of_week
      .split(",")
      .map((value) => cron_value_to_weekday.get(value.trim()))
      .filter((value): value is Weekday => Boolean(value));

  if (selected_weekdays.length === 0) {
    return null;
  }

  return {
    daily_time: `${String(hour).padStart(2, "0")}:${String(minute).padStart(2, "0")}`,
    selected_weekdays,
  };
}

export function to_interval_seconds(value: string, unit: EveryUnit): number | null {
  const normalized_value = value.trim();
  if (!/^\d+$/.test(normalized_value)) {
    return null;
  }
  const numeric_value = Number(normalized_value);
  if (!Number.isInteger(numeric_value) || numeric_value <= 0) {
    return null;
  }
  if (unit === "hours") {
    return numeric_value * 3600;
  }
  if (unit === "minutes") {
    return numeric_value * 60;
  }
  return numeric_value;
}

export function format_session_label(title: string, agent_name: string): string {
  return `${title} · ${agent_name}`;
}

export function build_room_session_selections(
  contexts: RoomContextAggregate[],
  agent_name_by_id: Map<string, string>,
): RoomSessionSelection[] {
  return contexts.flatMap((context) => {
    const room_title = context.conversation.title?.trim() || context.room.name?.trim() || "未命名会话";
    const room_type = context.room.room_type;

    return context.sessions.map((session) => ({
      session_key: build_room_agent_session_key(
        context.conversation.id,
        session.agent_id,
        room_type === "dm" ? "dm" : "room",
      ),
      agent_id: session.agent_id,
      room_id: context.room.id,
      conversation_id: context.conversation.id,
      room_type,
      title: room_title,
      session,
      label: format_session_label(room_title, agent_name_by_id.get(session.agent_id) || session.agent_id),
    }));
  });
}
