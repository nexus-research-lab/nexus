/**
 * =====================================================
 * @File   : scheduled-task-dialog-initializer.ts
 * @Date   : 2026-04-16 13:44
 * @Author : leemysw
 * 2026-04-16 13:44   Create
 * =====================================================
 */

"use client";

import type { ScheduledTaskItem } from "@/types/capability/scheduled-task";
import {
  build_room_shared_session_key,
  parse_session_key,
} from "@/lib/conversation/session-key";

import {
  get_default_timezone,
} from "./scheduled-task-dialog-options";
import {
  build_room_executor_selection_key,
  isoToZonedLocalInput,
  parse_daily_cron_expression,
} from "./scheduled-task-dialog-time";
import type {
  ScheduledTaskDialogInitialState,
  ScheduledTaskDialogScheduleSnapshot,
} from "./scheduled-task-dialog-types";

function buildRoomExecutorSelectionFromSessionKey(sessionKey: string, agentId: string): string {
  const parsed = parse_session_key(sessionKey);
  const sharedSessionKey = parsed.kind === "room"
    ? sessionKey
    : parsed.kind === "agent" && parsed.ref
      ? build_room_shared_session_key(parsed.ref)
      : sessionKey;
  if (!sharedSessionKey.trim() || !agentId.trim()) {
    return "";
  }
  return build_room_executor_selection_key(sharedSessionKey, agentId);
}

function buildRoomTaskExecutorSelectionKey(task: ScheduledTaskItem): string {
  const executionSessionKey = task.session_target.kind === "bound"
    ? task.session_target.bound_session_key
    : task.source?.session_key || "";
  return buildRoomExecutorSelectionFromSessionKey(executionSessionKey, task.agent_id);
}

function buildDefaultScheduleSnapshot(): ScheduledTaskDialogScheduleSnapshot {
  return {
    schedule_kind: "every",
    every_value: "30",
    every_unit: "minutes",
  };
}

export function build_default_dialog_initial_state(agentId: string): ScheduledTaskDialogInitialState {
  return {
    task_name: "",
    target_type: "agent",
    execution_kind: "agent",
    selected_agent_id: agentId,
    selected_room_id: "",
    execution_mode: "existing",
    selected_session_key: "",
    reply_mode: "execution",
    selected_reply_session_key: "",
    dedicated_session_key: "",
    timezone: get_default_timezone(),
    enabled: true,
    instruction: "",
    schedule_snapshot: buildDefaultScheduleSnapshot(),
  };
}

function buildTaskScheduleSnapshot(task: ScheduledTaskItem): ScheduledTaskDialogScheduleSnapshot {
  if (task.schedule.kind === "every") {
    const intervalSeconds = task.schedule.interval_seconds;
    if (intervalSeconds % 3600 === 0) {
      return {
        schedule_kind: "every",
        every_value: String(intervalSeconds / 3600),
        every_unit: "hours",
      };
    }
    if (intervalSeconds % 60 === 0) {
      return {
        schedule_kind: "every",
        every_value: String(intervalSeconds / 60),
        every_unit: "minutes",
      };
    }
    return {
      schedule_kind: "every",
      every_value: String(intervalSeconds),
      every_unit: "seconds",
    };
  }

  if (task.schedule.kind === "cron") {
    const parsedCron = parse_daily_cron_expression(task.schedule.cron_expression);
    return {
      schedule_kind: "cron",
      daily_time: parsedCron?.daily_time,
      selected_weekdays: parsedCron?.selected_weekdays,
    };
  }

  const timezone = task.schedule.timezone?.trim() || get_default_timezone();
  return {
    schedule_kind: "at",
    run_at: isoToZonedLocalInput(task.schedule.run_at, timezone)
      || task.schedule.run_at.replace("Z", "").slice(0, 19),
  };
}

export function build_task_dialog_initial_state(
  task: ScheduledTaskItem,
): ScheduledTaskDialogInitialState {
  const sourceContextType = task.source?.context_type === "room" ? "room" : "agent";
  const executionKind = task.execution_kind === "script" ? "script" : "agent";
  const executionDeliveryTarget = task.session_target.kind === "bound"
    ? task.session_target.bound_session_key
    : sourceContextType === "room"
      ? (task.source?.session_key || "")
      : "";

  return {
    task_name: task.name,
    target_type: executionKind === "script" ? "agent" : sourceContextType,
    execution_kind: executionKind,
    selected_agent_id: executionKind === "script"
      ? task.agent_id
      : sourceContextType === "agent"
      ? (task.source?.context_id || task.agent_id)
      : task.agent_id,
    selected_room_id: executionKind === "script" ? "" : sourceContextType === "room" ? (task.source?.context_id || "") : "",
    execution_mode: task.session_target.kind === "main"
      ? "main"
      : task.session_target.kind === "named"
        ? "dedicated"
        : task.session_target.kind === "isolated"
          ? "temporary"
          : "existing",
    selected_session_key: sourceContextType === "room"
      ? buildRoomTaskExecutorSelectionKey(task)
      : task.session_target.kind === "bound"
        ? task.session_target.bound_session_key
        : "",
    reply_mode: executionKind === "script"
      ? "none"
      : task.delivery.mode === "none"
      ? "none"
      : task.delivery.mode === "explicit"
        && task.delivery.to
        && executionDeliveryTarget
        && task.delivery.to !== executionDeliveryTarget
        ? "selected"
        : task.delivery.mode === "explicit" && !executionDeliveryTarget
          ? "selected"
          : "execution",
    selected_reply_session_key: task.delivery.mode === "explicit"
      && task.delivery.to
      && task.delivery.to !== executionDeliveryTarget
      ? sourceContextType === "room"
        ? buildRoomExecutorSelectionFromSessionKey(task.delivery.to, task.agent_id)
        : task.delivery.to
      : "",
    dedicated_session_key: task.session_target.kind === "named" ? task.session_target.named_session_key : "",
    timezone: task.schedule.timezone?.trim() || get_default_timezone(),
    enabled: task.enabled,
    instruction: task.instruction,
    schedule_snapshot: buildTaskScheduleSnapshot(task),
  };
}
