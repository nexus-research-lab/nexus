"use client";

import { createPortal } from "react-dom";
import { Pencil, X } from "lucide-react";

import {
  DIALOG_ICON_BUTTON_CLASS_NAME,
  get_dialog_action_class_name,
} from "@/shared/ui/dialog/dialog-styles";
import type { ScheduledTaskItem } from "@/types/scheduled-task";

import { TaskBasicsPanel } from "./dialog/task-basics-panel";
import { TaskSchedulePanel } from "./dialog/task-schedule-panel";
import { useScheduledTaskDialogState } from "./dialog/use-scheduled-task-dialog-state";
import {
  EVERY_UNIT_OPTIONS,
  EXECUTION_MODE_OPTIONS,
  REPLY_MODE_OPTIONS,
  SCHEDULE_OPTIONS,
  TARGET_TYPE_OPTIONS,
  TIMEZONE_OPTIONS,
} from "./dialog/scheduled-task-dialog-constants";

interface ScheduledTaskDialogProps {
  agent_id: string;
  is_open: boolean;
  on_close: () => void;
  initial_task?: ScheduledTaskItem | null;
  on_created?: (task: ScheduledTaskItem) => void | Promise<void>;
  on_saved?: (task: ScheduledTaskItem) => void | Promise<void>;
}

