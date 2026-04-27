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
  ExecutionMode,
  ReplyMode,
  TargetType,
} from "./scheduled-task-dialog-types";
import { useScheduledTaskDialogData } from "./use-scheduled-task-dialog-data";
import { useScheduledTaskDialogScheduleState } from "./use-scheduled-task-dialog-schedule";

export function useScheduledTaskDialogState({
  agent_id,
  initial_task,
  is_open,
  on_close,
  on_created,
  on_saved,
}: {
  agent_id: string;
  initial_task?: ScheduledTaskItem | null;
  is_open: boolean;
  on_close: () => void;
  on_created?: (task: ScheduledTaskItem) => void | Promise<void>;
  on_saved?: (task: ScheduledTaskItem) => void | Promise<void>;
}) {
  const name_ref = useRef<HTMLInputElement>(null);
  const [task_name, set_task_name] = useState("");
  const [target_type, set_target_type_state] = useState<TargetType>("agent");
  const [selected_agent_id, set_selected_agent_id_state] = useState(agent_id);
  const [selected_room_id, set_selected_room_id_state] = useState("");
  const [execution_mode, set_execution_mode_state] = useState<ExecutionMode>("existing");
  const [selected_session_key, set_selected_session_key_state] = useState("");
  const [reply_mode, set_reply_mode] = useState<ReplyMode>("execution");
  const [selected_reply_session_key, set_selected_reply_session_key_state] = useState("");
  const [dedicated_session_key, set_dedicated_session_key] = useState("");
  const [timezone, set_timezone] = useState(get_default_timezone());
  const [enabled, set_enabled] = useState(true);
  const [instruction, set_instruction] = useState("");
  const [error_message, set_error_message] = useState<string | null>(null);
  const [is_submitting, set_is_submitting] = useState(false);
  const daily_picker_anchor_ref = useRef<HTMLButtonElement>(null);
  const single_picker_anchor_ref = useRef<HTMLButtonElement>(null);

  const schedule = useScheduledTaskDialogScheduleState(timezone);
  const hydrate_schedule = schedule.hydrate;
  const reset_schedule = schedule.reset;

  const reset_context_selection = useCallback(() => {
    set_selected_session_key_state("");
    set_selected_reply_session_key_state("");
    set_error_message(null);
  }, []);

  const set_target_type = useCallback((value: TargetType) => {
    set_target_type_state(value);
    reset_context_selection();
  }, [reset_context_selection]);

  const set_selected_agent_id = useCallback((value: string) => {
    set_selected_agent_id_state(value);
    reset_context_selection();
  }, [reset_context_selection]);

  const set_selected_room_id = useCallback((value: string) => {
    set_selected_room_id_state(value);
    reset_context_selection();
  }, [reset_context_selection]);

  const set_selected_session_key = useCallback((value: string) => {
    set_selected_session_key_state(value);
    set_error_message(null);
  }, []);

  const set_selected_reply_session_key = useCallback((value: string) => {
    set_selected_reply_session_key_state(value);
    set_error_message(null);
  }, []);

  const set_execution_mode = useCallback((value: ExecutionMode) => {
    set_execution_mode_state(value);
    if (value === "main") {
      set_reply_mode("none");
      set_selected_reply_session_key_state("");
    }
    set_error_message(null);
  }, []);

  const data = useScheduledTaskDialogData({
    is_open,
    target_type,
    selected_agent_id,
    selected_room_id,
  });

  const selected_session = data.session_options.find((option) => option.session_key === selected_session_key) ?? null;
  const selected_reply_session = data.session_options.find((option) => option.session_key === selected_reply_session_key) ?? null;

  const apply_dialog_initial_state = useCallback(() => {
    const next_state = initial_task
      ? build_task_dialog_initial_state(agent_id, initial_task)
      : build_default_dialog_initial_state(agent_id);

    set_task_name(next_state.task_name);
    set_target_type_state(next_state.target_type);
    set_selected_agent_id_state(next_state.selected_agent_id);
    set_selected_room_id_state(next_state.selected_room_id);
    set_execution_mode_state(next_state.execution_mode);
    set_selected_session_key_state(next_state.selected_session_key);
    set_reply_mode(next_state.reply_mode);
    set_selected_reply_session_key_state(next_state.selected_reply_session_key);
    set_dedicated_session_key(next_state.dedicated_session_key);
    set_timezone(next_state.timezone);
    set_enabled(next_state.enabled);
    set_instruction(next_state.instruction);
    set_error_message(null);
    set_is_submitting(false);

    if (initial_task && next_state.schedule_snapshot) {
      hydrate_schedule(next_state.schedule_snapshot);
      return;
    }
    reset_schedule();
  }, [agent_id, hydrate_schedule, initial_task, reset_schedule]);

  function build_submit_state(): ScheduledTaskDialogSubmitState {
    return {
      task_name,
      target_type,
      selected_agent_id,
      selected_room_id,
      execution_mode,
      selected_session_key,
      reply_mode,
      selected_reply_session_key,
      dedicated_session_key,
      timezone,
      enabled,
      instruction,
      every_value: schedule.every_value,
      every_unit: schedule.every_unit,
      daily_time: schedule.daily_time,
      selected_weekdays: schedule.selected_weekdays,
      run_at: schedule.run_at,
      selected_session,
      selected_reply_session,
      agent_options: data.agent_options,
      room_options: data.room_options,
      schedule_kind: schedule.schedule_kind,
    };
  }

  function is_room_executor_selection_required() {
    return target_type === "room" && execution_mode !== "existing";
  }

  async function handle_submit() {
    const submit_state = build_submit_state();
    const validation_error = get_scheduled_task_validation_error(submit_state);
    if (validation_error) {
      set_error_message(validation_error);
      return;
    }

    set_is_submitting(true);
    set_error_message(null);
    try {
      const payload = build_scheduled_task_payload(submit_state, initial_task?.source);
      if (initial_task) {
        const updated = await update_scheduled_task_api(initial_task.job_id, payload);
        await on_saved?.(updated);
      } else {
        const created = await create_scheduled_task_api(payload);
        await on_created?.(created);
      }
      on_close();
    } catch (error) {
      set_error_message(error instanceof Error ? error.message : "创建任务失败");
    } finally {
      set_is_submitting(false);
    }
  }

  useEffect(() => {
    if (is_open && name_ref.current) {
      name_ref.current.focus();
    }
  }, [is_open]);

  useEffect(() => {
    const on_key_down = (event: KeyboardEvent) => {
      if (!is_open) {
        return;
      }
      close_on_escape(event, on_close);
    };
    window.addEventListener("keydown", on_key_down);
    return () => window.removeEventListener("keydown", on_key_down);
  }, [is_open, on_close]);

  useEffect(() => {
    if (!is_open) {
      return;
    }
    apply_dialog_initial_state();
  }, [apply_dialog_initial_state, is_open]);

  return {
    ...schedule,
    ...data,
    name_ref,
    task_name,
    set_task_name,
    target_type,
    set_target_type,
    selected_agent_id,
    set_selected_agent_id,
    selected_room_id,
    set_selected_room_id,
    execution_mode,
    set_execution_mode,
    selected_session_key,
    set_selected_session_key,
    reply_mode,
    set_reply_mode,
    selected_reply_session_key,
    set_selected_reply_session_key,
    dedicated_session_key,
    set_dedicated_session_key,
    enabled,
    set_enabled,
    timezone,
    set_timezone,
    instruction,
    set_instruction,
    error_message,
    set_error_message,
    is_submitting,
    daily_picker_anchor_ref,
    single_picker_anchor_ref,
    selected_session,
    selected_reply_session,
    is_room_executor_selection_required,
    handle_submit,
  };
}
