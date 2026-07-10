import type {
  CreateScheduledTaskParams,
  ScheduledTaskDeliveryTarget,
  ScheduledTaskSchedule,
  ScheduledTaskSessionTarget,
  ScheduledTaskSource,
} from "@/types/capability/scheduled-task";

import type {
  TaskDialogLabelOption,
  TaskDialogSessionOption,
  TaskFormDraft,
  TaskScheduleDraft,
} from "../scheduled-task-dialog-types";
import {
  buildDailyCronExpression,
  toIntervalSeconds,
  zonedDateTimeToEpochMs,
} from "../schedule/task-schedule-time";

export interface TaskDialogSubmitContext {
  agentOptions: TaskDialogLabelOption[];
  form: TaskFormDraft;
  roomOptions: TaskDialogLabelOption[];
  schedule: TaskScheduleDraft;
  selectedReplySession: TaskDialogSessionOption | null;
  selectedSession: TaskDialogSessionOption | null;
}

type Validator = (context: TaskDialogSubmitContext) => string | null;

function validateBasics({ form }: TaskDialogSubmitContext): string | null {
  if (!form.taskName.trim()) {
    return "请输入任务名称";
  }
  if (!form.instruction.trim()) {
    return form.executionKind === "script" ? "请输入脚本内容" : "请输入任务指令";
  }
  return null;
}

function validateTarget({ form }: TaskDialogSubmitContext): string | null {
  if (form.executionKind === "script" || form.targetType === "agent") {
    return form.selectedAgentId.trim() ? null : "请选择智能体";
  }
  return form.selectedRoomId.trim() ? null : "请选择 Room";
}

function validateExecution(context: TaskDialogSubmitContext): string | null {
  const { form, selectedSession } = context;
  if (form.executionKind === "script") {
    return null;
  }
  if (form.targetType === "room" && !selectedSession) {
    return "请选择执行成员";
  }
  if (form.executionMode === "existing" && !selectedSession) {
    return "请选择执行会话";
  }
  if (form.executionMode === "dedicated" && !form.dedicatedSessionKey.trim()) {
    return "请输入专用长期会话名称";
  }
  return null;
}

function validateSchedule({ schedule }: TaskDialogSubmitContext): string | null {
  if (schedule.kind === "every") {
    return toIntervalSeconds(schedule.everyValue, schedule.everyUnit) === null
      ? "循环间隔必须是大于 0 的整数"
      : null;
  }
  if (schedule.kind === "cron") {
    if (schedule.selectedWeekdays.length === 0) {
      return "请至少选择一个执行日";
    }
    return buildDailyCronExpression(
      schedule.dailyTime,
      schedule.selectedWeekdays,
    ) ? null : "请选择有效的固定执行时间";
  }
  const runAtEpoch = zonedDateTimeToEpochMs(
    schedule.runAt,
    schedule.timezone.trim() || "Asia/Shanghai",
  );
  if (runAtEpoch === null) {
    return "请选择有效的执行时间";
  }
  return runAtEpoch > Date.now() ? null : "单次执行时间必须晚于当前时间";
}

function validateDelivery(context: TaskDialogSubmitContext): string | null {
  const { form, selectedReplySession } = context;
  if (form.executionKind === "script") {
    return null;
  }
  if (form.executionMode === "main" && form.replyMode !== "none") {
    return "主会话任务暂不支持额外结果回传";
  }
  if (form.replyMode === "selected" && !selectedReplySession) {
    return "请选择回复会话";
  }
  return null;
}

const VALIDATORS: Validator[] = [
  validateBasics,
  validateTarget,
  validateExecution,
  validateSchedule,
  validateDelivery,
];

export function getTaskDialogValidationError(
  context: TaskDialogSubmitContext,
): string | null {
  for (const validate of VALIDATORS) {
    const error = validate(context);
    if (error) {
      return error;
    }
  }
  return null;
}

