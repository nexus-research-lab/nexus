import {
  buildRoomSharedSessionKey,
  parseSessionKey,
} from "@/lib/conversation/session-key";
import type { ScheduledTaskItem } from "@/types/capability/scheduled-task/task";

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

type TaskExecutionInitialState = Pick<
  TaskFormDraft,
  | "dedicatedSessionKey"
  | "executionKind"
  | "executionMode"
  | "selectedAgentId"
  | "selectedRoomId"
  | "selectedSessionKey"
  | "targetType"
>;

type TaskReplyInitialState = Pick<
  TaskFormDraft,
  "replyMode" | "selectedReplySessionKey"
>;

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

function sourceContextId(task: ScheduledTaskItem): string {
  return task.source.context_id?.trim() || "";
}

function namedSessionKey(task: ScheduledTaskItem): string {
  return task.session_target.kind === "named"
    ? task.session_target.named_session_key
    : "";
}

function boundSessionKey(task: ScheduledTaskItem): string {
  return task.session_target.kind === "bound"
    ? task.session_target.bound_session_key
    : "";
}

function buildAgentTargetInitialState(
  task: ScheduledTaskItem,
): TaskExecutionInitialState {
  return {
    dedicatedSessionKey: namedSessionKey(task),
    executionKind: "agent",
    executionMode: SESSION_TARGET_MODES[task.session_target.kind],
    selectedAgentId: sourceContextId(task) || task.agent_id,
    selectedRoomId: "",
    selectedSessionKey: boundSessionKey(task),
    targetType: "agent",
  };
}

function buildRoomTargetInitialState(
  task: ScheduledTaskItem,
): TaskExecutionInitialState {
  return {
    dedicatedSessionKey: "",
    executionKind: "agent",
    executionMode: "existing",
    selectedAgentId: task.agent_id,
    selectedRoomId: sourceContextId(task),
    selectedSessionKey: buildRoomTaskExecutorSelectionKey(task),
    targetType: "room",
  };
}

const AGENT_TARGET_INITIALIZERS: Record<
  TargetType,
  (task: ScheduledTaskItem) => TaskExecutionInitialState
> = {
  agent: buildAgentTargetInitialState,
  room: buildRoomTargetInitialState,
};

function agentTargetType(task: ScheduledTaskItem): TargetType {
  return task.source.context_type === "room" ? "room" : "agent";
}

function buildAgentExecutionInitialState(
  task: ScheduledTaskItem,
): TaskExecutionInitialState {
  return AGENT_TARGET_INITIALIZERS[agentTargetType(task)](task);
}

function buildScriptExecutionInitialState(
  task: ScheduledTaskItem,
): TaskExecutionInitialState {
  return {
    dedicatedSessionKey: "",
    executionKind: "script",
    executionMode: "temporary",
    selectedAgentId: task.agent_id,
    selectedRoomId: "",
    selectedSessionKey: "",
    targetType: "agent",
  };
}

const EXECUTION_INITIALIZERS: Record<
  TaskFormDraft["executionKind"],
  (task: ScheduledTaskItem) => TaskExecutionInitialState
> = {
  agent: buildAgentExecutionInitialState,
  script: buildScriptExecutionInitialState,
};

function executionKind(task: ScheduledTaskItem): TaskFormDraft["executionKind"] {
  return task.execution_kind === "script" ? "script" : "agent";
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

const REPLY_SESSION_KEY_BUILDERS: Record<
  TargetType,
  (sessionKey: string, agentId: string) => string
> = {
  agent: (sessionKey) => sessionKey,
  room: buildRoomExecutorSelectionFromSessionKey,
};

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
  return REPLY_SESSION_KEY_BUILDERS[targetType](
    task.delivery.to,
    task.agent_id,
  );
}

function buildAgentReplyInitialState(
  task: ScheduledTaskItem,
  execution: TaskExecutionInitialState,
): TaskReplyInitialState {
  const executionTarget = executionSessionKey(task);
  return {
    replyMode: resolveReplyMode(task, executionTarget),
    selectedReplySessionKey: selectedReplySessionKey(
      task,
      execution.targetType,
      executionTarget,
    ),
  };
}

function buildScriptReplyInitialState(): TaskReplyInitialState {
  return {
    replyMode: "none",
    selectedReplySessionKey: "",
  };
}

const REPLY_INITIALIZERS: Record<
  TaskFormDraft["executionKind"],
  (
    task: ScheduledTaskItem,
    execution: TaskExecutionInitialState,
  ) => TaskReplyInitialState
> = {
  agent: buildAgentReplyInitialState,
  script: buildScriptReplyInitialState,
};

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
  const kind = executionKind(task);
  const execution = EXECUTION_INITIALIZERS[kind](task);
  const reply = REPLY_INITIALIZERS[kind](task, execution);
  const form: TaskFormDraft = {
    ...execution,
    ...reply,
    enabled: task.enabled,
    expiresAt: buildExpirationInput(task, schedule.timezone),
    instruction: task.instruction,
    taskName: task.name,
  };
  return { form, schedule };
}

function buildExpirationInput(
  task: ScheduledTaskItem,
  timezone: string,
): string {
  if (task.expires_at === null) {
    return "";
  }
  return isoToZonedLocalInput(
    new Date(task.expires_at).toISOString(),
    timezone,
  ) ?? "";
}
