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
  get_default_timezone,
} from "./scheduled-task-dialog-options";
import {
  isoToZonedLocalInput,
  parse_daily_cron_expression,
} from "./scheduled-task-dialog-time";
import type {
  ScheduledTaskDialogInitialState,
  ScheduledTaskDialogScheduleSnapshot,
} from "./scheduled-task-dialog-types";

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
  agent_id: string,
  task: ScheduledTaskItem,
): ScheduledTaskDialogInitialState {
  const source_context_type = task.source?.context_type === "room" ? "room" : "agent";
  const execution_delivery_target = task.session_target.kind === "bound"
    ? task.session_target.bound_session_key
    : source_context_type === "room"
      ? (task.source?.session_key || "")
      : "";

  return {
    task_name: task.name,
    target_type: source_context_type,
    selected_agent_id: source_context_type === "agent"
      ? (task.source?.context_id || task.agent_id)
      : task.agent_id,
    selected_room_id: source_context_type === "room" ? (task.source?.context_id || "") : "",
    execution_mode: task.session_target.kind === "main"
      ? "main"
      : task.session_target.kind === "named"
        ? "dedicated"
        : task.session_target.kind === "isolated"
          ? "temporary"
          : "existing",
    selected_session_key: task.session_target.kind === "bound"
      ? task.session_target.bound_session_key
      : source_context_type === "room"
        ? (task.source?.session_key || "")
        : "",
    reply_mode: task.delivery.mode === "none"
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
      ? task.delivery.to
      : "",
    dedicated_session_key: task.session_target.kind === "named" ? task.session_target.named_session_key : "",
    timezone: task.schedule.timezone?.trim() || get_default_timezone(),
    enabled: task.enabled,
    instruction: task.instruction,
    schedule_snapshot: build_task_schedule_snapshot(task),
  };
}
