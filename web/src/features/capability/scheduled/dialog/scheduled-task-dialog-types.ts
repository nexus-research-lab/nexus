import type { RefObject } from "react";

import type {
  ScheduledTaskExecutionKind,
  ScheduledTaskSchedule,
} from "@/types/capability/scheduled-task/task";

import type { Weekday } from "../pickers/picker-types";

export type ScheduleKind = ScheduledTaskSchedule["kind"];
export type EveryUnit = "hours" | "minutes" | "seconds";
export type TargetType = "agent" | "room";
export type ExecutionKind = ScheduledTaskExecutionKind;
export type ExecutionMode = "dedicated" | "existing" | "main" | "temporary";
export type ReplyMode = "execution" | "none" | "selected";

export interface ChoiceDef<Value extends string> {
  key: Value;
  label: string;
}

export interface TaskDialogLabelOption {
  label: string;
  value: string;
}

export interface TaskDialogSessionOption extends TaskDialogLabelOption {
  agentId: string;
  sessionKey: string;
}

export interface TaskFormDraft {
  dedicatedSessionKey: string;
  enabled: boolean;
  expiresAt: string;
  executionKind: ExecutionKind;
  executionMode: ExecutionMode;
  instruction: string;
  replyMode: ReplyMode;
  selectedAgentId: string;
  selectedReplySessionKey: string;
  selectedRoomId: string;
  selectedSessionKey: string;
  targetType: TargetType;
  taskName: string;
}

export interface TaskScheduleDraft {
  dailyTime: string;
  everyUnit: EveryUnit;
  everyValue: string;
  kind: ScheduleKind;
  runAt: string;
  selectedWeekdays: Weekday[];
  timezone: string;
}

export interface TaskDialogInitialState {
  form: TaskFormDraft;
  schedule: TaskScheduleDraft;
}

export interface TaskDialogRefs {
  dailyPickerAnchorRef: RefObject<HTMLButtonElement | null>;
  nameRef: RefObject<HTMLInputElement | null>;
  singlePickerAnchorRef: RefObject<HTMLButtonElement | null>;
}
