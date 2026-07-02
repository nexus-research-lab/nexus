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
  build_daily_cron_expression,
  to_interval_seconds,
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
  task_name: string;
  target_type: TargetType;
  execution_kind: ExecutionKind;
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
  every_value: string;
  every_unit: EveryUnit;
  daily_time: string;
  selected_weekdays: Weekday[];
  run_at: string;
  selected_session: ScheduledTaskDialogSessionOption | null;
  selected_reply_session: ScheduledTaskDialogSessionOption | null;
  agent_options: ScheduledTaskDialogLabelOption[];
  room_options: ScheduledTaskDialogLabelOption[];
  schedule_kind: ScheduledTaskSchedule["kind"];
}

function buildSessionTarget(state: ScheduledTaskDialogSubmitState): ScheduledTaskSessionTarget {
  if (state.target_type === "room") {
    if (!state.selected_session) {
      throw new Error("请选择执行成员");
    }
    return {
      kind: "bound",
      bound_session_key: state.selected_session.session_key,
      wake_mode: "next-heartbeat",
    };
  }
  if (state.execution_mode === "main") {
    return { kind: "main", wake_mode: "next-heartbeat" };
  }
  if (state.execution_mode === "temporary") {
    return { kind: "isolated", wake_mode: "next-heartbeat" };
  }
  if (state.execution_mode === "dedicated") {
    return { kind: "named", named_session_key: state.dedicated_session_key.trim(), wake_mode: "next-heartbeat" };
  }
  if (!state.selected_session) {
    throw new Error("请选择执行会话");
  }
  return {
    kind: "bound",
    bound_session_key: state.selected_session.session_key,
    wake_mode: "next-heartbeat",
  };
}

function buildDelivery(state: ScheduledTaskDialogSubmitState): ScheduledTaskDeliveryTarget {
  if (state.reply_mode === "none") {
    return { mode: "none" };
  }
  if (state.reply_mode === "execution") {
    if (state.execution_mode === "main") {
      return { mode: "none" };
    }
    if (state.execution_mode === "existing" || state.target_type === "room") {
      if (!state.selected_session) {
        throw new Error(state.target_type === "room" ? "请选择执行成员" : "请选择执行会话");
      }
      return { mode: "explicit", channel: "websocket", to: state.selected_session.session_key };
    }
    return { mode: "none" };
  }
  if (!state.selected_reply_session) {
    throw new Error("请选择回复会话");
  }
  return { mode: "explicit", channel: "websocket", to: state.selected_reply_session.session_key };
}

function resolveAgentIdForTask(state: ScheduledTaskDialogSubmitState): string {
  if (state.execution_kind === "script") {
    return state.selected_agent_id.trim();
  }
  if (state.target_type === "agent") {
    return state.selected_agent_id.trim();
  }
  if (!state.selected_session) {
    throw new Error("请选择执行成员");
  }
  return state.selected_session.agent_id;
}

function buildSchedule(state: ScheduledTaskDialogSubmitState): ScheduledTaskSchedule {
  const timezone = state.timezone.trim() || "Asia/Shanghai";
  if (state.schedule_kind === "every") {
    const intervalSeconds = to_interval_seconds(state.every_value, state.every_unit);
    if (intervalSeconds === null) {
      throw new Error("循环间隔必须是大于 0 的整数");
    }
    return { kind: "every", interval_seconds: intervalSeconds, timezone };
  }
  if (state.schedule_kind === "cron") {
    const cronExpression = build_daily_cron_expression(state.daily_time, state.selected_weekdays);
    if (!cronExpression) {
      throw new Error("请选择有效的固定执行时间");
    }
    return { kind: "cron", cron_expression: cronExpression, timezone };
  }
  return { kind: "at", run_at: state.run_at.trim(), timezone };
}

