/**
 * =====================================================
 * @File   : scheduled-task-dialog-types.ts
 * @Date   : 2026-04-16 13:44
 * @Author : leemysw
 * 2026-04-16 13:44   Create
 * =====================================================
 */

"use client";

import type { ScheduledTaskSchedule } from "@/types/capability/scheduled-task";

import type { Weekday } from "../pickers/picker-types";

export type ScheduleKind = ScheduledTaskSchedule["kind"];
export type EveryUnit = "seconds" | "minutes" | "hours";
export type TargetType = "agent" | "room";
export type ExecutionMode = "main" | "existing" | "temporary" | "dedicated";
export type ReplyMode = "none" | "execution" | "selected";

export interface ChoiceDef<TValue extends string> {
  key: TValue;
  label: string;
}

export interface ScheduledTaskDialogLabelOption {
  value: string;
  label: string;
}

export interface ScheduledTaskDialogSessionOption {
  session_key: string;
  agent_id: string;
  label: string;
}

export interface ScheduledTaskDialogScheduleSnapshot {
  schedule_kind: ScheduleKind;
  every_value?: string;
  every_unit?: EveryUnit;
  daily_time?: string;
  selected_weekdays?: Weekday[];
  run_at?: string;
}

export interface ScheduledTaskDialogInitialState {
  task_name: string;
  target_type: TargetType;
  selected_agent_id: string;
  selected_room_id: string;
  execution_mode: ExecutionMode;
  selected_session_key: string;
  reply_mode: ReplyMode;
  selected_reply_session_key: string;
  dedicated_session_key: string;
  timezone: string;
  enabled: boolean;
  instruction: string;
  schedule_snapshot: ScheduledTaskDialogScheduleSnapshot | null;
}
