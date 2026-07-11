import {
  buildRoomSharedSessionKey,
  parseSessionKey,
} from "@/lib/conversation/session-key";
import type { ScheduledTaskItem } from "@/types/capability/scheduled-task";

import type {
  ExecutionMode,
  ReplyMode,
  TaskDialogInitialState,
  TaskFormDraft,
  TaskScheduleDraft,
  TargetType,
} from "../scheduled-task-dialog-types";
import {
  createDefaultTaskSchedule,
  getDefaultTimezone,
} from "../schedule/task-schedule-model";
import {
  buildRoomExecutorSelectionKey,
  isoToZonedLocalInput,
  parseDailyCronExpression,
} from "../schedule/task-schedule-time";

const SESSION_TARGET_MODES: Record<
  ScheduledTaskItem["session_target"]["kind"],
  ExecutionMode
> = {
  bound: "existing",
  isolated: "temporary",
  main: "main",
  named: "dedicated",
};

function buildRoomExecutorSelectionFromSessionKey(
  sessionKey: string,
  agentId: string,
): string {
  const parsed = parseSessionKey(sessionKey);
  let sharedSessionKey = sessionKey;
  if (parsed.kind === "agent" && parsed.ref) {
    sharedSessionKey = buildRoomSharedSessionKey(parsed.ref);
  }
  if (!sharedSessionKey.trim() || !agentId.trim()) {
    return "";
  }
  return buildRoomExecutorSelectionKey(sharedSessionKey, agentId);
}

function executionSessionKey(task: ScheduledTaskItem): string {
  if (task.session_target.kind === "bound") {
    return task.session_target.bound_session_key;
  }
  return task.source?.session_key || "";
}

function buildRoomTaskExecutorSelectionKey(task: ScheduledTaskItem): string {
  return buildRoomExecutorSelectionFromSessionKey(
    executionSessionKey(task),
    task.agent_id,
  );
}

function resolveReplyMode(
  task: ScheduledTaskItem,
  executionTarget: string,
): ReplyMode {
  if (task.execution_kind === "script" || task.delivery.mode === "none") {
    return "none";
  }
  if (task.delivery.mode === "explicit"
    && (!executionTarget || task.delivery.to !== executionTarget)) {
    return "selected";
  }
  return "execution";
}

function buildTaskSchedule(task: ScheduledTaskItem): TaskScheduleDraft {
  const timezone = task.schedule.timezone?.trim() || getDefaultTimezone();
  const defaults = createDefaultTaskSchedule(new Date(), timezone);
  if (task.schedule.kind === "cron") {
    const parsed = parseDailyCronExpression(task.schedule.cron_expression);
    return {
      ...defaults,
      dailyTime: parsed?.dailyTime ?? defaults.dailyTime,
      kind: "cron",
      selectedWeekdays: parsed?.selectedWeekdays ?? defaults.selectedWeekdays,
    };
  }
  if (task.schedule.kind === "at") {
    return {
      ...defaults,
      kind: "at",
      runAt: isoToZonedLocalInput(task.schedule.run_at, timezone)
        || task.schedule.run_at.replace("Z", "").slice(0, 19),
    };
  }
  const interval = intervalDisplay(task.schedule.interval_seconds);
  return {
    ...defaults,
    everyUnit: interval.unit,
    everyValue: interval.value,
    kind: "every",
  };
}

function intervalDisplay(intervalSeconds: number): {
  unit: TaskScheduleDraft["everyUnit"];
  value: string;
} {
  const rules: Array<{
    divisor: number;
    unit: TaskScheduleDraft["everyUnit"];
  }> = [
    { divisor: 3600, unit: "hours" },
    { divisor: 60, unit: "minutes" },
    { divisor: 1, unit: "seconds" },
  ];
  const rule = rules.find(({ divisor }) => intervalSeconds % divisor === 0)
    ?? rules[rules.length - 1];
  return {
    unit: rule.unit,
    value: String(intervalSeconds / rule.divisor),
  };
}

export function buildDefaultTaskDialogInitialState(
  agentId: string,
): TaskDialogInitialState {
  return {
    form: {
      dedicatedSessionKey: "",
      enabled: true,
      expiresAt: "",
      executionKind: "agent",
      executionMode: "temporary",
      instruction: "",
      replyMode: "none",
      selectedAgentId: agentId,
      selectedReplySessionKey: "",
      selectedRoomId: "",
      selectedSessionKey: "",
      targetType: "agent",
      taskName: "",
    },
    schedule: createDefaultTaskSchedule(),
  };
}

export function buildTaskDialogInitialState(
  task: ScheduledTaskItem,
): TaskDialogInitialState {
  const schedule = buildTaskSchedule(task);
  const executionKind = task.execution_kind === "script" ? "script" : "agent";
  const targetType: TargetType = task.source?.context_type === "room"
    ? "room"
    : "agent";
  const executionTarget = executionSessionKey(task);
  const form: TaskFormDraft = {
    dedicatedSessionKey: task.session_target.kind === "named"
      ? task.session_target.named_session_key
      : "",
    enabled: task.enabled,
    expiresAt: task.expires_at === null
      ? ""
      : isoToZonedLocalInput(
          new Date(task.expires_at).toISOString(),
          schedule.timezone,
        ) ?? "",
    executionKind,
    executionMode: executionKind === "script"
      ? "temporary"
      : targetType === "room"
        ? "existing"
        : SESSION_TARGET_MODES[task.session_target.kind],
    instruction: task.instruction,
    replyMode: resolveReplyMode(task, executionTarget),
    selectedAgentId: executionKind === "script"
      ? task.agent_id
      : targetType === "agent"
        ? task.source?.context_id || task.agent_id
        : task.agent_id,
    selectedReplySessionKey: selectedReplySessionKey(
      task,
      targetType,
      executionTarget,
    ),
    selectedRoomId: executionKind === "script" || targetType !== "room"
      ? ""
      : task.source?.context_id || "",
    selectedSessionKey: targetType === "room"
      ? buildRoomTaskExecutorSelectionKey(task)
      : task.session_target.kind === "bound"
        ? task.session_target.bound_session_key
        : "",
    targetType: executionKind === "script" ? "agent" : targetType,
    taskName: task.name,
  };
  return { form, schedule };
}

function selectedReplySessionKey(
  task: ScheduledTaskItem,
  targetType: TargetType,
  executionTarget: string,
): string {
  if (task.delivery.mode !== "explicit"
    || !task.delivery.to
    || task.delivery.to === executionTarget) {
    return "";
  }
  return targetType === "room"
    ? buildRoomExecutorSelectionFromSessionKey(task.delivery.to, task.agent_id)
    : task.delivery.to;
}
