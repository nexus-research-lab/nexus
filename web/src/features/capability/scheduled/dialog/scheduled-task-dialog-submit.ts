/**
 * =====================================================
 * @File   : scheduled-task-dialog-submit.ts
 * @Date   : 2026-04-16 13:44
 * @Author : leemysw
 * 2026-04-16 13:44   Create
 * =====================================================
 */

"use client";

import type {
  CreateScheduledTaskParams,
  ScheduledTaskDeliveryTarget,
  ScheduledTaskSchedule,
  ScheduledTaskSessionTarget,
  ScheduledTaskSourceKind,
  ScheduledTaskSource,
} from "@/types/capability/scheduled-task";

import {
  buildDailyCronExpression,
  toIntervalSeconds,
  zonedDateTimeToEpochMs,
} from "./scheduled-task-dialog-time";
import type {
  EveryUnit,
  ExecutionKind,
  ExecutionMode,
  ReplyMode,
  ScheduledTaskDialogLabelOption,
  ScheduledTaskDialogSessionOption,
  TargetType,
} from "./scheduled-task-dialog-types";
import type { Weekday } from "../pickers/picker-types";

export interface ScheduledTaskDialogSubmitState {
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
  everyValue: string;
  everyUnit: EveryUnit;
  dailyTime: string;
  selectedWeekdays: Weekday[];
  runAt: string;
  selectedSession: ScheduledTaskDialogSessionOption | null;
  selectedReplySession: ScheduledTaskDialogSessionOption | null;
  agentOptions: ScheduledTaskDialogLabelOption[];
  roomOptions: ScheduledTaskDialogLabelOption[];
  scheduleKind: ScheduledTaskSchedule["kind"];
}

function buildSessionTarget(state: ScheduledTaskDialogSubmitState): ScheduledTaskSessionTarget {
  if (state.targetType === "room") {
    if (!state.selectedSession) {
      throw new Error("请选择执行成员");
    }
    return {
      kind: "bound",
      bound_session_key: state.selectedSession.sessionKey,
      wake_mode: "next-heartbeat",
    };
  }
  if (state.executionMode === "main") {
    return { kind: "main", wake_mode: "next-heartbeat" };
  }
  if (state.executionMode === "temporary") {
    return { kind: "isolated", wake_mode: "next-heartbeat" };
  }
  if (state.executionMode === "dedicated") {
    return { kind: "named", named_session_key: state.dedicatedSessionKey.trim(), wake_mode: "next-heartbeat" };
  }
  if (!state.selectedSession) {
    throw new Error("请选择执行会话");
  }
  return {
    kind: "bound",
    bound_session_key: state.selectedSession.sessionKey,
    wake_mode: "next-heartbeat",
  };
}

function buildDelivery(state: ScheduledTaskDialogSubmitState): ScheduledTaskDeliveryTarget {
  if (state.replyMode === "none") {
    return { mode: "none" };
  }
  if (state.replyMode === "execution") {
    if (state.executionMode === "main") {
      return { mode: "none" };
    }
    if (state.executionMode === "existing" || state.targetType === "room") {
      if (!state.selectedSession) {
        throw new Error(state.targetType === "room" ? "请选择执行成员" : "请选择执行会话");
      }
      return { mode: "explicit", channel: "websocket", to: state.selectedSession.sessionKey };
    }
    return { mode: "none" };
  }
  if (!state.selectedReplySession) {
    throw new Error("请选择回复会话");
  }
  return { mode: "explicit", channel: "websocket", to: state.selectedReplySession.sessionKey };
}

function resolveAgentIdForTask(state: ScheduledTaskDialogSubmitState): string {
  if (state.executionKind === "script") {
    return state.selectedAgentId.trim();
  }
  if (state.targetType === "agent") {
    return state.selectedAgentId.trim();
  }
  if (!state.selectedSession) {
    throw new Error("请选择执行成员");
  }
  return state.selectedSession.agentId;
}

function buildSchedule(state: ScheduledTaskDialogSubmitState): ScheduledTaskSchedule {
  const timezone = state.timezone.trim() || "Asia/Shanghai";
  if (state.scheduleKind === "every") {
    const intervalSeconds = toIntervalSeconds(state.everyValue, state.everyUnit);
    if (intervalSeconds === null) {
      throw new Error("循环间隔必须是大于 0 的整数");
    }
    return { kind: "every", interval_seconds: intervalSeconds, timezone };
  }
  if (state.scheduleKind === "cron") {
    const cronExpression = buildDailyCronExpression(state.dailyTime, state.selectedWeekdays);
    if (!cronExpression) {
      throw new Error("请选择有效的固定执行时间");
    }
    return { kind: "cron", cron_expression: cronExpression, timezone };
  }
  return { kind: "at", run_at: state.runAt.trim(), timezone };
}

