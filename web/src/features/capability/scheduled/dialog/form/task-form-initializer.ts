import {
  buildRoomSharedSessionKey,
  parseSessionKey,
} from "@/lib/conversation/session-key";
import type { ScheduledTaskItem } from "@/types/capability/scheduled-task/task";

import type {
  ExecutionMode,
  ReplyMode,
  TaskDialogCreatePreset,
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
  isoToZonedLocalInput,
  parseDailyCronExpression,
  parseMonthlyCronExpression,
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
  | "deliveryTargetType"
  | "replyMode"
  | "selectedDeliveryAgentId"
  | "selectedDeliveryPresenterAgentId"
  | "selectedDeliveryRoomId"
  | "selectedReplySessionKey"
>;

function buildRoomSharedSelectionFromSessionKey(sessionKey: string): string {
  const parsed = parseSessionKey(sessionKey);
  if (parsed.kind === "agent" && parsed.ref) {
    return buildRoomSharedSessionKey(parsed.ref);
  }
  return parsed.kind === "room" ? sessionKey : "";
}

function executionSessionKey(task: ScheduledTaskItem): string {
  if (task.session_target.kind === "bound") {
    return task.session_target.bound_session_key;
  }
  return "";
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
    // Source 记录的是不可变的创建 provenance；任务后续重绑 Agent 时必须以
    // 当前 task.agent_id 回显，否则再次保存会把配置悄悄切回创建者。
    selectedAgentId: task.agent_id,
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
    selectedRoomId: "",
    selectedSessionKey: buildRoomSharedSelectionFromSessionKey(
      executionSessionKey(task),
    ),
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
  const parsed = parseSessionKey(executionSessionKey(task));
  return parsed.kind === "room" ? "room" : "agent";
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
  _executionTarget: string,
): ReplyMode {
  if (task.execution_kind === "script" || task.delivery.mode === "none") {
    return "none";
  }
  return "selected";
}

function rawDeliverySessionKey(task: ScheduledTaskItem): string {
  const sessionKey = task.delivery.session_key?.trim() || "";
  if (sessionKey) {
    return sessionKey;
  }
  if (task.delivery.mode === "explicit") {
    return task.delivery.to?.trim() || "";
  }
  return "";
}

function isLegacyAutomationInboxSessionKey(sessionKey: string): boolean {
  const parsed = parseSessionKey(sessionKey);
  return parsed.kind === "agent"
    && parsed.channel === "internal"
    && parsed.ref === "automation-inbox";
}

function deliverySessionKey(task: ScheduledTaskItem): string {
  const sessionKey = rawDeliverySessionKey(task);
  const parsed = parseSessionKey(sessionKey);
  return parsed.is_structured && !isLegacyAutomationInboxSessionKey(sessionKey)
    ? sessionKey
    : "";
}

function selectedReplySessionKey(
  task: ScheduledTaskItem,
  _executionTarget: string,
): string {
  return deliverySessionKey(task);
}

function deliveryTargetInitialState(
  task: ScheduledTaskItem,
): Pick<
  TaskReplyInitialState,
  | "deliveryTargetType"
  | "selectedDeliveryAgentId"
  | "selectedDeliveryPresenterAgentId"
  | "selectedDeliveryRoomId"
> {
  const parsed = parseSessionKey(rawDeliverySessionKey(task));
  if (parsed.kind === "room") {
    return {
      deliveryTargetType: "room",
      selectedDeliveryAgentId: "",
      selectedDeliveryPresenterAgentId: task.delivery.agent_id?.trim() || "",
      selectedDeliveryRoomId: "",
    };
  }
  return {
    deliveryTargetType: "agent",
    selectedDeliveryAgentId: parsed.agent_id || task.agent_id,
    selectedDeliveryPresenterAgentId: "",
    selectedDeliveryRoomId: "",
  };
}

function buildAgentReplyInitialState(
  task: ScheduledTaskItem,
  _execution: TaskExecutionInitialState,
): TaskReplyInitialState {
  const executionTarget = executionSessionKey(task);
  return {
    ...deliveryTargetInitialState(task),
    replyMode: resolveReplyMode(task, executionTarget),
    selectedReplySessionKey: selectedReplySessionKey(
      task,
      executionTarget,
    ),
  };
}

function buildScriptReplyInitialState(): TaskReplyInitialState {
  return {
    deliveryTargetType: "agent",
    replyMode: "none",
    selectedDeliveryAgentId: "",
    selectedDeliveryPresenterAgentId: "",
    selectedDeliveryRoomId: "",
    selectedReplySessionKey: "",
  };
}

function needsSessionRebind(
  task: ScheduledTaskItem,
  issue: "delivery" | "execution",
): boolean {
  if (task.session_binding_state !== "rebind_required") {
    return false;
  }
  const issues = task.session_binding_issues ?? [];
  return issues.length === 0 || issues.includes(issue);
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
    const expression = task.schedule.cron_expression;
    const parsed = parseDailyCronExpression(expression);
    if (parsed) {
      return {
        ...defaults,
        dailyTime: parsed.dailyTime,
        kind: "cron",
        selectedWeekdays: parsed.selectedWeekdays,
      };
    }
    const monthly = parseMonthlyCronExpression(expression);
    if (monthly) {
      return {
        ...defaults,
        dailyTime: monthly.dailyTime,
        kind: "monthly",
        monthlyDay: monthly.monthlyDay,
      };
    }
    return {
      ...defaults,
      cronExpression: expression,
      kind: "custom",
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
  preset?: TaskDialogCreatePreset | null,
): TaskDialogInitialState {
  const schedule = createDefaultTaskSchedule();
  return {
    form: {
      dedicatedSessionKey: "",
      deliveryTargetType: "agent",
      enabled: true,
      expiresAt: "",
      executionKind: "agent",
      executionMode: "temporary",
      instruction: preset?.instruction ?? "",
      permissionMode: "copy",
      replyMode: "none",
      selectedAgentId: agentId,
      selectedDeliveryAgentId: agentId,
      selectedDeliveryPresenterAgentId: "",
      selectedDeliveryRoomId: "",
      selectedReplySessionKey: "",
      selectedRoomId: "",
      selectedSessionKey: "",
      targetType: "agent",
      taskName: preset?.taskName ?? "",
    },
    schedule: preset
      ? {
          ...schedule,
          dailyTime: preset.dailyTime,
          kind: "cron",
          selectedWeekdays: preset.selectedWeekdays,
        }
      : schedule,
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
    permissionMode: task.permission_mode ?? "default",
    taskName: task.name,
    selectedReplySessionKey: needsSessionRebind(task, "delivery")
      ? ""
      : reply.selectedReplySessionKey,
    selectedSessionKey: needsSessionRebind(task, "execution")
      ? ""
      : execution.selectedSessionKey,
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
