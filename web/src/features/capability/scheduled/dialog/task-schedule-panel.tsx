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
    close_daily_picker: closeDailyPicker,
    close_single_picker: closeSinglePicker,
    daily_anchor_ref: dailyAnchorRef,
    daily_display: dailyDisplay,
    daily_hour12: dailyHour12,
    daily_meridiem: dailyMeridiem,
    daily_minute: dailyMinute,
    enabled,
    error_message: errorMessage,
    every_unit: everyUnit,
    every_unit_options: everyUnitOptions,
    every_value: everyValue,
    instruction,
    instruction_label: instructionLabel,
    instruction_placeholder: instructionPlaceholder,
    is_daily_picker_open: isDailyPickerOpen,
    is_single_picker_open: isSinglePickerOpen,
    is_single_date_disabled: isSingleDateDisabled,
    is_single_hour_disabled: isSingleHourDisabled,
    is_single_meridiem_disabled: isSingleMeridiemDisabled,
    is_single_minute_disabled: isSingleMinuteDisabled,
    is_single_second_disabled: isSingleSecondDisabled,
    on_daily_hour_select: onDailyHourSelect,
    on_daily_meridiem_select: onDailyMeridiemSelect,
    on_daily_minute_select: onDailyMinuteSelect,
    on_daily_trigger_click: onDailyTriggerClick,
    on_next_month: onNextMonth,
    on_prev_month: onPrevMonth,
    on_single_date_select: onSingleDateSelect,
    on_single_hour_select: onSingleHourSelect,
    on_single_meridiem_select: onSingleMeridiemSelect,
    on_single_minute_select: onSingleMinuteSelect,
    on_single_second_select: onSingleSecondSelect,
    on_single_trigger_click: onSingleTriggerClick,
    on_toggle_weekday: onToggleWeekday,
    run_at_display: runAtDisplay,
    schedule_kind: scheduleKind,
    schedule_options: scheduleOptions,
    selected_run_date: selectedRunDate,
    selected_weekdays: selectedWeekdays,
    set_enabled: setEnabled,
    set_every_unit: setEveryUnit,
    set_every_value: setEveryValue,
    set_instruction: setInstruction,
    set_schedule_kind: setScheduleKind,
    set_timezone: setTimezone,
    single_anchor_ref: singleAnchorRef,
    single_hour12: singleHour12,
    single_meridiem: singleMeridiem,
    single_minute: singleMinute,
    single_picker_days: singlePickerDays,
    single_picker_month: singlePickerMonth,
    single_second: singleSecond,
    timezone,
    timezone_options: timezoneOptions,
  } = props;

  return (
    <div className="flex min-w-0 flex-col gap-4">
      <div className="dialog-field">
        <div className="flex items-center justify-between gap-4">
          <span className="dialog-label !mb-0">调度</span>
          <UiSegmentedControl
            class_name="shrink-0"
            on_change={setScheduleKind}
            options={scheduleOptions.map((option) => ({
              label: option.label,
              value: option.key,
            }))}
            title="调度"
            value={scheduleKind}
          />
        </div>
      </div>

      {scheduleKind === "at" ? (
        <SingleRunPicker
          anchor_ref={singleAnchorRef}
          display={runAtDisplay}
          hour12={singleHour12}
          is_date_disabled={isSingleDateDisabled}
          is_hour_disabled={isSingleHourDisabled}
          is_open={isSinglePickerOpen}
          is_meridiem_disabled={isSingleMeridiemDisabled}
          is_minute_disabled={isSingleMinuteDisabled}
          is_second_disabled={isSingleSecondDisabled}
          meridiem={singleMeridiem}
          minute={singleMinute}
          month_label={`${singlePickerMonth.replace("-", "年")}月`}
          on_close={closeSinglePicker}
          on_date_select={onSingleDateSelect}
          on_hour_select={onSingleHourSelect}
          on_meridiem_select={onSingleMeridiemSelect}
          on_minute_select={onSingleMinuteSelect}
          on_next_month={onNextMonth}
          on_prev_month={onPrevMonth}
          on_second_select={onSingleSecondSelect}
          on_toggle={onSingleTriggerClick}
          second={singleSecond}
          selected_date={selectedRunDate}
          visible_days={singlePickerDays}
        />
      ) : null}

      {scheduleKind === "cron" ? (
        <div className="grid gap-4">
          <DailyTimePicker
            anchor_ref={dailyAnchorRef}
            display={dailyDisplay}
            hour12={dailyHour12}
            is_open={isDailyPickerOpen}
            meridiem={dailyMeridiem}
            minute={dailyMinute}
            on_close={closeDailyPicker}
            on_hour_select={onDailyHourSelect}
            on_meridiem_select={onDailyMeridiemSelect}
            on_minute_select={onDailyMinuteSelect}
            on_toggle={onDailyTriggerClick}
          />
          <div className="dialog-field">
            <span className="dialog-label">执行日</span>
            <div className="flex flex-wrap gap-2">
              {WEEKDAY_OPTIONS.map((option) => {
                const isSelected = selectedWeekdays.includes(option.key);
                return (
                  <UiChoiceButton
                    active={isSelected}
                    choice_size="md"
                    class_name="min-w-9 px-3"
                    key={option.key}
                    onClick={() => onToggleWeekday(option.key)}
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

      {scheduleKind === "every" ? (
        <UiPanel padding="md" variant="inset">
          <div className="flex flex-wrap items-center gap-3">
            <span className="text-sm font-semibold text-(--text-default)">每隔</span>
            <UiInput
              class_name="min-w-[96px]"
              control_size="lg"
              id="task-every-value"
              max="999"
              min="1"
              onChange={(e) => setEveryValue(e.target.value)}
              step="1"
              type="number"
              value={everyValue}
            />
            <UiSelectMenu
              aria_label="选择间隔单位"
              class_name="min-w-[132px]"
              id="task-every-unit"
              on_change={(value) => setEveryUnit(value as EveryUnit)}
              options={everyUnitOptions.map((option) => ({
                value: option.key,
                label: option.label,
              }))}
              surface="dialog"
              value={everyUnit}
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
          on_change={setTimezone}
          options={timezoneOptions.map((option) => ({
            value: option,
            label: option,
          }))}
          surface="dialog"
          value={timezone}
        />
      </div>

      <div className="dialog-field">
        <label className="dialog-label" htmlFor="task-instruction">
          {instructionLabel}
        </label>
        <UiTextarea
          class_name="resize-none"
          id="task-instruction"
          onChange={(e) => setInstruction(e.target.value)}
          placeholder={instructionPlaceholder}
          rows={4}
          value={instruction}
        />
      </div>

      <UiCheckboxRow
        checked={enabled}
        label="创建后立即启用任务"
        on_change={setEnabled}
      />

      {errorMessage ? (
        <UiStateBlock description={errorMessage} size="sm" title="任务配置无效" tone="danger" />
      ) : null}
    </div>
  );
}
