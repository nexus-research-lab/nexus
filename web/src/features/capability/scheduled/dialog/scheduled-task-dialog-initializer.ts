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
  buildRoomSharedSessionKey,
  parseSessionKey,
} from "@/lib/conversation/session-key";

import {
  getDefaultTimezone,
} from "./scheduled-task-dialog-options";
import {
  buildRoomExecutorSelectionKey,
  isoToZonedLocalInput,
  parseDailyCronExpression,
} from "./scheduled-task-dialog-time";
import type {
  ScheduledTaskDialogInitialState,
  ScheduledTaskDialogScheduleSnapshot,
} from "./scheduled-task-dialog-types";

function buildRoomExecutorSelectionFromSessionKey(sessionKey: string, agentId: string): string {
  const parsed = parseSessionKey(sessionKey);
  const sharedSessionKey = parsed.kind === "room"
    ? sessionKey
    : parsed.kind === "agent" && parsed.ref
      ? buildRoomSharedSessionKey(parsed.ref)
      : sessionKey;
  if (!sharedSessionKey.trim() || !agentId.trim()) {
    return "";
  }
  return buildRoomExecutorSelectionKey(sharedSessionKey, agentId);
}

function buildRoomTaskExecutorSelectionKey(task: ScheduledTaskItem): string {
  const executionSessionKey = task.session_target.kind === "bound"
    ? task.session_target.bound_session_key
    : task.source?.session_key || "";
  return buildRoomExecutorSelectionFromSessionKey(executionSessionKey, task.agent_id);
}

function buildDefaultScheduleSnapshot(): ScheduledTaskDialogScheduleSnapshot {
  return {
    scheduleKind: "every",
    everyValue: "30",
    everyUnit: "minutes",
  };
}

export function buildDefaultDialogInitialState(agentId: string): ScheduledTaskDialogInitialState {
  return {
    taskName: "",
    targetType: "agent",
    executionKind: "agent",
    selectedAgentId: agentId,
    selectedRoomId: "",
    executionMode: "existing",
    selectedSessionKey: "",
    replyMode: "execution",
    selectedReplySessionKey: "",
    dedicatedSessionKey: "",
    timezone: getDefaultTimezone(),
    enabled: true,
    instruction: "",
    scheduleSnapshot: buildDefaultScheduleSnapshot(),
  };
}

function buildTaskScheduleSnapshot(task: ScheduledTaskItem): ScheduledTaskDialogScheduleSnapshot {
  if (task.schedule.kind === "every") {
    const intervalSeconds = task.schedule.interval_seconds;
    if (intervalSeconds % 3600 === 0) {
      return {
        scheduleKind: "every",
        everyValue: String(intervalSeconds / 3600),
        everyUnit: "hours",
      };
    }
    if (intervalSeconds % 60 === 0) {
      return {
        scheduleKind: "every",
        everyValue: String(intervalSeconds / 60),
        everyUnit: "minutes",
      };
    }
    return {
      scheduleKind: "every",
      everyValue: String(intervalSeconds),
      everyUnit: "seconds",
    };
  }

  if (task.schedule.kind === "cron") {
    const parsedCron = parseDailyCronExpression(task.schedule.cron_expression);
    return {
      scheduleKind: "cron",
      dailyTime: parsedCron?.dailyTime,
      selectedWeekdays: parsedCron?.selectedWeekdays,
    };
  }

  const timezone = task.schedule.timezone?.trim() || getDefaultTimezone();
  return {
    scheduleKind: "at",
    runAt: isoToZonedLocalInput(task.schedule.run_at, timezone)
      || task.schedule.run_at.replace("Z", "").slice(0, 19),
  };
}

export function buildTaskDialogInitialState(
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
    taskName: task.name,
    targetType: executionKind === "script" ? "agent" : sourceContextType,
    executionKind: executionKind,
    selectedAgentId: executionKind === "script"
      ? task.agent_id
      : sourceContextType === "agent"
      ? (task.source?.context_id || task.agent_id)
      : task.agent_id,
    selectedRoomId: executionKind === "script" ? "" : sourceContextType === "room" ? (task.source?.context_id || "") : "",
    executionMode: task.session_target.kind === "main"
      ? "main"
      : task.session_target.kind === "named"
        ? "dedicated"
        : task.session_target.kind === "isolated"
          ? "temporary"
          : "existing",
    selectedSessionKey: sourceContextType === "room"
      ? buildRoomTaskExecutorSelectionKey(task)
      : task.session_target.kind === "bound"
        ? task.session_target.bound_session_key
        : "",
    replyMode: executionKind === "script"
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
    selectedReplySessionKey: task.delivery.mode === "explicit"
      && task.delivery.to
      && task.delivery.to !== executionDeliveryTarget
      ? sourceContextType === "room"
        ? buildRoomExecutorSelectionFromSessionKey(task.delivery.to, task.agent_id)
        : task.delivery.to
      : "",
    dedicatedSessionKey: task.session_target.kind === "named" ? task.session_target.named_session_key : "",
    timezone: task.schedule.timezone?.trim() || getDefaultTimezone(),
    enabled: task.enabled,
    instruction: task.instruction,
    scheduleSnapshot: buildTaskScheduleSnapshot(task),
  };
}
