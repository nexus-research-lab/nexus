"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import {
  createScheduledTaskApi,
  updateScheduledTaskApi,
} from "@/lib/api/capability/scheduled-task-api";
import { isExternalSessionChannel } from "@/lib/conversation/external-session";
import { parseSessionKey } from "@/lib/conversation/session-key";
import {
  type I18nContextValue,
  useI18n,
} from "@/shared/i18n/i18n-context";
import type {
  ScheduledTaskItem,
  UpdateScheduledTaskParams,
} from "@/types/capability/scheduled-task/task";

import {
  buildDefaultTaskDialogInitialState,
  buildTaskDialogInitialState,
} from "./form/task-form-initializer";
import {
  buildScheduledTaskPayload,
  getTaskDialogValidationError,
  type TaskDialogSubmitContext,
} from "./form/task-form-submit";
import { useTaskForm } from "./form/use-task-form";
import { useTaskDialogData } from "./resources/use-task-dialog-data";
import { useTaskSchedule } from "./schedule/use-task-schedule";
import type {
  TaskDialogCreatePreset,
  TaskDialogRefs,
} from "./scheduled-task-dialog-types";

interface TaskDialogControllerOptions {
  agentId: string;
  createPreset?: TaskDialogCreatePreset | null;
  initialTask?: ScheduledTaskItem | null;
  isOpen: boolean;
  onClose: () => void;
  onCreated?: (task: ScheduledTaskItem) => void | Promise<void>;
  onSaved?: (task: ScheduledTaskItem) => void | Promise<void>;
}

interface SubmitTaskDialogOptions {
  context: TaskDialogSubmitContext;
  initialTask: ScheduledTaskItem | null;
  onCreated?: (task: ScheduledTaskItem) => void | Promise<void>;
  onSaved?: (task: ScheduledTaskItem) => void | Promise<void>;
  t: I18nContextValue["t"];
}

function needsLegacyDeliveryRebind(task: ScheduledTaskItem | null): boolean {
  if (!task) {
    return false;
  }
  if (task.execution_kind === "script" || task.delivery.mode === "none") {
    return false;
  }
  const sessionKey = task.delivery.session_key?.trim()
    || task.delivery.to?.trim()
    || "";
  const parsed = parseSessionKey(sessionKey);
  return !parsed.is_structured || (parsed.kind === "agent"
    && parsed.channel === "internal"
    && parsed.ref === "automation-inbox");
}

function buildUpdatePayload(
  payload: UpdateScheduledTaskParams,
  initialTask: ScheduledTaskItem,
  expiresAtDraft: string,
  permissionModeDraft: TaskDialogSubmitContext["form"]["permissionMode"],
): UpdateScheduledTaskParams {
  // source 是任务创建来源，不是可编辑配置。更新时保持原始 provenance，避免
  // 历史 IM 会话已解绑后，修改名称或计划被误判成一次新的会话授权。
  const { source: _source, ...configurationPayload } = payload;
  const withPermissionRefresh = permissionModeDraft === "copy"
    ? { ...configurationPayload, permission_mode: "" as const }
    : configurationPayload;
  if (expiresAtDraft.trim() || initialTask.expires_at === null) {
    return withPermissionRefresh;
  }
  return { ...withPermissionRefresh, clear_expires_at: true };
}

async function submitTaskDialog({
  context,
  initialTask,
  onCreated,
  onSaved,
  t,
}: SubmitTaskDialogOptions): Promise<void> {
  const payload = buildScheduledTaskPayload(context, t);
  if (initialTask) {
    const updatePayload = buildUpdatePayload(
      payload,
      initialTask,
      context.form.expiresAt,
      context.form.permissionMode,
    );
    updatePayload.expected_configuration_version = initialTask.configuration_version;
    const updated = await updateScheduledTaskApi(initialTask.job_id, updatePayload);
    await onSaved?.(updated);
    return;
  }

  const created = await createScheduledTaskApi(payload);
  await onCreated?.(created);
}

function getSubmitErrorMessage(
  error: unknown,
  initialTask: ScheduledTaskItem | null,
  t: I18nContextValue["t"],
): string {
  if (error instanceof Error) {
    return error.message;
  }
  return initialTask
    ? t("capability.scheduled_dialog_save_failed")
    : t("capability.scheduled_dialog_create_failed");
}

