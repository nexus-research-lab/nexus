"use client";

import { UiCheckboxRow } from "@/shared/ui/checkbox-row";
import { UiChoiceButton } from "@/shared/ui/choice";
import { UiInput, UiTextarea } from "@/shared/ui/form-control";
import { UiPanel } from "@/shared/ui/panel";
import { UiSegmentedControl } from "@/shared/ui/segmented-control";
import { UiSelectMenu } from "@/shared/ui/menu/select-menu";
import { UiStateBlock } from "@/shared/ui/state-block";

import { DailyTimePicker } from "../../pickers/daily-time-picker";
import { SingleRunPicker } from "../../pickers/single-run-picker";
import {
  type Meridiem,
  type Weekday,
  WEEKDAY_OPTIONS,
} from "../../pickers/picker-types";
import type {
  EveryUnit,
  ScheduleKind,
  TaskDialogRefs,
  TaskFormDraft,
  TaskScheduleDraft,
} from "../scheduled-task-dialog-types";
import {
  EVERY_UNIT_OPTIONS,
  SCHEDULE_OPTIONS,
  TIMEZONE_OPTIONS,
} from "./task-schedule-model";

interface CalendarDay {
  label: string;
  muted: boolean;
  value: string;
}

interface MeridiemParts {
  hour12: string;
  meridiem: Meridiem;
  minute: string;
  second: string;
}

interface TaskScheduleView {
  dailyDisplay: string;
  dailyMeridiemParts: MeridiemParts;
  isDailyPickerOpen: boolean;
  isSinglePickerOpen: boolean;
  runAtDisplay: string;
  runAtParts: { date: string };
  singleMeridiemParts: MeridiemParts;
  singlePickerDays: CalendarDay[];
  singlePickerMonth: string;
}

interface TaskScheduleActions {
  closeDailyPicker: () => void;
  closeSinglePicker: () => void;
  goToNextMonth: () => void;
  goToPrevMonth: () => void;
  isSingleDateDisabled: (value: string) => boolean;
  isSingleHourDisabled: (value: string) => boolean;
  isSingleMeridiemDisabled: (value: Meridiem) => boolean;
  isSingleMinuteDisabled: (value: string) => boolean;
  isSingleSecondDisabled: (value: string) => boolean;
  setEveryUnit: (value: EveryUnit) => void;
  setEveryValue: (value: string) => void;
  setKind: (value: ScheduleKind) => void;
  setTimezone: (value: string) => void;
  toggleDailyPicker: () => void;
  toggleSinglePicker: () => void;
  toggleWeekday: (value: Weekday) => void;
  updateDailyPicker: (value: {
    hour12?: string;
    meridiem?: Meridiem;
    minute?: string;
  }) => void;
  updateSinglePicker: (value: {
    date?: string;
    hour12?: string;
    meridiem?: Meridiem;
    minute?: string;
    second?: string;
  }) => void;
}

interface TaskSchedulePanelProps {
  actions: TaskScheduleActions;
  errorMessage: string | null;
  form: Pick<TaskFormDraft, "enabled" | "executionKind" | "instruction">;
  formActions: {
    setEnabled: (value: boolean) => void;
    setInstruction: (value: string) => void;
  };
  refs: Pick<TaskDialogRefs, "dailyPickerAnchorRef" | "singlePickerAnchorRef">;
  schedule: TaskScheduleDraft;
  view: TaskScheduleView;
}