function buildSessionTarget(
  context: TaskDialogSubmitContext,
): ScheduledTaskSessionTarget {
  const { form, selectedSession } = context;
  if (form.targetType === "room" || form.executionMode === "existing") {
    if (!selectedSession) {
      throw new Error(form.targetType === "room" ? "请选择执行成员" : "请选择执行会话");
    }
    return {
      bound_session_key: selectedSession.sessionKey,
      kind: "bound",
      wake_mode: "next-heartbeat",
    };
  }
  if (form.executionMode === "dedicated") {
    return {
      kind: "named",
      named_session_key: form.dedicatedSessionKey.trim(),
      wake_mode: "next-heartbeat",
    };
  }
  return {
    kind: form.executionMode === "main" ? "main" : "isolated",
    wake_mode: "next-heartbeat",
  };
}

function buildDelivery(
  context: TaskDialogSubmitContext,
): ScheduledTaskDeliveryTarget {
  const { form, selectedReplySession, selectedSession } = context;
  if (form.replyMode === "none" || form.executionMode === "main") {
    return { mode: "none" };
  }
  if (form.replyMode === "selected") {
    if (!selectedReplySession) {
      throw new Error("请选择回复会话");
    }
    return {
      channel: "websocket",
      mode: "explicit",
      to: selectedReplySession.sessionKey,
    };
  }
  if (!selectedSession) {
    return { mode: "none" };
  }
  return {
    channel: "websocket",
    mode: "explicit",
    to: selectedSession.sessionKey,
  };
}

function buildSchedule(schedule: TaskScheduleDraft): ScheduledTaskSchedule {
  const timezone = schedule.timezone.trim() || "Asia/Shanghai";
  if (schedule.kind === "every") {
    const intervalSeconds = toIntervalSeconds(
      schedule.everyValue,
      schedule.everyUnit,
    );
    if (intervalSeconds === null) {
      throw new Error("循环间隔必须是大于 0 的整数");
    }
    return { interval_seconds: intervalSeconds, kind: "every", timezone };
  }
  if (schedule.kind === "cron") {
    const cronExpression = buildDailyCronExpression(
      schedule.dailyTime,
      schedule.selectedWeekdays,
    );
    if (!cronExpression) {
      throw new Error("请选择有效的固定执行时间");
    }
    return { cron_expression: cronExpression, kind: "cron", timezone };
  }
  return { kind: "at", run_at: schedule.runAt.trim(), timezone };
}

function resolveAgentId(context: TaskDialogSubmitContext): string {
  const { form, selectedSession } = context;
  if (form.executionKind === "script" || form.targetType === "agent") {
    return form.selectedAgentId.trim();
  }
  if (!selectedSession) {
    throw new Error("请选择执行成员");
  }
  return selectedSession.agentId;
}

function selectedLabel(
  options: TaskDialogLabelOption[],
  value: string,
): string {
  return options.find((option) => option.value === value)?.label || value.trim();
}

function buildSource(
  context: TaskDialogSubmitContext,
  originalSource?: ScheduledTaskSource | null,
): ScheduledTaskSource {
  const { agentOptions, form, roomOptions, selectedSession } = context;
  const isRoom = form.executionKind === "agent" && form.targetType === "room";
  const contextId = isRoom ? form.selectedRoomId.trim() : form.selectedAgentId.trim();
  return {
    context_id: contextId,
    context_label: isRoom
      ? selectedLabel(roomOptions, contextId)
      : selectedLabel(agentOptions, contextId),
    context_type: isRoom ? "room" : "agent",
    creator_agent_id: originalSource?.creator_agent_id ?? null,
    kind: originalSource?.kind ?? "user_page",
    session_key: form.executionKind === "script"
      ? null
      : selectedSession?.sessionKey ?? null,
    session_label: form.executionKind === "script"
      ? null
      : selectedSession?.label ?? null,
  };
}

export function buildScheduledTaskPayload(
  context: TaskDialogSubmitContext,
  originalSource?: ScheduledTaskSource | null,
): CreateScheduledTaskParams {
  const { form, schedule } = context;
  const common = {
    agent_id: resolveAgentId(context),
    enabled: form.enabled,
    instruction: form.instruction.trim(),
    name: form.taskName.trim(),
    schedule: buildSchedule(schedule),
    source: buildSource(context, originalSource),
  };
  if (form.executionKind === "script") {
    return {
      ...common,
      delivery: { mode: "none" },
      execution_kind: "script",
      session_target: { kind: "isolated", wake_mode: "next-heartbeat" },
    };
  }
  return {
    ...common,
    delivery: buildDelivery(context),
    execution_kind: "agent",
    session_target: buildSessionTarget(context),
  };
}
