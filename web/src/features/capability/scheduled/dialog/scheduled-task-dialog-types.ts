import type { RefObject } from "react";

import type {
  ScheduledTaskPermissionMode,
  ScheduledTaskSchedule,
} from "@/types/capability/scheduled-task/task";

import type { Weekday } from "../pickers/picker-types";

export type ScheduleKind = ScheduledTaskSchedule["kind"] | "custom" | "monthly";
export type EveryUnit = "hours" | "minutes" | "seconds";
export type TargetType = "agent" | "room";
export type DeliveryTargetType = TargetType;
// 页面仅创建 Agent 任务；script 只用于识别并只读展示历史任务。
export type ExecutionKind = "agent" | "script";
export type ExecutionMode = "dedicated" | "existing" | "main" | "temporary";
export type ReplyMode = "none" | "selected";
export type PermissionMode = ScheduledTaskPermissionMode | "copy";

export interface ChoiceDef<Value extends string> {
  key: Value;
  label: string;
}

export interface TaskDialogLabelOption {
  badge?: string | null;
  label: string;
  value: string;
}

export interface TaskDialogSessionOption extends TaskDialogLabelOption {
  sessionKey: string;
}

export interface TaskFormDraft {
  dedicatedSessionKey: string;
  deliveryTargetType: DeliveryTargetType;
  enabled: boolean;
  expiresAt: string;
  executionKind: ExecutionKind;
  executionMode: ExecutionMode;
  instruction: string;
  permissionMode: PermissionMode;
  replyMode: ReplyMode;
  selectedAgentId: string;
  selectedDeliveryAgentId: string;
  selectedDeliveryPresenterAgentId: string;
  selectedDeliveryRoomId: string;
  selectedReplySessionKey: string;
  selectedRoomId: string;
  selectedSessionKey: string;
  targetType: TargetType;
  taskName: string;
}

export interface TaskScheduleDraft {
  cronExpression: string;
  dailyTime: string;
  everyUnit: EveryUnit;
  everyValue: string;
  kind: ScheduleKind;
  monthlyDay: string;
  runAt: string;
  selectedWeekdays: Weekday[];
  timezone: string;
}

export interface TaskDialogInitialState {
  form: TaskFormDraft;
  schedule: TaskScheduleDraft;
}

export interface TaskDialogCreatePreset {
  dailyTime: string;
  instruction: string;
  selectedWeekdays: Weekday[];
  taskName: string;
}

export interface TaskDialogRefs {
  dailyPickerAnchorRef: RefObject<HTMLButtonElement | null>;
  nameRef: RefObject<HTMLInputElement | null>;
  singlePickerAnchorRef: RefObject<HTMLButtonElement | null>;
}