export function TaskSchedulePanel({
  actions,
  errorMessage,
  form,
  formActions,
  refs,
  schedule,
  view,
}: TaskSchedulePanelProps) {
  const instructionLabel = form.executionKind === "script"
    ? "脚本内容"
    : "任务指令";

  return (
    <div className="flex min-w-0 flex-col gap-4">
      <div className="dialog-field">
        <div className="flex items-center justify-between gap-4">
          <span className="dialog-label !mb-0">调度</span>
          <UiSegmentedControl
            className="shrink-0"
            onChange={actions.setKind}
            options={SCHEDULE_OPTIONS.map((option) => ({
              label: option.label,
              value: option.key,
            }))}
            title="调度"
            value={schedule.kind}
          />
        </div>
      </div>

      {schedule.kind === "at" ? (
        <SingleRunPicker
          anchorRef={refs.singlePickerAnchorRef}
          display={view.runAtDisplay}
          hour12={view.singleMeridiemParts.hour12}
          isDateDisabled={actions.isSingleDateDisabled}
          isHourDisabled={actions.isSingleHourDisabled}
          isOpen={view.isSinglePickerOpen}
          isMeridiemDisabled={actions.isSingleMeridiemDisabled}
          isMinuteDisabled={actions.isSingleMinuteDisabled}
          isSecondDisabled={actions.isSingleSecondDisabled}
          meridiem={view.singleMeridiemParts.meridiem}
          minute={view.singleMeridiemParts.minute}
          monthLabel={`${view.singlePickerMonth.replace("-", "年")}月`}
          onClose={actions.closeSinglePicker}
          onDateSelect={(date) => actions.updateSinglePicker({ date })}
          onHourSelect={(hour12) => actions.updateSinglePicker({ hour12 })}
          onMeridiemSelect={(meridiem) => actions.updateSinglePicker({ meridiem })}
          onMinuteSelect={(minute) => actions.updateSinglePicker({ minute })}
          onNextMonth={actions.goToNextMonth}
          onPrevMonth={actions.goToPrevMonth}
          onSecondSelect={(second) => actions.updateSinglePicker({ second })}
          onToggle={actions.toggleSinglePicker}
          second={view.singleMeridiemParts.second}
          selectedDate={view.runAtParts.date}
          visibleDays={view.singlePickerDays}
        />
      ) : null}

      {schedule.kind === "cron" ? (
        <div className="grid gap-4">
          <DailyTimePicker
            anchorRef={refs.dailyPickerAnchorRef}
            display={view.dailyDisplay}
            hour12={view.dailyMeridiemParts.hour12}
            isOpen={view.isDailyPickerOpen}
            meridiem={view.dailyMeridiemParts.meridiem}
            minute={view.dailyMeridiemParts.minute}
            onClose={actions.closeDailyPicker}
            onHourSelect={(hour12) => actions.updateDailyPicker({ hour12 })}
            onMeridiemSelect={(meridiem) => actions.updateDailyPicker({ meridiem })}
            onMinuteSelect={(minute) => actions.updateDailyPicker({ minute })}
            onToggle={actions.toggleDailyPicker}
          />
          <div className="dialog-field">
            <span className="dialog-label">执行日</span>
            <div className="flex flex-wrap gap-2">
              {WEEKDAY_OPTIONS.map((option) => (
                <UiChoiceButton
                  active={schedule.selectedWeekdays.includes(option.key)}
                  choiceSize="md"
                  className="min-w-9 px-3"
                  key={option.key}
                  onClick={() => actions.toggleWeekday(option.key)}
                  shape="pill"
                >
                  {option.shortLabel}
                </UiChoiceButton>
              ))}
            </div>
            <p className="text-xs leading-5 text-(--text-muted)">
              选中的日期会在这个时间执行；全选就是每天执行。
            </p>
          </div>
        </div>
      ) : null}

      {schedule.kind === "every" ? (
        <UiPanel padding="md" variant="inset">
          <div className="flex flex-wrap items-center gap-3">
            <span className="text-sm font-semibold text-(--text-default)">每隔</span>
            <UiInput
              className="min-w-[96px]"
              controlSize="lg"
              id="task-every-value"
              max="999"
              min="1"
              onChange={(event) => actions.setEveryValue(event.target.value)}
              step="1"
              type="number"
              value={schedule.everyValue}
            />
            <UiSelectMenu
              ariaLabel="选择间隔单位"
              className="min-w-[132px]"
              id="task-every-unit"
              onChange={(value) => actions.setEveryUnit(value as EveryUnit)}
              options={EVERY_UNIT_OPTIONS.map((option) => ({
                label: option.label,
                value: option.key,
              }))}
              surface="dialog"
              value={schedule.everyUnit}
            />
          </div>
        </UiPanel>
      ) : null}

      <div className="dialog-field">
        <label className="dialog-label" htmlFor="task-timezone">时区</label>
        <UiSelectMenu
          ariaLabel="选择任务时区"
          id="task-timezone"
          onChange={actions.setTimezone}
          options={TIMEZONE_OPTIONS.map((timezone) => ({
            label: timezone,
            value: timezone,
          }))}
          surface="dialog"
          value={schedule.timezone}
        />
      </div>

      <div className="dialog-field">
        <label className="dialog-label" htmlFor="task-instruction">
          {instructionLabel}
        </label>
        <UiTextarea
          className="resize-none"
          id="task-instruction"
          onChange={(event) => formActions.setInstruction(event.target.value)}
          placeholder={form.executionKind === "script"
            ? "输入要在目标工作区执行的 shell 脚本"
            : "输入 Agent 需要执行的指令"}
          rows={4}
          value={form.instruction}
        />
      </div>

      <UiCheckboxRow
        checked={form.enabled}
        label="创建后立即启用任务"
        onChange={formActions.setEnabled}
      />

      {errorMessage ? (
        <UiStateBlock
          description={errorMessage}
          size="sm"
          title="任务配置无效"
          tone="danger"
        />
      ) : null}
    </div>
  );
}