export function useTaskDialogController({
  agentId,
  createPreset = null,
  initialTask = null,
  isOpen,
  onClose,
  onCreated,
  onSaved,
}: TaskDialogControllerOptions) {
  const { t } = useI18n();
  const initialState = useMemo(
    () => initialTask
      ? buildTaskDialogInitialState(initialTask)
      : buildDefaultTaskDialogInitialState(agentId, createPreset),
    [agentId, createPreset, initialTask],
  );
  const [errorMessage, setErrorMessage] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const submitInFlightRef = useRef(false);
  const refs: TaskDialogRefs = {
    dailyPickerAnchorRef: useRef<HTMLButtonElement>(null),
    nameRef: useRef<HTMLInputElement>(null),
    singlePickerAnchorRef: useRef<HTMLButtonElement>(null),
  };

  const clearError = useCallback(() => setErrorMessage(null), []);
  const form = useTaskForm(initialState.form, clearError);
  const schedule = useTaskSchedule(initialState.schedule, clearError);
  const hydrateForm = form.hydrate;
  const hydrateSchedule = schedule.hydrate;
  const data = useTaskDialogData({
    form: form.draft,
    isOpen,
  });
  const resolveSelectedRoomIds = form.actions.resolveSelectedRoomIds;
  const selectedSession = data.sessionOptions.find(
    (option) => option.value === form.draft.selectedSessionKey,
  ) ?? null;
  const selectedReplySession = data.deliverySessionOptions.find(
    (option) => option.value === form.draft.selectedReplySessionKey,
  ) ?? null;
  const hasUnavailableExternalSession = (
    !data.sessions.loading
    && !data.sessions.error
    && Boolean(form.draft.selectedSessionKey)
    && isExternalSessionChannel(null, form.draft.selectedSessionKey)
    && !data.sessionOptions.some(
      (option) => option.value === form.draft.selectedSessionKey,
    )
  ) || (
    !data.deliverySessions.loading
    && !data.deliverySessions.error
    && Boolean(form.draft.selectedReplySessionKey)
    && isExternalSessionChannel(null, form.draft.selectedReplySessionKey)
    && !data.deliverySessionOptions.some(
      (option) => option.value === form.draft.selectedReplySessionKey,
    )
  );
  const needsSessionRebind = initialTask?.session_binding_state === "rebind_required"
    || hasUnavailableExternalSession
    || needsLegacyDeliveryRebind(initialTask ?? null);

  const submitContext = useMemo<TaskDialogSubmitContext>(() => ({
    form: form.draft,
    schedule: schedule.draft,
    selectedReplySession,
    selectedSession,
  }), [
    form.draft,
    schedule.draft,
    selectedReplySession,
    selectedSession,
  ]);

  const hydrate = useCallback(() => {
    hydrateForm(initialState.form);
    hydrateSchedule(initialState.schedule);
    setErrorMessage(null);
    setIsSubmitting(false);
    submitInFlightRef.current = false;
  }, [hydrateForm, hydrateSchedule, initialState]);

  const handleSubmit = useCallback(async () => {
    if (submitInFlightRef.current) {
      return;
    }
    const validationError = getTaskDialogValidationError(submitContext, t);
    if (validationError) {
      setErrorMessage(validationError);
      return;
    }

    submitInFlightRef.current = true;
    setIsSubmitting(true);
    setErrorMessage(null);
    try {
      await submitTaskDialog({
        context: submitContext,
        initialTask,
        onCreated,
        onSaved,
        t,
      });
      onClose();
    } catch (error) {
      setErrorMessage(getSubmitErrorMessage(error, initialTask, t));
    } finally {
      submitInFlightRef.current = false;
      setIsSubmitting(false);
    }
  }, [initialTask, onClose, onCreated, onSaved, submitContext, t]);

  useEffect(() => {
    resolveSelectedRoomIds({
      deliveryRoomId: data.resolvedDeliveryRoomId,
      executionRoomId: data.resolvedExecutionRoomId,
    });
  }, [
    data.resolvedDeliveryRoomId,
    data.resolvedExecutionRoomId,
    resolveSelectedRoomIds,
  ]);

  useEffect(() => {
    if (!isOpen) {
      return;
    }
    hydrate();
  }, [hydrate, isOpen]);

  return {
    clearError,
    data,
    errorMessage,
    form,
    handleSubmit,
    isSubmitting,
    needsSessionRebind,
    refs,
    schedule,
  };
}