function buildSourceSnapshot(
  state: ScheduledTaskDialogSubmitState,
  originalSource?: ScheduledTaskSource | null,
): ScheduledTaskSource {
  const selectedAgent = state.agent_options.find((option) => option.value === state.selected_agent_id);
  const selectedRoom = state.room_options.find((option) => option.value === state.selected_room_id);
  if (state.execution_kind === "script") {
    return {
      kind: (originalSource?.kind || "user_page") as ScheduledTaskSourceKind,
      creator_agent_id: originalSource?.creator_agent_id ?? null,
      context_type: "agent",
      context_id: state.selected_agent_id.trim(),
      context_label: selectedAgent?.label || state.selected_agent_id.trim(),
      session_key: null,
      session_label: null,
    };
  }
  return {
    kind: (originalSource?.kind || "user_page") as ScheduledTaskSourceKind,
    creator_agent_id: originalSource?.creator_agent_id ?? null,
    context_type: state.target_type,
    context_id: state.target_type === "agent" ? state.selected_agent_id.trim() : state.selected_room_id.trim(),
    context_label: state.target_type === "agent"
      ? (selectedAgent?.label || state.selected_agent_id.trim())
      : (selectedRoom?.label || state.selected_room_id.trim()),
    session_key: state.selected_session?.session_key ?? null,
    session_label: state.selected_session?.label ?? null,
  };
}

export function get_scheduled_task_validation_error(state: ScheduledTaskDialogSubmitState): string | null {
  if (!state.task_name.trim()) {
    return "请输入任务名称";
  }
  if (!state.instruction.trim()) {
    return state.execution_kind === "script" ? "请输入脚本内容" : "请输入任务指令";
  }
  if (state.execution_kind === "script") {
    if (!state.selected_agent_id.trim()) {
      return "请选择智能体";
    }
  } else if (state.target_type === "agent") {
    if (!state.selected_agent_id.trim()) {
      return "请选择智能体";
    }
  } else if (!state.selected_room_id.trim()) {
    return "请选择 Room";
  }
  if (state.execution_kind !== "script") {
    if (state.target_type === "room" && !state.selected_session_key.trim()) {
      return "请选择执行成员";
    }
    if (state.execution_mode === "existing" && !state.selected_session_key.trim()) {
      return "请选择执行会话";
    }
    if (state.execution_mode === "dedicated" && !state.dedicated_session_key.trim()) {
      return "请输入专用长期会话名称";
    }
    if (state.reply_mode === "selected" && !state.selected_reply_session_key.trim()) {
      return "请选择回复会话";
    }
  }
  if (state.schedule_kind === "every" && to_interval_seconds(state.every_value, state.every_unit) === null) {
    return "循环间隔必须是大于 0 的整数";
  }
  if (state.schedule_kind === "cron" && !build_daily_cron_expression(state.daily_time, state.selected_weekdays)) {
    return state.selected_weekdays.length === 0 ? "请至少选择一个执行日" : "请选择有效的固定执行时间";
  }
  if (state.schedule_kind === "at") {
    if (!state.run_at.trim()) {
      return "请选择有效的执行时间";
    }
    const runAtEpoch = zonedDateTimeToEpochMs(state.run_at, state.timezone.trim() || "Asia/Shanghai");
    if (runAtEpoch === null) {
      return "请选择有效的执行时间";
    }
    if (runAtEpoch <= Date.now()) {
      return "单次执行时间必须晚于当前时间";
    }
  }
  if (state.execution_kind !== "script" && state.execution_mode === "main" && state.reply_mode !== "none") {
    return "主会话任务暂不支持额外结果回传";
  }
  return null;
}

export function build_scheduled_task_payload(
  state: ScheduledTaskDialogSubmitState,
  originalSource?: ScheduledTaskSource | null,
): CreateScheduledTaskParams {
  const resolvedAgentId = resolveAgentIdForTask(state);
  if (state.execution_kind === "script") {
    return {
      name: state.task_name.trim(),
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
    name: state.task_name.trim(),
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
