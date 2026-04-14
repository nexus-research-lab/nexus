"use client";

import { buildRoomAgentSessionKey } from "@/lib/session-key";
import type { RoomContextAggregate, RoomSessionSelection } from "@/types/room";
import type { ScheduledTaskSchedule } from "@/types/scheduled-task";

import { type Weekday, WEEKDAY_OPTIONS } from "../pickers/picker-utils";

export type ScheduleKind = ScheduledTaskSchedule["kind"];
export type EveryUnit = "seconds" | "minutes" | "hours";
export type TargetType = "agent" | "room";
export type ExecutionMode = "existing" | "temporary" | "dedicated";
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

export function getDefaultTimezone(): string {
  if (typeof Intl === "undefined") {
    return "Asia/Shanghai";
  }
  return Intl.DateTimeFormat().resolvedOptions().timeZone || "Asia/Shanghai";
}

export function buildDailyCronExpression(time_value: string, weekdays: Weekday[]): string | null {
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

export function parseDailyCronExpression(
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

export function toIntervalSeconds(value: string, unit: EveryUnit): number | null {
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

export function formatSessionLabel(title: string, agent_name: string): string {
  return `${title} · ${agent_name}`;
}

export function buildRoomSessionSelections(
  contexts: RoomContextAggregate[],
  agent_name_by_id: Map<string, string>,
): RoomSessionSelection[] {
  return contexts.flatMap((context) => {
    const room_title = context.conversation.title?.trim() || context.room.name?.trim() || "未命名会话";
    const room_type = context.room.room_type;

    return context.sessions.map((session) => ({
      session_key: buildRoomAgentSessionKey(
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
      label: formatSessionLabel(room_title, agent_name_by_id.get(session.agent_id) || session.agent_id),
    }));
  });
}
