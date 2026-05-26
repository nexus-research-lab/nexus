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

function build_room_executor_selection_from_session_key(session_key: string, agent_id: string): string {
  const parsed = parse_session_key(session_key);
  const shared_session_key = parsed.kind === "room"
    ? session_key
    : parsed.kind === "agent" && parsed.ref
      ? build_room_shared_session_key(parsed.ref)
      : session_key;
  if (!shared_session_key.trim() || !agent_id.trim()) {
    return "";
  }
  return build_room_executor_selection_key(shared_session_key, agent_id);
}

function build_room_task_executor_selection_key(task: ScheduledTaskItem): string {
  const execution_session_key = task.session_target.kind === "bound"
    ? task.session_target.bound_session_key
    : task.source?.session_key || "";
  return build_room_executor_selection_from_session_key(execution_session_key, task.agent_id);
}

function build_default_schedule_snapshot(): ScheduledTaskDialogScheduleSnapshot {
  return {
    schedule_kind: "every",
    every_value: "30",
    every_unit: "minutes",
  };
}

export function build_default_dialog_initial_state(agent_id: string): ScheduledTaskDialogInitialState {
  return {
    task_name: "",
    target_type: "agent",
    execution_kind: "agent",
    selected_agent_id: agent_id,
    selected_room_id: "",
    execution_mode: "existing",
    selected_session_key: "",
    reply_mode: "execution",
    selected_reply_session_key: "",
    dedicated_session_key: "",
    timezone: get_default_timezone(),
    enabled: true,
    instruction: "",
    schedule_snapshot: build_default_schedule_snapshot(),
  };
}

function build_task_schedule_snapshot(task: ScheduledTaskItem): ScheduledTaskDialogScheduleSnapshot {
  if (task.schedule.kind === "every") {
    const interval_seconds = task.schedule.interval_seconds;
    if (interval_seconds % 3600 === 0) {
      return {
        schedule_kind: "every",
        every_value: String(interval_seconds / 3600),
        every_unit: "hours",
      };
    }
    if (interval_seconds % 60 === 0) {
      return {
        schedule_kind: "every",
        every_value: String(interval_seconds / 60),
        every_unit: "minutes",
      };
    }
    return {
      schedule_kind: "every",
      every_value: String(interval_seconds),
      every_unit: "seconds",
    };
  }

  if (task.schedule.kind === "cron") {
    const parsed_cron = parse_daily_cron_expression(task.schedule.cron_expression);
    return {
      schedule_kind: "cron",
      daily_time: parsed_cron?.daily_time,
      selected_weekdays: parsed_cron?.selected_weekdays,
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
  const source_context_type = task.source?.context_type === "room" ? "room" : "agent";
  const execution_kind = task.execution_kind === "script" ? "script" : "agent";
  const execution_delivery_target = task.session_target.kind === "bound"
    ? task.session_target.bound_session_key
    : source_context_type === "room"
      ? (task.source?.session_key || "")
      : "";

  return {
    task_name: task.name,
    target_type: execution_kind === "script" ? "agent" : source_context_type,
    execution_kind,
    selected_agent_id: execution_kind === "script"
      ? task.agent_id
      : source_context_type === "agent"
      ? (task.source?.context_id || task.agent_id)
      : task.agent_id,
    selected_room_id: execution_kind === "script" ? "" : source_context_type === "room" ? (task.source?.context_id || "") : "",
    execution_mode: task.session_target.kind === "main"
      ? "main"
      : task.session_target.kind === "named"
        ? "dedicated"
        : task.session_target.kind === "isolated"
          ? "temporary"
          : "existing",
    selected_session_key: source_context_type === "room"
      ? build_room_task_executor_selection_key(task)
      : task.session_target.kind === "bound"
        ? task.session_target.bound_session_key
        : "",
    reply_mode: execution_kind === "script"
      ? "none"
      : task.delivery.mode === "none"
      ? "none"
      : task.delivery.mode === "explicit"
        && task.delivery.to
        && execution_delivery_target
        && task.delivery.to !== execution_delivery_target
        ? "selected"
        : task.delivery.mode === "explicit" && !execution_delivery_target
          ? "selected"
          : "execution",
    selected_reply_session_key: task.delivery.mode === "explicit"
      && task.delivery.to
      && task.delivery.to !== execution_delivery_target
      ? source_context_type === "room"
        ? build_room_executor_selection_from_session_key(task.delivery.to, task.agent_id)
        : task.delivery.to
      : "",
    dedicated_session_key: task.session_target.kind === "named" ? task.session_target.named_session_key : "",
    timezone: task.schedule.timezone?.trim() || get_default_timezone(),
    enabled: task.enabled,
    instruction: task.instruction,
    schedule_snapshot: build_task_schedule_snapshot(task),
  };
}
