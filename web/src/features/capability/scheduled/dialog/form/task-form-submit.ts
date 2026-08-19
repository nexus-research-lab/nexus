import { isExternalSessionChannel } from "@/lib/conversation/external-session";
import type { I18nContextValue } from "@/shared/i18n/i18n-context";
import type {
  CreateScheduledTaskParams,
  ScheduledTaskDeliveryTarget,
  ScheduledTaskSchedule,
  ScheduledTaskSessionTarget,
} from "@/types/capability/scheduled-task/task";

import type {
  TaskDialogSessionOption,
  TaskFormDraft,
  TaskScheduleDraft,
} from "../scheduled-task-dialog-types";
import {
  buildDailyCronExpression,
  buildMonthlyCronExpression,
  isStandardCronExpression,
  toIntervalSeconds,
  zonedDateTimeToEpochMs,
} from "../schedule/task-schedule-time";

type Translate = I18nContextValue["t"];

export interface TaskDialogSubmitContext {
  defaultDeliveryRoomAgentId: string;
  defaultExecutionRoomAgentId: string;
  form: TaskFormDraft;
  schedule: TaskScheduleDraft;
  selectedReplySession: TaskDialogSessionOption | null;
  selectedSession: TaskDialogSessionOption | null;
}

type Validator = (
  context: TaskDialogSubmitContext,
  t: Translate,
) => string | null;

function validateBasics(
  { form }: TaskDialogSubmitContext,
  t: Translate,
): string | null {
  if (!form.taskName.trim()) {
    return t("capability.scheduled_dialog_validation_name");
  }
  if (!form.instruction.trim()) {
    return t("capability.scheduled_dialog_validation_instruction");
  }
  return null;
}

function validateTarget(
  { form }: TaskDialogSubmitContext,
  t: Translate,
): string | null {
  if (form.executionKind === "script" || form.targetType === "agent") {
    return form.selectedAgentId.trim()
      ? null
      : t("capability.scheduled_dialog_validation_agent");
  }
  return form.selectedRoomId.trim()
    ? null
    : t("capability.scheduled_dialog_validation_room");
}

function validateExecution(
  context: TaskDialogSubmitContext,
  t: Translate,
): string | null {
  const { form, selectedSession } = context;
  if (form.executionKind === "script") {
    return null;
  }
  if (form.targetType === "room" && !selectedSession) {
    return t("capability.scheduled_dialog_validation_session");
  }
  if (form.targetType === "room" && !resolveExecutionRoomAgentId(context)) {
    return t("capability.scheduled_dialog_validation_member");
  }
  if (form.executionMode === "existing" && !selectedSession) {
    return t("capability.scheduled_dialog_validation_session");
  }
  if (form.executionMode === "dedicated" && !form.dedicatedSessionKey.trim()) {
    return t("capability.scheduled_dialog_validation_dedicated");
  }
  return null;
}

function validateSchedule(
  { schedule }: TaskDialogSubmitContext,
  t: Translate,
): string | null {
  if (schedule.kind === "every") {
    return toIntervalSeconds(schedule.everyValue, schedule.everyUnit) === null
      ? t("capability.scheduled_dialog_validation_interval")
      : null;
  }
  if (schedule.kind === "cron") {
    if (schedule.selectedWeekdays.length === 0) {
      return t("capability.scheduled_dialog_validation_weekday");
    }
    return buildDailyCronExpression(
      schedule.dailyTime,
      schedule.selectedWeekdays,
    ) ? null : t("capability.scheduled_dialog_validation_daily_time");
  }
  if (schedule.kind === "monthly") {
    return buildMonthlyCronExpression(
      schedule.dailyTime,
      schedule.monthlyDay,
    ) ? null : t("capability.scheduled_dialog_validation_monthly");
  }
  if (schedule.kind === "custom") {
    return isStandardCronExpression(schedule.cronExpression)
      ? null
      : t("capability.scheduled_dialog_validation_cron");
  }
  const runAtEpoch = zonedDateTimeToEpochMs(
    schedule.runAt,
    schedule.timezone.trim() || "Asia/Shanghai",
  );
  if (runAtEpoch === null) {
    return t("capability.scheduled_dialog_validation_run_at");
  }
  return runAtEpoch > Date.now()
    ? null
    : t("capability.scheduled_dialog_validation_run_at_future");
}