function buildSourceSnapshot(
  state: ScheduledTaskDialogSubmitState,
  originalSource?: ScheduledTaskSource | null,
): ScheduledTaskSource {
  const selectedAgent = state.agentOptions.find((option) => option.value === state.selectedAgentId);
  const selectedRoom = state.roomOptions.find((option) => option.value === state.selectedRoomId);
  if (state.executionKind === "script") {
    return {
      kind: (originalSource?.kind || "user_page") as ScheduledTaskSourceKind,
      creator_agent_id: originalSource?.creator_agent_id ?? null,
      context_type: "agent",
      context_id: state.selectedAgentId.trim(),
      context_label: selectedAgent?.label || state.selectedAgentId.trim(),
      session_key: null,
      session_label: null,
    };
  }
  return {
    kind: (originalSource?.kind || "user_page") as ScheduledTaskSourceKind,
    creator_agent_id: originalSource?.creator_agent_id ?? null,
    context_type: state.targetType,
    context_id: state.targetType === "agent" ? state.selectedAgentId.trim() : state.selectedRoomId.trim(),
    context_label: state.targetType === "agent"
      ? (selectedAgent?.label || state.selectedAgentId.trim())
      : (selectedRoom?.label || state.selectedRoomId.trim()),
    session_key: state.selectedSession?.sessionKey ?? null,
    session_label: state.selectedSession?.label ?? null,
  };
}

export function getScheduledTaskValidationError(state: ScheduledTaskDialogSubmitState): string | null {
  if (!state.taskName.trim()) {
    return "请输入任务名称";
  }
  if (!state.instruction.trim()) {
    return state.executionKind === "script" ? "请输入脚本内容" : "请输入任务指令";
  }
  if (state.executionKind === "script") {
    if (!state.selectedAgentId.trim()) {
      return "请选择智能体";
    }
  } else if (state.targetType === "agent") {
    if (!state.selectedAgentId.trim()) {
      return "请选择智能体";
    }
  } else if (!state.selectedRoomId.trim()) {
    return "请选择 Room";
  }
  if (state.executionKind !== "script") {
    if (state.targetType === "room" && !state.selectedSessionKey.trim()) {
      return "请选择执行成员";
    }
    if (state.executionMode === "existing" && !state.selectedSessionKey.trim()) {
      return "请选择执行会话";
    }
    if (state.executionMode === "dedicated" && !state.dedicatedSessionKey.trim()) {
      return "请输入专用长期会话名称";
    }
    if (state.replyMode === "selected" && !state.selectedReplySessionKey.trim()) {
      return "请选择回复会话";
    }
  }
  if (state.scheduleKind === "every" && toIntervalSeconds(state.everyValue, state.everyUnit) === null) {
    return "循环间隔必须是大于 0 的整数";
  }
  if (state.scheduleKind === "cron" && !buildDailyCronExpression(state.dailyTime, state.selectedWeekdays)) {
    return state.selectedWeekdays.length === 0 ? "请至少选择一个执行日" : "请选择有效的固定执行时间";
  }
  if (state.scheduleKind === "at") {
    if (!state.runAt.trim()) {
      return "请选择有效的执行时间";
    }
    const runAtEpoch = zonedDateTimeToEpochMs(state.runAt, state.timezone.trim() || "Asia/Shanghai");
    if (runAtEpoch === null) {
      return "请选择有效的执行时间";
    }
    if (runAtEpoch <= Date.now()) {
      return "单次执行时间必须晚于当前时间";
    }
  }
  if (state.executionKind !== "script" && state.executionMode === "main" && state.replyMode !== "none") {
    return "主会话任务暂不支持额外结果回传";
  }
  return null;
}

export function buildScheduledTaskPayload(
  state: ScheduledTaskDialogSubmitState,
  originalSource?: ScheduledTaskSource | null,
): CreateScheduledTaskParams {
  const resolvedAgentId = resolveAgentIdForTask(state);
  if (state.executionKind === "script") {
    return {
      name: state.taskName.trim(),
      schedule: buildSchedule(state),
      instruction: state.instruction.trim(),
      execution_kind: "script",
      session_target: { kind: "isolated", wake_mode: "next-heartbeat" },
      delivery: { mode: "none" },
      source: buildSourceSnapshot(state, originalSource),
      enabled: state.enabled,
      agent_id: resolvedAgentId,
    };
  }
  return {
    name: state.taskName.trim(),
    schedule: buildSchedule(state),
    instruction: state.instruction.trim(),
    execution_kind: "agent",
    session_target: buildSessionTarget(state),
    delivery: buildDelivery(state),
    source: buildSourceSnapshot(state, originalSource),
    enabled: state.enabled,
    agent_id: resolvedAgentId,
  };
}