export function ScheduledTaskDialog({
  agent_id,
  is_open,
  initial_task = null,
  on_close,
  on_created,
  on_saved,
}: ScheduledTaskDialogProps) {
  const state = useScheduledTaskDialogState({
    agent_id,
    initial_task,
    is_open,
    on_close,
    on_created,
    on_saved,
  });

  if (!is_open) return null;

  return createPortal(
    <>
      <div
        aria-hidden="true"
        className="dialog-backdrop z-[9998] animate-in fade-in duration-(--motion-duration-fast)"
        onClick={on_close}
      />
      <div
        data-modal-root="true"
        aria-labelledby="create-task-dialog-title"
        aria-modal="true"
        className="fixed inset-0 z-[9999] flex items-center justify-center p-6"
        role="dialog"
        onPointerDown={(event) => event.stopPropagation()}
        onPointerMove={(event) => event.stopPropagation()}
        onPointerUp={(event) => event.stopPropagation()}
      >
        <div className="dialog-shell radius-shell-lg w-full max-w-[1120px] animate-in zoom-in-95 duration-(--motion-duration-fast)">
          <div className="dialog-header">
            <div className="min-w-0 flex-1">
              <h3 className="dialog-title" id="create-task-dialog-title">
                {initial_task ? "编辑任务" : "新建任务"}
              </h3>
              <p className="dialog-subtitle">
                {initial_task ? "修改调度、执行会话和结果回传方式。" : "先选目标对象，再决定执行会话和结果回传方式。"}
              </p>
            </div>
            <button
              aria-label="关闭"
              className={DIALOG_ICON_BUTTON_CLASS_NAME}
              onClick={on_close}
              type="button"
            >
              <X className="h-4 w-4" />
            </button>
          </div>

          <div className="dialog-body grid grid-cols-1 gap-6 md:grid-cols-2 md:items-start">
            <TaskBasicsPanel
              agent_options={state.agent_options}
              agents_error={state.agents_error}
              agents_loading={state.agents_loading}
              dedicated_session_key={state.dedicated_session_key}
              execution_mode={state.execution_mode}
              execution_mode_options={EXECUTION_MODE_OPTIONS}
              name_ref={state.name_ref}
              on_reset_context_error={() => state.set_error_message(null)}
              reply_mode={state.reply_mode}
              reply_mode_options={REPLY_MODE_OPTIONS}
              room_options={state.room_options}
              rooms_error={state.rooms_error}
              rooms_loading={state.rooms_loading}
              selected_agent_id={state.selected_agent_id}
              selected_reply_session_key={state.selected_reply_session_key}
              selected_room_id={state.selected_room_id}
              selected_session_key={state.selected_session_key}
              session_empty_message={
                state.target_type === "agent"
                  ? state.selected_agent_id && !state.agent_sessions_loading && state.session_options.length === 0
                    ? "这个智能体没有可选会话"
                    : null
                  : state.selected_room_id && !state.room_contexts_loading && state.session_options.length === 0
                    ? "这个 Room 没有可选会话"
                    : null
              }
              session_error={state.target_type === "agent" ? state.agent_sessions_error : state.room_contexts_error}
              session_loading={state.target_type === "agent" ? state.agent_sessions_loading : state.room_contexts_loading}
              session_options={state.session_options}
              set_dedicated_session_key={state.set_dedicated_session_key}
              set_execution_mode={state.set_execution_mode}
              set_reply_mode={state.set_reply_mode}
              set_selected_agent_id={state.set_selected_agent_id}
              set_selected_reply_session_key={state.set_selected_reply_session_key}
              set_selected_room_id={state.set_selected_room_id}
              set_selected_session_key={state.set_selected_session_key}
              set_target_type={state.set_target_type}
              set_task_name={state.set_task_name}
              target_type={state.target_type}
              target_type_options={TARGET_TYPE_OPTIONS}
              task_name={state.task_name}
              require_session_selection={state.execution_mode === "existing" || state.is_room_executor_selection_required()}
            />

            <TaskSchedulePanel
              close_daily_picker={() => state.set_is_daily_picker_open(false)}
              close_single_picker={() => state.set_is_single_picker_open(false)}
              daily_anchor_ref={state.daily_picker_anchor_ref}
              daily_display={state.daily_display}
              daily_hour12={state.daily_meridiem_parts.hour12}
              daily_meridiem={state.daily_meridiem_parts.meridiem}
              daily_minute={state.daily_meridiem_parts.minute}
              enabled={state.enabled}
              error_message={state.error_message}
              every_unit={state.every_unit}
              every_unit_options={EVERY_UNIT_OPTIONS}
              every_value={state.every_value}
              instruction={state.instruction}
              is_daily_picker_open={state.is_daily_picker_open}
              is_single_picker_open={state.is_single_picker_open}
              is_single_date_disabled={state.is_single_date_disabled}
              is_single_hour_disabled={state.is_single_hour_disabled}
              is_single_meridiem_disabled={state.is_single_meridiem_disabled}
              is_single_minute_disabled={state.is_single_minute_disabled}
              is_single_second_disabled={state.is_single_second_disabled}
              on_daily_hour_select={(value) => state.update_daily_picker({ hour12: value })}
              on_daily_meridiem_select={(value) => state.update_daily_picker({ meridiem: value })}
              on_daily_minute_select={(value) => state.update_daily_picker({ minute: value })}
              on_daily_trigger_click={() => {
                state.set_is_daily_picker_open((value) => !value);
                state.set_is_single_picker_open(false);
              }}
              on_next_month={state.go_to_next_month}
              on_prev_month={state.go_to_prev_month}
              on_single_date_select={(value) => state.update_single_picker({ date: value })}
              on_single_hour_select={(value) => state.update_single_picker({ hour12: value })}
              on_single_meridiem_select={(value) => state.update_single_picker({ meridiem: value })}
              on_single_minute_select={(value) => state.update_single_picker({ minute: value })}
              on_single_second_select={(value) => state.update_single_picker({ second: value })}
              on_single_trigger_click={() => {
                state.sync_single_picker_to_now();
                state.set_is_single_picker_open((value) => !value);
                state.set_is_daily_picker_open(false);
              }}
              on_toggle_weekday={state.toggle_weekday}
              run_at_display={state.run_at_display}
              schedule_kind={state.schedule_kind}
              schedule_options={SCHEDULE_OPTIONS}
              selected_run_date={state.run_at_parts.date}
              selected_weekdays={state.selected_weekdays}
              set_enabled={state.set_enabled}
              set_every_unit={state.set_every_unit}
              set_every_value={state.set_every_value}
              set_instruction={state.set_instruction}
              set_schedule_kind={state.set_schedule_kind}
              set_timezone={state.set_timezone}
              single_anchor_ref={state.single_picker_anchor_ref}
              single_hour12={state.single_meridiem_parts.hour12}
              single_meridiem={state.single_meridiem_parts.meridiem}
              single_minute={state.single_meridiem_parts.minute}
              single_picker_days={state.single_picker_days}
              single_picker_month={state.single_picker_month}
              single_second={state.single_meridiem_parts.second}
              timezone={state.timezone}
              timezone_options={TIMEZONE_OPTIONS}
            />
          </div>

          <div className="dialog-footer">
            <button
              className={get_dialog_action_class_name("default")}
              disabled={state.is_submitting}
              onClick={on_close}
              type="button"
            >
              取消
            </button>
            <button
              className={get_dialog_action_class_name("primary")}
              disabled={state.is_submitting}
              onClick={() => void state.handle_submit()}
              type="button"
            >
              {state.is_submitting ? (initial_task ? "保存中" : "创建中") : (
                <>
                  {initial_task ? <Pencil className="h-3.5 w-3.5" /> : null}
                  {initial_task ? "保存修改" : "创建"}
                </>
              )}
            </button>
          </div>
        </div>
      </div>
    </>,
    document.body,
  );
}