function validateExpiration(
  { form, schedule }: TaskDialogSubmitContext,
  t: Translate,
): string | null {
  if (!form.expiresAt.trim()) {
    return null;
  }
  const expiresAt = zonedDateTimeToEpochMs(
    form.expiresAt,
    schedule.timezone.trim() || "Asia/Shanghai",
  );
  if (expiresAt === null) {
    return t("capability.scheduled_dialog_validation_expiration");
  }
  return expiresAt > Date.now()
    ? null
    : t("capability.scheduled_dialog_validation_expiration_future");
}

function validateDelivery(
  context: TaskDialogSubmitContext,
  t: Translate,
): string | null {
  const { form, selectedReplySession } = context;
  if (form.executionKind === "script") {
    return null;
  }
  if (form.executionMode === "main" && form.replyMode !== "none") {
    return t("capability.scheduled_dialog_validation_main_delivery");
  }
  if (form.replyMode === "selected" && !selectedReplySession) {
    return t("capability.scheduled_dialog_validation_reply_session");
  }
  if (form.replyMode === "selected"
    && form.deliveryTargetType === "agent"
    && !form.selectedDeliveryAgentId.trim()) {
    return t("capability.scheduled_dialog_validation_delivery_agent");
  }
  if (form.replyMode === "selected"
    && form.deliveryTargetType === "room"
    && !form.selectedDeliveryRoomId.trim()) {
    return t("capability.scheduled_dialog_validation_delivery_room");
  }
  if (form.replyMode === "selected"
    && form.deliveryTargetType === "room"
    && !resolveDeliveryRoomAgentId(context)) {
    return t("capability.scheduled_dialog_validation_delivery_room_agent");
  }
  return null;
}

const VALIDATORS: Validator[] = [
  validateBasics,
  validateTarget,
  validateExecution,
  validateSchedule,
  validateExpiration,
  validateDelivery,
];

export function getTaskDialogValidationError(
  context: TaskDialogSubmitContext,
  t: Translate,
): string | null {
  for (const validate of VALIDATORS) {
    const error = validate(context, t);
    if (error) {
      return error;
    }
  }
  return null;
}

