"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import {
  createScheduledTaskApi,
  updateScheduledTaskApi,
} from "@/lib/api/capability/scheduled-task-api";
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
}

function buildUpdatePayload(
  payload: UpdateScheduledTaskParams,
  initialTask: ScheduledTaskItem,
  expiresAtDraft: string,
): UpdateScheduledTaskParams {
  if (expiresAtDraft.trim() || initialTask.expires_at === null) {
    return payload;
  }
  return { ...payload, clear_expires_at: true };
}

async function submitTaskDialog({
  context,
  initialTask,
  onCreated,
  onSaved,
}: SubmitTaskDialogOptions): Promise<void> {
  const payload = buildScheduledTaskPayload(context, initialTask?.source);
  if (initialTask) {
    const updatePayload = buildUpdatePayload(
      payload,
      initialTask,
      context.form.expiresAt,
    );
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
): string {
  if (error instanceof Error) {
    return error.message;
  }
  return initialTask ? "保存任务失败" : "创建任务失败";
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
  const data = useTaskDialogData({ form: form.draft, isOpen });
  const selectedSession = data.sessionOptions.find(
    (option) => option.value === form.draft.selectedSessionKey,
  ) ?? null;
  const selectedReplySession = data.sessionOptions.find(
    (option) => option.value === form.draft.selectedReplySessionKey,
  ) ?? null;

  const submitContext = useMemo<TaskDialogSubmitContext>(() => ({
    agentOptions: data.agentOptions,
    form: form.draft,
    roomOptions: data.roomOptions,
    schedule: schedule.draft,
    selectedReplySession,
    selectedSession,
  }), [
    data.agentOptions,
    data.roomOptions,
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
    const validationError = getTaskDialogValidationError(submitContext);
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
      });
      onClose();
    } catch (error) {
      setErrorMessage(getSubmitErrorMessage(error, initialTask));
    } finally {
      submitInFlightRef.current = false;
      setIsSubmitting(false);
    }
  }, [initialTask, onClose, onCreated, onSaved, submitContext]);

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
    refs,
    schedule,
  };
}
