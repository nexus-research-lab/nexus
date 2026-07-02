/**
 * =====================================================
 * @File   : use-scheduled-task-dialog-state.ts
 * @Date   : 2026-04-16 13:44
 * @Author : leemysw
 * 2026-04-16 13:44   Create
 * =====================================================
 */

"use client";

import { useCallback, useEffect, useRef, useState } from "react";

import { create_scheduled_task_api, update_scheduled_task_api } from "@/lib/api/scheduled-task-api";
import { close_on_escape } from "@/shared/ui/dialog/dialog-keyboard";
import type { ScheduledTaskItem } from "@/types/capability/scheduled-task";

import { get_default_timezone } from "./scheduled-task-dialog-options";
import {
  build_default_dialog_initial_state,
  build_task_dialog_initial_state,
} from "./scheduled-task-dialog-initializer";
import {
  build_scheduled_task_payload,
  get_scheduled_task_validation_error,
  type ScheduledTaskDialogSubmitState,
} from "./scheduled-task-dialog-submit";
import type {
  ExecutionKind,
  ExecutionMode,
  ReplyMode,
  TargetType,
} from "./scheduled-task-dialog-types";
import { useScheduledTaskDialogData } from "./use-scheduled-task-dialog-data";
import { useScheduledTaskDialogScheduleState } from "./use-scheduled-task-dialog-schedule";

