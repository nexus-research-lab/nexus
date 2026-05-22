"use client";

import { UiChoiceButton } from "@/shared/ui/choice";
import { UiCheckboxRow } from "@/shared/ui/checkbox-row";
import { UiInput, UiTextarea } from "@/shared/ui/form-control";
import { UiPanel } from "@/shared/ui/panel";
import { UiSegmentedControl } from "@/shared/ui/segmented-control";
import { UiSelectMenu } from "@/shared/ui/select-menu";
import { UiStateBlock } from "@/shared/ui/state-block";

import { DailyTimePicker } from "../pickers/daily-time-picker";
import { SingleRunPicker } from "../pickers/single-run-picker";
import { WEEKDAY_OPTIONS } from "../pickers/picker-types";
import type { EveryUnit } from "./scheduled-task-dialog-types";
import {
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
          <UiSegmentedControl
            class_name="shrink-0"
            on_change={set_schedule_kind}
            options={schedule_options.map((option) => ({
              label: option.label,
              value: option.key,
            }))}
            title="调度"
            value={schedule_kind}
          />
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
                  <UiChoiceButton
                    active={is_selected}
                    choice_size="md"
                    class_name="min-w-9 px-3"
                    key={option.key}
                    onClick={() => on_toggle_weekday(option.key)}
                    shape="pill"
                  >
                    {option.short_label}
                  </UiChoiceButton>
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
        <UiPanel padding="md" variant="inset">
          <div className="flex flex-wrap items-center gap-3">
            <span className="text-sm font-semibold text-(--text-default)">每隔</span>
            <UiInput
              class_name="min-w-[96px]"
              control_size="lg"
              id="task-every-value"
              max="999"
              min="1"
              onChange={(e) => set_every_value(e.target.value)}
              step="1"
              type="number"
              value={every_value}
            />
            <UiSelectMenu
              aria_label="选择间隔单位"
              class_name="min-w-[132px]"
              id="task-every-unit"
              on_change={(value) => set_every_unit(value as EveryUnit)}
              options={every_unit_options.map((option) => ({
                value: option.key,
                label: option.label,
              }))}
              value={every_unit}
            />
          </div>
        </UiPanel>
      ) : null}

      <div className="dialog-field">
        <label className="dialog-label" htmlFor="task-timezone">
          时区
        </label>
        <UiSelectMenu
          aria_label="选择任务时区"
          id="task-timezone"
          on_change={set_timezone}
          options={timezone_options.map((option) => ({
            value: option,
            label: option,
          }))}
          value={timezone}
        />
      </div>

      <div className="dialog-field">
        <label className="dialog-label" htmlFor="task-instruction">
          任务指令
        </label>
        <UiTextarea
          class_name="resize-none"
          id="task-instruction"
          onChange={(e) => set_instruction(e.target.value)}
          placeholder="输入 Agent 需要执行的指令"
          rows={4}
          value={instruction}
        />
      </div>

      <UiCheckboxRow
        checked={enabled}
        label="创建后立即启用任务"
        on_change={set_enabled}
      />

      {error_message ? (
        <UiStateBlock description={error_message} size="sm" title="任务配置无效" tone="danger" />
      ) : null}
    </div>
  );
}
