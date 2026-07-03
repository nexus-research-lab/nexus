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

import { createScheduledTaskApi, updateScheduledTaskApi } from "@/lib/api/scheduled-task-api";
import { closeOnEscape } from "@/shared/ui/dialog/dialog-keyboard";
import type { ScheduledTaskItem } from "@/types/capability/scheduled-task";

import { getDefaultTimezone } from "./scheduled-task-dialog-options";
import {
  buildDefaultDialogInitialState,
  buildTaskDialogInitialState,
} from "./scheduled-task-dialog-initializer";
import {
  buildScheduledTaskPayload,
  getScheduledTaskValidationError,
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
  agentId,
  initialTask,
  isOpen,
  onClose,
  onCreated,
  onSaved,
}: {
  agentId: string;
  initialTask?: ScheduledTaskItem | null;
  isOpen: boolean;
  onClose: () => void;
  onCreated?: (task: ScheduledTaskItem) => void | Promise<void>;
  onSaved?: (task: ScheduledTaskItem) => void | Promise<void>;
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
  const [timezone, setTimezone] = useState(getDefaultTimezone());
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
    isOpen,
    targetType,
    selectedAgentId,
    selectedRoomId,
  });

  const selectedSession = data.sessionOptions.find((option) => option.value === selectedSessionKey) ?? null;
  const selectedReplySession = data.sessionOptions.find((option) => option.value === selectedReplySessionKey) ?? null;

  const applyDialogInitialState = useCallback(() => {
    const nextState = initialTask
      ? buildTaskDialogInitialState(initialTask)
      : buildDefaultDialogInitialState(agentId);

    setTaskName(nextState.taskName);
    setTargetTypeState(nextState.targetType);
    setExecutionKindState(nextState.executionKind);
    setSelectedAgentIdState(nextState.selectedAgentId);
    setSelectedRoomIdState(nextState.selectedRoomId);
    setExecutionModeState(nextState.executionMode);
    setSelectedSessionKeyState(nextState.selectedSessionKey);
    setReplyMode(nextState.replyMode);
    setSelectedReplySessionKeyState(nextState.selectedReplySessionKey);
    setDedicatedSessionKey(nextState.dedicatedSessionKey);
    setTimezone(nextState.timezone);
    setEnabled(nextState.enabled);
    setInstruction(nextState.instruction);
    setErrorMessage(null);
    setIsSubmitting(false);

    if (initialTask && nextState.scheduleSnapshot) {
      hydrateSchedule(nextState.scheduleSnapshot);
      return;
    }
    resetSchedule();
  }, [agentId, hydrateSchedule, initialTask, resetSchedule]);

  function buildSubmitState(): ScheduledTaskDialogSubmitState {
    return {
      taskName: taskName,
      targetType: targetType,
      executionKind: executionKind,
      selectedAgentId: selectedAgentId,
      selectedRoomId: selectedRoomId,
      executionMode: executionMode,
      selectedSessionKey: selectedSessionKey,
      replyMode: replyMode,
      selectedReplySessionKey: selectedReplySessionKey,
      dedicatedSessionKey: dedicatedSessionKey,
      timezone,
      enabled,
      instruction,
      everyValue: schedule.everyValue,
      everyUnit: schedule.everyUnit,
      dailyTime: schedule.dailyTime,
      selectedWeekdays: schedule.selectedWeekdays,
      runAt: schedule.runAt,
      selectedSession: selectedSession,
      selectedReplySession: selectedReplySession,
      agentOptions: data.agentOptions,
      roomOptions: data.roomOptions,
      scheduleKind: schedule.scheduleKind,
    };
  }

  function isRoomExecutorSelectionRequired() {
    return executionKind !== "script" && targetType === "room" && executionMode !== "existing";
  }

  async function handleSubmit() {
    const submitState = buildSubmitState();
    const validationError = getScheduledTaskValidationError(submitState);
    if (validationError) {
      setErrorMessage(validationError);
      return;
    }

    setIsSubmitting(true);
    setErrorMessage(null);
    try {
      const payload = buildScheduledTaskPayload(submitState, initialTask?.source);
      if (initialTask) {
        const updated = await updateScheduledTaskApi(initialTask.job_id, payload);
        await onSaved?.(updated);
      } else {
        const created = await createScheduledTaskApi(payload);
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
      closeOnEscape(event, onClose);
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
    nameRef: nameRef,
    taskName: taskName,
    setTaskName: setTaskName,
    targetType: targetType,
    setTargetType: setTargetType,
    executionKind: executionKind,
    setExecutionKind: setExecutionKind,
    selectedAgentId: selectedAgentId,
    setSelectedAgentId: setSelectedAgentId,
    selectedRoomId: selectedRoomId,
    setSelectedRoomId: setSelectedRoomId,
    executionMode: executionMode,
    setExecutionMode: setExecutionMode,
    selectedSessionKey: selectedSessionKey,
    setSelectedSessionKey: setSelectedSessionKey,
    replyMode: replyMode,
    setReplyMode: setReplyMode,
    selectedReplySessionKey: selectedReplySessionKey,
    setSelectedReplySessionKey: setSelectedReplySessionKey,
    dedicatedSessionKey: dedicatedSessionKey,
    setDedicatedSessionKey: setDedicatedSessionKey,
    enabled,
    setEnabled: setEnabled,
    timezone,
    setTimezone: setTimezone,
    instruction,
    setInstruction: setInstruction,
    errorMessage: errorMessage,
    setErrorMessage: setErrorMessage,
    isSubmitting: isSubmitting,
    dailyPickerAnchorRef: dailyPickerAnchorRef,
    singlePickerAnchorRef: singlePickerAnchorRef,
    selectedSession: selectedSession,
    selectedReplySession: selectedReplySession,
    isRoomExecutorSelectionRequired: isRoomExecutorSelectionRequired,
    handleSubmit: handleSubmit,
  };
}
