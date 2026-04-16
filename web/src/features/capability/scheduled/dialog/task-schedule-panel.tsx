"use client";

import { DailyTimePicker } from "../pickers/daily-time-picker";
import { SingleRunPicker } from "../pickers/single-run-picker";
import { WEEKDAY_OPTIONS } from "../pickers/picker-types";
import type { EveryUnit } from "./scheduled-task-dialog-types";
import {
  COMPACT_SELECT_CLASS_NAME,
  COMPACT_STEPPER_CLASS_NAME,
  get_schedule_tab_class_name,
  get_weekday_pill_class_name,
  type TaskSchedulePanelProps,
} from "./task-schedule-panel-model";

export function TaskSchedulePanel(props: TaskSchedulePanelProps) {
  const {
    close_daily_picker,
    close_single_picker,
    daily_anchor_ref,
    daily_display,
    daily_hour12,
    daily_meridiem,
    daily_minute,
    enabled,
    error_message,
    every_unit,
    every_unit_options,
    every_value,
    instruction,
    is_daily_picker_open,
    is_single_picker_open,
    is_single_date_disabled,
    is_single_hour_disabled,
    is_single_meridiem_disabled,
    is_single_minute_disabled,
    is_single_second_disabled,
    on_daily_hour_select,
    on_daily_meridiem_select,
    on_daily_minute_select,
    on_daily_trigger_click,
    on_next_month,
    on_prev_month,
    on_single_date_select,
    on_single_hour_select,
    on_single_meridiem_select,
    on_single_minute_select,
    on_single_second_select,
    on_single_trigger_click,
    on_toggle_weekday,
    run_at_display,
    schedule_kind,
    schedule_options,
    selected_run_date,
    selected_weekdays,
    set_enabled,
    set_every_unit,
    set_every_value,
    set_instruction,
    set_schedule_kind,
    set_timezone,
    single_anchor_ref,
    single_hour12,
    single_meridiem,
    single_minute,
    single_picker_days,
    single_picker_month,
    single_second,
    timezone,
    timezone_options,
  } = props;

  return (
    <div className="flex min-w-0 flex-col gap-4">
      <div className="dialog-field">
        <div className="flex items-center justify-between gap-4">
          <span className="dialog-label !mb-0">调度</span>
          <div className="inline-flex shrink-0 items-center rounded-[14px] bg-(--surface-panel-subtle-background) p-1">
            {schedule_options.map((opt) => (
              <button
                className={get_schedule_tab_class_name(schedule_kind === opt.key)}
                key={opt.key}
                onClick={() => set_schedule_kind(opt.key)}
                type="button"
              >
                {opt.label}
              </button>
            ))}
          </div>
        </div>
      </div>

      {schedule_kind === "at" ? (
        <SingleRunPicker
          anchor_ref={single_anchor_ref}
          display={run_at_display}
          hour12={single_hour12}
          is_date_disabled={is_single_date_disabled}
          is_hour_disabled={is_single_hour_disabled}
          is_open={is_single_picker_open}
          is_meridiem_disabled={is_single_meridiem_disabled}
          is_minute_disabled={is_single_minute_disabled}
          is_second_disabled={is_single_second_disabled}
          meridiem={single_meridiem}
          minute={single_minute}
          month_label={`${single_picker_month.replace("-", "年")}月`}
          on_close={close_single_picker}
          on_date_select={on_single_date_select}
          on_hour_select={on_single_hour_select}
          on_meridiem_select={on_single_meridiem_select}
          on_minute_select={on_single_minute_select}
          on_next_month={on_next_month}
          on_prev_month={on_prev_month}
          on_second_select={on_single_second_select}
          on_toggle={on_single_trigger_click}
          second={single_second}
          selected_date={selected_run_date}
          visible_days={single_picker_days}
        />
      ) : null}

      {schedule_kind === "cron" ? (
        <div className="grid gap-4">
          <DailyTimePicker
            anchor_ref={daily_anchor_ref}
            display={daily_display}
            hour12={daily_hour12}
            is_open={is_daily_picker_open}
            meridiem={daily_meridiem}
            minute={daily_minute}
            on_close={close_daily_picker}
            on_hour_select={on_daily_hour_select}
            on_meridiem_select={on_daily_meridiem_select}
            on_minute_select={on_daily_minute_select}
            on_toggle={on_daily_trigger_click}
          />
          <div className="dialog-field">
            <span className="dialog-label">执行日</span>
            <div className="flex flex-wrap gap-2">
              {WEEKDAY_OPTIONS.map((option) => {
                const is_selected = selected_weekdays.includes(option.key);
                return (
                  <button
                    className={get_weekday_pill_class_name(is_selected)}
                    key={option.key}
                    onClick={() => on_toggle_weekday(option.key)}
                    type="button"
                  >
                    {option.short_label}
                  </button>
                );
              })}
            </div>
            <p className="text-xs leading-5 text-(--text-muted)">
              选中的日期会在这个时间执行；全选就是每天执行。
            </p>
          </div>
        </div>
      ) : null}

      {schedule_kind === "every" ? (
        <div className="rounded-[18px] border border-(--divider-subtle-color) bg-white/35 px-4 py-4">
          <div className="flex flex-wrap items-center gap-3">
            <span className="text-sm font-semibold text-(--text-default)">每隔</span>
            <input
              className={`${COMPACT_STEPPER_CLASS_NAME} min-w-[96px]`}
              id="task-every-value"
              max="999"
              min="1"
              onChange={(e) => set_every_value(e.target.value)}
              step="1"
              type="number"
              value={every_value}
            />
            <select
              className={`${COMPACT_SELECT_CLASS_NAME} min-w-[132px]`}
              id="task-every-unit"
              onChange={(e) => set_every_unit(e.target.value as EveryUnit)}
              value={every_unit}
            >
              {every_unit_options.map((option) => (
                <option key={option.key} value={option.key}>
                  {option.label}
                </option>
              ))}
            </select>
          </div>
        </div>
      ) : null}

      <div className="dialog-field">
        <label className="dialog-label" htmlFor="task-timezone">
          时区
        </label>
        <select
          className={COMPACT_SELECT_CLASS_NAME}
          id="task-timezone"
          onChange={(e) => set_timezone(e.target.value)}
          value={timezone}
        >
          {timezone_options.map((option) => (
            <option key={option} value={option}>
              {option}
            </option>
          ))}
        </select>
      </div>

      <div className="dialog-field">
        <label className="dialog-label" htmlFor="task-instruction">
          任务指令
        </label>
        <textarea
          className="dialog-input radius-shell-sm w-full resize-none px-4 py-2.5 text-sm text-foreground placeholder:text-muted-foreground/50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/20"
          id="task-instruction"
          onChange={(e) => set_instruction(e.target.value)}
          placeholder="输入 Agent 需要执行的指令"
          rows={4}
          value={instruction}
        />
      </div>

      <label className="flex items-center gap-3 rounded-[18px] border border-(--divider-subtle-color) bg-white/45 px-4 py-3 text-sm text-(--text-default)">
        <input
          checked={enabled}
          className="h-4 w-4"
          onChange={(e) => set_enabled(e.target.checked)}
          type="checkbox"
        />
        创建后立即启用任务
      </label>

      {error_message ? (
        <div className="rounded-[18px] border border-[color:color-mix(in_srgb,var(--destructive)_15%,transparent)] bg-[color:color-mix(in_srgb,var(--destructive)_6%,transparent)] px-4 py-3 text-sm text-(--destructive)">
          {error_message}
        </div>
      ) : null}
    </div>
  );
}
