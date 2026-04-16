/**
 * =====================================================
 * @File   : scheduled-task-dialog-options.ts
 * @Date   : 2026-04-16 14:28
 * @Author : leemysw
 * 2026-04-16 14:28   Create
 * =====================================================
 */

"use client";

import type {
  ChoiceDef,
  EveryUnit,
  ExecutionMode,
  ReplyMode,
  ScheduleKind,
  TargetType,
} from "./scheduled-task-dialog-types";

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
