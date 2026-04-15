"use client";

import { useCallback, useEffect, useRef, useState } from "react";

import { create_scheduled_task_api, update_scheduled_task_api } from "@/lib/api/scheduled-task-api";
import type {
  ScheduledTaskItem,
  ScheduledTaskSchedule,
  ScheduledTaskSessionTarget,
  ScheduledTaskSource,
} from "@/types/capability/scheduled-task";

import {
  build_daily_cron_expression,
  type ExecutionMode,
  get_default_timezone,
  parse_daily_cron_expression,
  type ReplyMode,
  type TargetType,
  to_interval_seconds,
} from "./scheduled-task-dialog-constants";
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
  const [execution_mode, set_execution_mode] = useState<ExecutionMode>("existing");
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

  const schedule = useScheduledTaskDialogScheduleState();

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

  const data = useScheduledTaskDialogData({
    is_open,
    target_type,
    selected_agent_id,
    selected_room_id,
  });

  useEffect(() => {
    if (is_open && name_ref.current) {
      name_ref.current.focus();
    }
  }, [is_open]);

  useEffect(() => {
    const handle_key_down = (e: KeyboardEvent) => {
      if (!is_open) return;
      if (e.key === "Escape") {
        e.preventDefault();
        on_close();
      }
    };
    window.addEventListener("keydown", handle_key_down);
    return () => window.removeEventListener("keydown", handle_key_down);
  }, [is_open, on_close]);

  useEffect(() => {
    if (!is_open) {
      return;
    }
    if (!initial_task) {
      set_task_name("");
      set_target_type_state("agent");
      set_selected_agent_id_state(agent_id);
      set_selected_room_id_state("");
      set_execution_mode("existing");
      set_selected_session_key_state("");
      set_reply_mode("execution");
      set_selected_reply_session_key_state("");
      set_dedicated_session_key("");
      set_timezone(get_default_timezone());
      set_enabled(true);
      set_instruction("");
      set_error_message(null);
      set_is_submitting(false);
      schedule.reset();
      return;
    }

    set_task_name(initial_task.name);
    set_target_type_state(initial_task.source?.context_type === "room" ? "room" : "agent");
    set_selected_agent_id_state(initial_task.source?.context_type === "agent"
      ? (initial_task.source.context_id || initial_task.agent_id)
      : initial_task.agent_id);
    set_selected_room_id_state(initial_task.source?.context_type === "room" ? (initial_task.source.context_id || "") : "");
    set_execution_mode(
      initial_task.session_target.kind === "named"
        ? "dedicated"
        : initial_task.session_target.kind === "isolated"
          ? "temporary"
          : "existing",
    );
    set_selected_session_key_state(
      initial_task.session_target.kind === "bound"
        ? initial_task.session_target.bound_session_key
        : (initial_task.source?.session_key || ""),
    );
    set_reply_mode(
      initial_task.delivery.mode === "none"
        ? "none"
        : initial_task.delivery.mode === "explicit"
          && initial_task.delivery.to
          && initial_task.source?.session_key
          && initial_task.delivery.to !== initial_task.source.session_key
          ? "selected"
          : "execution",
    );
    set_selected_reply_session_key_state(
      initial_task.delivery.mode === "explicit"
        && initial_task.delivery.to
        && initial_task.delivery.to !== initial_task.source?.session_key
        ? initial_task.delivery.to
        : "",
    );
    set_dedicated_session_key(
      initial_task.session_target.kind === "named" ? initial_task.session_target.named_session_key : "",
    );
    set_timezone(initial_task.schedule.timezone?.trim() || get_default_timezone());
    set_enabled(initial_task.enabled);
    set_instruction(initial_task.instruction);
    set_error_message(null);
    set_is_submitting(false);

    if (initial_task.schedule.kind === "every") {
      const interval_seconds = initial_task.schedule.interval_seconds;
      if (interval_seconds % 3600 === 0) {
        schedule.hydrate({
          schedule_kind: "every",
          every_value: String(interval_seconds / 3600),
          every_unit: "hours",
        });
      } else if (interval_seconds % 60 === 0) {
        schedule.hydrate({
          schedule_kind: "every",
          every_value: String(interval_seconds / 60),
          every_unit: "minutes",
        });
      } else {
        schedule.hydrate({
          schedule_kind: "every",
          every_value: String(interval_seconds),
          every_unit: "seconds",
        });
      }
      return;
    }

    if (initial_task.schedule.kind === "cron") {
      const parsed_cron = parse_daily_cron_expression(initial_task.schedule.cron_expression);
      schedule.hydrate({
        schedule_kind: "cron",
        daily_time: parsed_cron?.daily_time,
        selected_weekdays: parsed_cron?.selected_weekdays,
      });
      return;
    }

    schedule.hydrate({
      schedule_kind: "at",
      run_at: initial_task.schedule.run_at.replace("Z", "").slice(0, 19),
    });
  }, [agent_id, initial_task, is_open]); // eslint-disable-line react-hooks/exhaustive-deps

  const selected_session = data.session_options.find((option) => option.session_key === selected_session_key) ?? null;
  const selected_reply_session = data.session_options.find((option) => option.session_key === selected_reply_session_key) ?? null;

  function is_room_executor_selection_required() {
    return target_type === "room" && execution_mode !== "existing";
  }

  function build_session_target(): ScheduledTaskSessionTarget {
    if (execution_mode === "temporary") {
      return { kind: "isolated", wake_mode: "next-heartbeat" };
    }
    if (execution_mode === "dedicated") {
      return { kind: "named", named_session_key: dedicated_session_key.trim(), wake_mode: "next-heartbeat" };
    }
    if (!selected_session) {
      throw new Error("请选择执行会话");
    }
    return { kind: "bound", bound_session_key: selected_session.session_key, wake_mode: "next-heartbeat" };
  }

  function build_delivery() {
    if (reply_mode === "none") return { mode: "none" as const };
    if (reply_mode === "execution") {
      if (execution_mode === "existing") {
        if (!selected_session) throw new Error("请选择执行会话");
        return { mode: "explicit" as const, channel: "websocket", to: selected_session.session_key };
      }
      if (target_type === "room") {
        if (!selected_session) throw new Error("请选择一个 Room 会话作为回传目标");
        return { mode: "explicit" as const, channel: "websocket", to: selected_session.session_key };
      }
      return { mode: "none" as const };
    }
    if (!selected_reply_session) throw new Error("请选择回复会话");
    return { mode: "explicit" as const, channel: "websocket", to: selected_reply_session.session_key };
  }

  function resolve_agent_id_for_task() {
    if (target_type === "agent") return selected_agent_id.trim();
    if (!selected_session) throw new Error("请选择一个 Room 会话来确定执行智能体");
    return selected_session.agent_id;
  }

  function build_schedule(): ScheduledTaskSchedule {
    if (schedule.schedule_kind === "every") {
      const interval_seconds = to_interval_seconds(schedule.every_value, schedule.every_unit);
      if (interval_seconds === null) throw new Error("循环间隔必须是大于 0 的整数");
      return { kind: "every", interval_seconds, timezone: timezone.trim() || "Asia/Shanghai" };
    }
    if (schedule.schedule_kind === "cron") {
      const cron_expression = build_daily_cron_expression(schedule.daily_time, schedule.selected_weekdays);
      if (!cron_expression) throw new Error("请选择有效的固定执行时间");
      return { kind: "cron", cron_expression, timezone: timezone.trim() || "Asia/Shanghai" };
    }
    return { kind: "at", run_at: schedule.run_at.trim(), timezone: timezone.trim() || "Asia/Shanghai" };
  }

  function build_source_snapshot(
    source_session: { session_key: string; label: string } | null,
    original_source?: ScheduledTaskSource | null,
  ): ScheduledTaskSource {
    const selected_agent = data.agent_options.find((option) => option.value === selected_agent_id);
    const selected_room = data.room_options.find((option) => option.value === selected_room_id);
    return {
      kind: original_source?.kind ?? "user_page",
      creator_agent_id: original_source?.creator_agent_id ?? null,
      context_type: target_type,
      context_id: target_type === "agent" ? selected_agent_id.trim() : selected_room_id.trim(),
      context_label: target_type === "agent"
        ? (selected_agent?.label || selected_agent_id.trim())
        : (selected_room?.label || selected_room_id.trim()),
      session_key: source_session?.session_key ?? null,
      session_label: source_session?.label ?? null,
    };
  }

  function get_validation_error(): string | null {
    if (!task_name.trim()) return "请输入任务名称";
    if (!instruction.trim()) return "请输入任务指令";
    if (target_type === "agent") {
      if (!selected_agent_id.trim()) return "请选择智能体";
    } else if (!selected_room_id.trim()) return "请选择 Room";
    if (execution_mode === "existing" && !selected_session_key.trim()) return "请选择执行会话";
    if (is_room_executor_selection_required() && !selected_session_key.trim()) return "请选择一个 Room 会话来确定执行智能体";
    if (execution_mode === "dedicated" && !dedicated_session_key.trim()) return "请输入专用长期会话名称";
    if (reply_mode === "selected" && !selected_reply_session_key.trim()) return "请选择回复会话";
    if (schedule.schedule_kind === "every" && to_interval_seconds(schedule.every_value, schedule.every_unit) === null) {
      return "循环间隔必须是大于 0 的整数";
    }
    if (schedule.schedule_kind === "cron" && !build_daily_cron_expression(schedule.daily_time, schedule.selected_weekdays)) {
      return schedule.selected_weekdays.length === 0 ? "请至少选择一个执行日" : "请选择有效的固定执行时间";
    }
    if (schedule.schedule_kind === "at") {
      if (!schedule.run_at.trim()) return "请选择有效的执行时间";
      if (new Date(schedule.run_at).getTime() <= Date.now()) {
        return "单次执行时间必须晚于当前时间";
      }
    }
    return null;
  }

  async function handle_submit() {
    const validation_error = get_validation_error();
    if (validation_error) {
      set_error_message(validation_error);
      return;
    }
    set_is_submitting(true);
    set_error_message(null);
    try {
      const source_session = selected_session ?? selected_reply_session;
      const payload = {
        name: task_name.trim(),
        schedule: build_schedule(),
        instruction: instruction.trim(),
        session_target: build_session_target(),
        delivery: build_delivery(),
        enabled,
      };

      if (initial_task) {
        const updated = await update_scheduled_task_api(initial_task.job_id, {
          ...payload,
          agent_id: resolve_agent_id_for_task(),
          source: build_source_snapshot(source_session, initial_task.source),
        });
        await on_saved?.(updated);
      } else {
        const created = await create_scheduled_task_api({
          ...payload,
          agent_id: resolve_agent_id_for_task(),
          source: build_source_snapshot(source_session),
        });
        await on_created?.(created);
      }
      on_close();
    } catch (error) {
      set_error_message(error instanceof Error ? error.message : "创建任务失败");
    } finally {
      set_is_submitting(false);
    }
  }

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