function buildSessionTarget(
  context: TaskDialogSubmitContext,
  t: Translate,
): ScheduledTaskSessionTarget {
  const { form, selectedSession } = context;
  if (form.targetType === "room" || form.executionMode === "existing") {
    if (!selectedSession) {
      throw new Error(t(form.targetType === "room"
        ? "capability.scheduled_dialog_validation_member"
        : "capability.scheduled_dialog_validation_session"));
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
  t: Translate,
): ScheduledTaskDeliveryTarget {
  const { form, selectedReplySession } = context;
  if (form.replyMode === "none" || form.executionMode === "main") {
    return { mode: "none" };
  }
  if (!selectedReplySession) {
    throw new Error(t("capability.scheduled_dialog_validation_reply_session"));
  }
  return buildSessionDelivery(
    selectedReplySession.sessionKey,
    form.deliveryTargetType === "room"
      ? resolveDeliveryRoomAgentId(context)
      : "",
  );
}

function buildSessionDelivery(
  sessionKey: string,
  roomAgentId: string,
): ScheduledTaskDeliveryTarget {
  if (isExternalSessionChannel(null, sessionKey)) {
    return {
      mode: "last",
      session_key: sessionKey,
    };
  }
  return {
    agent_id: roomAgentId || undefined,
    channel: sessionKey.includes(":internal:") ? "internal" : "websocket",
    mode: "explicit",
    session_key: sessionKey,
    to: sessionKey,
  };
}

function buildSchedule(
  schedule: TaskScheduleDraft,
  t: Translate,
): ScheduledTaskSchedule {
  const timezone = schedule.timezone.trim() || "Asia/Shanghai";
  if (schedule.kind === "every") {
    const intervalSeconds = toIntervalSeconds(
      schedule.everyValue,
      schedule.everyUnit,
    );
    if (intervalSeconds === null) {
      throw new Error(t("capability.scheduled_dialog_validation_interval"));
    }
    return { interval_seconds: intervalSeconds, kind: "every", timezone };
  }
  if (schedule.kind === "cron") {
    const cronExpression = buildDailyCronExpression(
      schedule.dailyTime,
      schedule.selectedWeekdays,
    );
    if (!cronExpression) {
      throw new Error(t("capability.scheduled_dialog_validation_daily_time"));
    }
    return { cron_expression: cronExpression, kind: "cron", timezone };
  }
  if (schedule.kind === "monthly") {
    const cronExpression = buildMonthlyCronExpression(
      schedule.dailyTime,
      schedule.monthlyDay,
    );
    if (!cronExpression) {
      throw new Error(t("capability.scheduled_dialog_validation_monthly"));
    }
    return { cron_expression: cronExpression, kind: "cron", timezone };
  }
  if (schedule.kind === "custom") {
    const cronExpression = schedule.cronExpression.trim();
    if (!isStandardCronExpression(cronExpression)) {
      throw new Error(t("capability.scheduled_dialog_validation_cron"));
    }
    return { cron_expression: cronExpression, kind: "cron", timezone };
  }
  return { kind: "at", run_at: schedule.runAt.trim(), timezone };
}

function buildExpiresAt(
  form: TaskFormDraft,
  schedule: TaskScheduleDraft,
  t: Translate,
): string | undefined {
  if (!form.expiresAt.trim()) {
    return undefined;
  }
  const epochMs = zonedDateTimeToEpochMs(
    form.expiresAt,
    schedule.timezone.trim() || "Asia/Shanghai",
  );
  if (epochMs === null) {
    throw new Error(t("capability.scheduled_dialog_validation_expiration"));
  }
  return new Date(epochMs).toISOString();
}

function resolveAgentId(
  context: TaskDialogSubmitContext,
  t: Translate,
): string {
  const { form } = context;
  if (form.executionKind === "script" || form.targetType === "agent") {
    return form.selectedAgentId.trim();
  }
  const agentId = resolveExecutionRoomAgentId(context);
  if (!agentId) {
    throw new Error(t("capability.scheduled_dialog_validation_member"));
  }
  return agentId;
}

function resolveExecutionRoomAgentId(
  context: TaskDialogSubmitContext,
): string {
  return context.form.selectedAgentId.trim()
    || context.defaultExecutionRoomAgentId.trim();
}

function resolveDeliveryRoomAgentId(
  context: TaskDialogSubmitContext,
): string {
  return context.form.selectedDeliveryPresenterAgentId.trim()
    || context.defaultDeliveryRoomAgentId.trim();
}

export function buildScheduledTaskPayload(
  context: TaskDialogSubmitContext,
  t: Translate,
): CreateScheduledTaskParams {
  const { form, schedule } = context;
  const common = {
    agent_id: resolveAgentId(context, t),
    enabled: form.enabled,
    expires_at: buildExpiresAt(form, schedule, t),
    instruction: form.instruction.trim(),
    name: form.taskName.trim(),
    permission_mode: form.permissionMode === "copy"
      ? undefined
      : form.permissionMode,
    schedule: buildSchedule(schedule, t),
  };
  return {
    ...common,
    delivery: buildDelivery(context, t),
    execution_kind: "agent",
    session_target: buildSessionTarget(context, t),
  };
}
