/**
 * =====================================================
 * @File   : scheduled-task-dialog-types.ts
 * @Date   : 2026-04-16 13:44
 * @Author : leemysw
 * 2026-04-16 13:44   Create
 * =====================================================
 */

"use client";

import type { ScheduledTaskExecutionKind, ScheduledTaskSchedule } from "@/types/capability/scheduled-task";

import type { Weekday } from "../pickers/picker-types";

export type ScheduleKind = ScheduledTaskSchedule["kind"];
export type EveryUnit = "seconds" | "minutes" | "hours";
export type TargetType = "agent" | "room";
export type ExecutionKind = ScheduledTaskExecutionKind;
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
  value: string;
  sessionKey: string;
  agentId: string;
  label: string;
}

export interface ScheduledTaskDialogScheduleSnapshot {
  scheduleKind: ScheduleKind;
  everyValue?: string;
  everyUnit?: EveryUnit;
  dailyTime?: string;
  selectedWeekdays?: Weekday[];
  runAt?: string;
}

export interface ScheduledTaskDialogInitialState {
  taskName: string;
  targetType: TargetType;
  executionKind: ExecutionKind;
  selectedAgentId: string;
  selectedRoomId: string;
  executionMode: ExecutionMode;
  selectedSessionKey: string;
  replyMode: ReplyMode;
  selectedReplySessionKey: string;
  dedicatedSessionKey: string;
  timezone: string;
  enabled: boolean;
  instruction: string;
  scheduleSnapshot: ScheduledTaskDialogScheduleSnapshot | null;
}