export function useScheduledTaskDialogState({
  agent_id: agentId,
  initial_task: initialTask,
  is_open: isOpen,
  on_close: onClose,
  on_created: onCreated,
  on_saved: onSaved,
}: {
  agent_id: string;
  initial_task?: ScheduledTaskItem | null;
  is_open: boolean;
  on_close: () => void;
  on_created?: (task: ScheduledTaskItem) => void | Promise<void>;
  on_saved?: (task: ScheduledTaskItem) => void | Promise<void>;
}) {
  const nameRef = useRef<HTMLInputElement>(null);
  const [taskName, setTaskName] = useState("");
  const [targetType, setTargetTypeState] = useState<TargetType>("agent");
  const [executionKind, setExecutionKindState] = useState<ExecutionKind>("agent");
  const [selectedAgentId, setSelectedAgentIdState] = useState(agentId);
  const [selectedRoomId, setSelectedRoomIdState] = useState("");
  const [executionMode, setExecutionModeState] = useState<ExecutionMode>("existing");
  const [selectedSessionKey, setSelectedSessionKeyState] = useState("");
  const [replyMode, setReplyMode] = useState<ReplyMode>("execution");
  const [selectedReplySessionKey, setSelectedReplySessionKeyState] = useState("");
  const [dedicatedSessionKey, setDedicatedSessionKey] = useState("");
  const [timezone, setTimezone] = useState(get_default_timezone());
  const [enabled, setEnabled] = useState(true);
  const [instruction, setInstruction] = useState("");
  const [errorMessage, setErrorMessage] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const dailyPickerAnchorRef = useRef<HTMLButtonElement>(null);
  const singlePickerAnchorRef = useRef<HTMLButtonElement>(null);

  const schedule = useScheduledTaskDialogScheduleState(timezone);
  const hydrateSchedule = schedule.hydrate;
  const resetSchedule = schedule.reset;

  const resetContextSelection = useCallback(() => {
    setSelectedSessionKeyState("");
    setSelectedReplySessionKeyState("");
    setErrorMessage(null);
  }, []);

  const setTargetType = useCallback((value: TargetType) => {
    if (executionKind === "script") {
      setTargetTypeState("agent");
      return;
    }
    setTargetTypeState(value);
    resetContextSelection();
  }, [executionKind, resetContextSelection]);

  const setExecutionKind = useCallback((value: ExecutionKind) => {
    setExecutionKindState(value);
    if (value === "script") {
      setTargetTypeState("agent");
      setExecutionModeState("temporary");
      setReplyMode("none");
      setSelectedSessionKeyState("");
      setSelectedReplySessionKeyState("");
      setDedicatedSessionKey("");
    }
    setErrorMessage(null);
  }, []);

  const setSelectedAgentId = useCallback((value: string) => {
    setSelectedAgentIdState(value);
    resetContextSelection();
  }, [resetContextSelection]);

  const setSelectedRoomId = useCallback((value: string) => {
    setSelectedRoomIdState(value);
    resetContextSelection();
  }, [resetContextSelection]);

  const setSelectedSessionKey = useCallback((value: string) => {
    setSelectedSessionKeyState(value);
    setErrorMessage(null);
  }, []);

  const setSelectedReplySessionKey = useCallback((value: string) => {
    setSelectedReplySessionKeyState(value);
    setErrorMessage(null);
  }, []);

  const setExecutionMode = useCallback((value: ExecutionMode) => {
    setExecutionModeState(value);
    if (value === "main") {
      setReplyMode("none");
      setSelectedReplySessionKeyState("");
    }
    setErrorMessage(null);
  }, []);

  const data = useScheduledTaskDialogData({
    is_open: isOpen,
    target_type: targetType,
    selected_agent_id: selectedAgentId,
    selected_room_id: selectedRoomId,
  });

  const selectedSession = data.session_options.find((option) => option.value === selectedSessionKey) ?? null;
  const selectedReplySession = data.session_options.find((option) => option.value === selectedReplySessionKey) ?? null;

  const applyDialogInitialState = useCallback(() => {
    const nextState = initialTask
      ? build_task_dialog_initial_state(initialTask)
      : build_default_dialog_initial_state(agentId);

    setTaskName(nextState.task_name);
    setTargetTypeState(nextState.target_type);
    setExecutionKindState(nextState.execution_kind);
    setSelectedAgentIdState(nextState.selected_agent_id);
    setSelectedRoomIdState(nextState.selected_room_id);
    setExecutionModeState(nextState.execution_mode);
    setSelectedSessionKeyState(nextState.selected_session_key);
    setReplyMode(nextState.reply_mode);
    setSelectedReplySessionKeyState(nextState.selected_reply_session_key);
    setDedicatedSessionKey(nextState.dedicated_session_key);
    setTimezone(nextState.timezone);
    setEnabled(nextState.enabled);
    setInstruction(nextState.instruction);
    setErrorMessage(null);
    setIsSubmitting(false);

    if (initialTask && nextState.schedule_snapshot) {
      hydrateSchedule(nextState.schedule_snapshot);
      return;
    }
    resetSchedule();
  }, [agentId, hydrateSchedule, initialTask, resetSchedule]);

  function buildSubmitState(): ScheduledTaskDialogSubmitState {
    return {
      task_name: taskName,
      target_type: targetType,
      execution_kind: executionKind,
      selected_agent_id: selectedAgentId,
      selected_room_id: selectedRoomId,
      execution_mode: executionMode,
      selected_session_key: selectedSessionKey,
      reply_mode: replyMode,
      selected_reply_session_key: selectedReplySessionKey,
      dedicated_session_key: dedicatedSessionKey,
      timezone,
      enabled,
      instruction,
      every_value: schedule.every_value,
      every_unit: schedule.every_unit,
      daily_time: schedule.daily_time,
      selected_weekdays: schedule.selected_weekdays,
      run_at: schedule.run_at,
      selected_session: selectedSession,
      selected_reply_session: selectedReplySession,
      agent_options: data.agent_options,
      room_options: data.room_options,
      schedule_kind: schedule.schedule_kind,
    };
  }

  function isRoomExecutorSelectionRequired() {
    return executionKind !== "script" && targetType === "room" && executionMode !== "existing";
  }

  async function handleSubmit() {
    const submitState = buildSubmitState();
    const validationError = get_scheduled_task_validation_error(submitState);
    if (validationError) {
      setErrorMessage(validationError);
      return;
    }

    setIsSubmitting(true);
    setErrorMessage(null);
    try {
      const payload = build_scheduled_task_payload(submitState, initialTask?.source);
      if (initialTask) {
        const updated = await update_scheduled_task_api(initialTask.job_id, payload);
        await onSaved?.(updated);
      } else {
        const created = await create_scheduled_task_api(payload);
        await onCreated?.(created);
      }
      onClose();
    } catch (error) {
      setErrorMessage(error instanceof Error ? error.message : "创建任务失败");
    } finally {
      setIsSubmitting(false);
    }
  }

  useEffect(() => {
    if (isOpen && nameRef.current) {
      nameRef.current.focus();
    }
  }, [isOpen]);

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if (!isOpen) {
        return;
      }
      close_on_escape(event, onClose);
    };
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [isOpen, onClose]);

  useEffect(() => {
    if (!isOpen) {
      return;
    }
    applyDialogInitialState();
  }, [applyDialogInitialState, isOpen]);

  return {
    ...schedule,
    ...data,
    name_ref: nameRef,
    task_name: taskName,
    set_task_name: setTaskName,
    target_type: targetType,
    set_target_type: setTargetType,
    execution_kind: executionKind,
    set_execution_kind: setExecutionKind,
    selected_agent_id: selectedAgentId,
    set_selected_agent_id: setSelectedAgentId,
    selected_room_id: selectedRoomId,
    set_selected_room_id: setSelectedRoomId,
    execution_mode: executionMode,
    set_execution_mode: setExecutionMode,
    selected_session_key: selectedSessionKey,
    set_selected_session_key: setSelectedSessionKey,
    reply_mode: replyMode,
    set_reply_mode: setReplyMode,
    selected_reply_session_key: selectedReplySessionKey,
    set_selected_reply_session_key: setSelectedReplySessionKey,
    dedicated_session_key: dedicatedSessionKey,
    set_dedicated_session_key: setDedicatedSessionKey,
    enabled,
    set_enabled: setEnabled,
    timezone,
    set_timezone: setTimezone,
    instruction,
    set_instruction: setInstruction,
    error_message: errorMessage,
    set_error_message: setErrorMessage,
    is_submitting: isSubmitting,
    daily_picker_anchor_ref: dailyPickerAnchorRef,
    single_picker_anchor_ref: singlePickerAnchorRef,
    selected_session: selectedSession,
    selected_reply_session: selectedReplySession,
    is_room_executor_selection_required: isRoomExecutorSelectionRequired,
    handle_submit: handleSubmit,
  };
}
