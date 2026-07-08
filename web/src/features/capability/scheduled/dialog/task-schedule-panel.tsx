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
    closeDailyPicker: closeDailyPicker,
    closeSinglePicker: closeSinglePicker,
    dailyAnchorRef: dailyAnchorRef,
    dailyDisplay: dailyDisplay,
    dailyHour12: dailyHour12,
    dailyMeridiem: dailyMeridiem,
    dailyMinute: dailyMinute,
    enabled,
    errorMessage: errorMessage,
    everyUnit: everyUnit,
    everyUnitOptions: everyUnitOptions,
    everyValue: everyValue,
    instruction,
    instructionLabel: instructionLabel,
    instructionPlaceholder: instructionPlaceholder,
    isDailyPickerOpen: isDailyPickerOpen,
    isSinglePickerOpen: isSinglePickerOpen,
    isSingleDateDisabled: isSingleDateDisabled,
    isSingleHourDisabled: isSingleHourDisabled,
    isSingleMeridiemDisabled: isSingleMeridiemDisabled,
    isSingleMinuteDisabled: isSingleMinuteDisabled,
    isSingleSecondDisabled: isSingleSecondDisabled,
    onDailyHourSelect: onDailyHourSelect,
    onDailyMeridiemSelect: onDailyMeridiemSelect,
    onDailyMinuteSelect: onDailyMinuteSelect,
    onDailyTriggerClick: onDailyTriggerClick,
    onNextMonth: onNextMonth,
    onPrevMonth: onPrevMonth,
    onSingleDateSelect: onSingleDateSelect,
    onSingleHourSelect: onSingleHourSelect,
    onSingleMeridiemSelect: onSingleMeridiemSelect,
    onSingleMinuteSelect: onSingleMinuteSelect,
    onSingleSecondSelect: onSingleSecondSelect,
    onSingleTriggerClick: onSingleTriggerClick,
    onToggleWeekday: onToggleWeekday,
    runAtDisplay: runAtDisplay,
    scheduleKind: scheduleKind,
    scheduleOptions: scheduleOptions,
    selectedRunDate: selectedRunDate,
    selectedWeekdays: selectedWeekdays,
    setEnabled: setEnabled,
    setEveryUnit: setEveryUnit,
    setEveryValue: setEveryValue,
    setInstruction: setInstruction,
    setScheduleKind: setScheduleKind,
    setTimezone: setTimezone,
    singleAnchorRef: singleAnchorRef,
    singleHour12: singleHour12,
    singleMeridiem: singleMeridiem,
    singleMinute: singleMinute,
    singlePickerDays: singlePickerDays,
    singlePickerMonth: singlePickerMonth,
    singleSecond: singleSecond,
    timezone,
    timezoneOptions: timezoneOptions,
  } = props;

  return (
    <div className="flex min-w-0 flex-col gap-4">
      <div className="dialog-field">
        <div className="flex items-center justify-between gap-4">
          <span className="dialog-label !mb-0">调度</span>
          <UiSegmentedControl
            className="shrink-0"
            onChange={setScheduleKind}
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
          anchorRef={singleAnchorRef}
          display={runAtDisplay}
          hour12={singleHour12}
          isDateDisabled={isSingleDateDisabled}
          isHourDisabled={isSingleHourDisabled}
          isOpen={isSinglePickerOpen}
          isMeridiemDisabled={isSingleMeridiemDisabled}
          isMinuteDisabled={isSingleMinuteDisabled}
          isSecondDisabled={isSingleSecondDisabled}
          meridiem={singleMeridiem}
          minute={singleMinute}
          monthLabel={`${singlePickerMonth.replace("-", "年")}月`}
          onClose={closeSinglePicker}
          onDateSelect={onSingleDateSelect}
          onHourSelect={onSingleHourSelect}
          onMeridiemSelect={onSingleMeridiemSelect}
          onMinuteSelect={onSingleMinuteSelect}
          onNextMonth={onNextMonth}
          onPrevMonth={onPrevMonth}
          onSecondSelect={onSingleSecondSelect}
          onToggle={onSingleTriggerClick}
          second={singleSecond}
          selectedDate={selectedRunDate}
          visibleDays={singlePickerDays}
        />
      ) : null}

      {scheduleKind === "cron" ? (
        <div className="grid gap-4">
          <DailyTimePicker
            anchorRef={dailyAnchorRef}
            display={dailyDisplay}
            hour12={dailyHour12}
            isOpen={isDailyPickerOpen}
            meridiem={dailyMeridiem}
            minute={dailyMinute}
            onClose={closeDailyPicker}
            onHourSelect={onDailyHourSelect}
            onMeridiemSelect={onDailyMeridiemSelect}
            onMinuteSelect={onDailyMinuteSelect}
            onToggle={onDailyTriggerClick}
          />
          <div className="dialog-field">
            <span className="dialog-label">执行日</span>
            <div className="flex flex-wrap gap-2">
              {WEEKDAY_OPTIONS.map((option) => {
                const isSelected = selectedWeekdays.includes(option.key);
                return (
                  <UiChoiceButton
                    active={isSelected}
                    choiceSize="md"
                    className="min-w-9 px-3"
                    key={option.key}
                    onClick={() => onToggleWeekday(option.key)}
                    shape="pill"
                  >
                    {option.shortLabel}
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
              className="min-w-[96px]"
              controlSize="lg"
              id="task-every-value"
              max="999"
              min="1"
              onChange={(e) => setEveryValue(e.target.value)}
              step="1"
              type="number"
              value={everyValue}
            />
            <UiSelectMenu
              ariaLabel="选择间隔单位"
              className="min-w-[132px]"
              id="task-every-unit"
              onChange={(value) => setEveryUnit(value as EveryUnit)}
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
          ariaLabel="选择任务时区"
          id="task-timezone"
          onChange={setTimezone}
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
          className="resize-none"
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
        onChange={setEnabled}
      />

      {errorMessage ? (
        <UiStateBlock description={errorMessage} size="sm" title="任务配置无效" tone="danger" />
      ) : null}
    </div>
  );
}
