// INPUT: Scheduled 计划表单、提交/对账状态与 mutation failure 投影。
// OUTPUT: 实例级计划字段、具名选择/间隔输入、共享说明及保持原样的失败恢复面。
// POS: Scheduled 创建/编辑内容与时间表单；不发请求或自行判断 mutation 结果。

"use client";

import { useId } from "react";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { Locale, TranslationKey } from "@/shared/i18n/messages";
import { UiCheckboxRow } from "@/shared/ui/form/checkbox-row";
import { UiChoiceButton } from "@/shared/ui/form/choice";
import { UiField, UiInput, UiTextarea } from "@/shared/ui/form/form-control";
import { UiPanel } from "@/shared/ui/panel";
import { UiSelectMenu } from "@/shared/ui/menu/select-menu";
import { UiResourceState } from "@/shared/ui/display/resource-state";

import type { ScheduledTaskMutationFailureProjection } from "../../controller/scheduled-task-mutation-outcome";
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
  buildEveryUnitOptions,
  buildScheduleOptions,
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
  setCronExpression: (value: string) => void;
  setEveryUnit: (value: EveryUnit) => void;
  setEveryValue: (value: string) => void;
  setKind: (value: ScheduleKind) => void;
  setMonthlyDay: (value: string) => void;
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
  formError: string | null;
  form: Pick<TaskFormDraft, "enabled" | "executionKind" | "instruction">;
  formActions: {
    setEnabled: (value: boolean) => void;
    setInstruction: (value: string) => void;
  };
  isReconciling: boolean;
  isRestoredCreateIntent: boolean;
  isMutationReviewed: boolean;
  mutationFailure: ScheduledTaskMutationFailureProjection | null;
  onConfirmMutationReviewed: () => void;
  onReconcile: () => void;
  onStartNewCreateIntent: () => void;
  refs: Pick<TaskDialogRefs, "dailyPickerAnchorRef" | "singlePickerAnchorRef">;
  schedule: TaskScheduleDraft;
  view: TaskScheduleView;
}

const WEEKDAY_LABEL_KEYS: Record<Weekday, TranslationKey> = {
  fr: "capability.scheduled_dialog_weekday_fri",
  mo: "capability.scheduled_dialog_weekday_mon",
  sa: "capability.scheduled_dialog_weekday_sat",
  su: "capability.scheduled_dialog_weekday_sun",
  th: "capability.scheduled_dialog_weekday_thu",
  tu: "capability.scheduled_dialog_weekday_tue",
  we: "capability.scheduled_dialog_weekday_wed",
};

function formatPickerMonth(monthKey: string, locale: Locale): string {
  const [year, month] = monthKey.split("-").map(Number);
  if (locale === "zh") {
    return `${year}年${String(month).padStart(2, "0")}月`;
  }
  return new Intl.DateTimeFormat("en", {
    month: "long",
    timeZone: "UTC",
    year: "numeric",
  }).format(new Date(Date.UTC(year, month - 1, 1)));
}

export function TaskSchedulePanel({
  actions,
  formError,
  form,
  formActions,
  isReconciling,
  isRestoredCreateIntent,
  isMutationReviewed,
  mutationFailure,
  onConfirmMutationReviewed,
  onReconcile,
  onStartNewCreateIntent,
  refs,
  schedule,
  view,
}: TaskSchedulePanelProps) {
  const { locale, t } = useI18n();
  const formId = useId();
  const instructionLabel = t("capability.scheduled_dialog_instruction");

  return (
    <div className="flex min-w-0 flex-col gap-4">
      <UiField htmlFor={`${formId}-instruction`}>
        <UiTextarea
          aria-label={instructionLabel}
          className="resize-none"
          id={`${formId}-instruction`}
          onChange={(event) => formActions.setInstruction(event.target.value)}
          placeholder={t("capability.scheduled_dialog_instruction_placeholder")}
          required
          rows={3}
          value={form.instruction}
        />
      </UiField>


      <UiField className="grid grid-cols-[auto_minmax(0,1fr)] items-center gap-4 border-b border-(--divider-subtle-color) pb-2" htmlFor={`${formId}-kind`} label={t("capability.scheduled_dialog_schedule")}>
        <UiSelectMenu id={`${formId}-kind`} ariaLabel={t("capability.scheduled_dialog_schedule")}
          onChange={(value) => actions.setKind(value as ScheduleKind)}
          options={buildScheduleOptions(t).map((option) => ({label: option.label, value: option.key}))}
          surface="plain" className="text-right" value={schedule.kind} />
      </UiField>

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
          monthLabel={formatPickerMonth(view.singlePickerMonth, locale)}
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

      {schedule.kind === "cron" || schedule.kind === "monthly" ? (
        <div className="grid gap-4">
          {schedule.kind === "monthly" ? (
            <UiField
              description={t("capability.scheduled_dialog_monthly_day_help")}
              htmlFor={`${formId}-monthly-day`}
              label={t("capability.scheduled_dialog_monthly_day")}
              required
            >
              <UiInput
                id={`${formId}-monthly-day`}
                max="31"
                min="1"
                onChange={(event) => actions.setMonthlyDay(event.target.value)}
                required
                step="1"
                type="number"
                value={schedule.monthlyDay}
              />
            </UiField>
          ) : null}
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
          {schedule.kind === "cron" ? (
            <UiField
              description={t("capability.scheduled_dialog_execution_days_help")}
              label={t("capability.scheduled_dialog_execution_days")}
            >
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
                    {t(WEEKDAY_LABEL_KEYS[option.key])}
                  </UiChoiceButton>
                ))}
              </div>
            </UiField>
          ) : null}
        </div>
      ) : null}

      {schedule.kind === "custom" ? (
        <UiField
          description={t("capability.scheduled_dialog_custom_cron_help")}
          htmlFor={`${formId}-custom-cron`}
          label={t("capability.scheduled_dialog_custom_cron")}
          required
        >
          <UiInput
            id={`${formId}-custom-cron`}
            onChange={(event) => actions.setCronExpression(event.target.value)}
            placeholder="0 9 15 * *"
            required
            textRole="code"
            value={schedule.cronExpression}
          />
        </UiField>
      ) : null}

      {schedule.kind === "every" ? (
        <UiPanel padding="sm" variant="card">
          <div className="grid grid-cols-2 items-center gap-3">
            <UiField className="grid grid-cols-[auto_minmax(0,1fr)] items-center gap-2" htmlFor={`${formId}-every-value`} label={t("capability.scheduled_dialog_every")}>
              <UiInput
                id={`${formId}-every-value`}
                max="999"
                min="1"
                onChange={(event) => actions.setEveryValue(event.target.value)}
                step="1"
                type="number"
                value={schedule.everyValue}
              />
            </UiField>
            <UiField htmlFor={`${formId}-every-unit`}>
              <UiSelectMenu
                ariaLabel={t("capability.scheduled_dialog_select_interval_unit")}
                id={`${formId}-every-unit`}
                onChange={(value) => actions.setEveryUnit(value as EveryUnit)}
                options={buildEveryUnitOptions(t).map((option) => ({ label: option.label, value: option.key }))}
                surface="dialog"
                value={schedule.everyUnit}
              />
            </UiField>
          </div>
        </UiPanel>
      ) : null}

      {mutationFailure ? (
        <UiResourceState
          className="min-h-0 py-3"
          impact={t(mutationFailure.effect === "not_applied"
              ? "capability.scheduled_mutation_not_applied_impact"
            : mutationFailure.effect === "accepted"
              ? "capability.scheduled_mutation_accepted_impact"
              : mutationFailure.effect === "committed"
                ? "capability.scheduled_mutation_committed_impact"
                : "capability.scheduled_mutation_unknown_impact")}
          primaryAction={mutationFailure.blocksRepeat
            ? isRestoredCreateIntent
              ? {
                  label: t("capability.scheduled_dialog_create_start_new"),
                  onClick: onStartNewCreateIntent,
                }
              : isMutationReviewed
                ? {
                    label: t("capability.scheduled_mutation_review_unlock_action"),
                    onClick: onConfirmMutationReviewed,
                  }
                : {
                    busy: isReconciling,
                    busyLabel: t("capability.scheduled_dialog_reconciling"),
                    label: t("capability.scheduled_dialog_reconcile"),
                    onClick: onReconcile,
                  }
            : undefined}
          size="sm"
          state="error"
          title={t(mutationFailure.effect === "not_applied"
              ? "capability.scheduled_mutation_not_applied_title"
            : mutationFailure.effect === "accepted"
              ? "capability.scheduled_mutation_accepted_title"
              : mutationFailure.effect === "committed"
                ? "capability.scheduled_mutation_committed_title"
                : "capability.scheduled_mutation_unknown_title")}
        />
      ) : formError ? (
        <UiResourceState
          impact={t("capability.scheduled_dialog_invalid_impact")}
          size="sm"
          state="error"
          title={formError}
        />
      ) : null}
    </div>
  );
}

// 次要设置放在表单底部，与主计划字段共用同一草稿。
export function TaskScheduleAdvanced({ actions, form, formActions, schedule }: Pick<TaskSchedulePanelProps, "actions" | "form" | "formActions" | "schedule">) {
  const { t } = useI18n();
  const formId = useId();
  return <>
      <UiField htmlFor={`${formId}-timezone`} label={t("capability.scheduled_dialog_timezone")}>
        <UiSelectMenu
          ariaLabel={t("capability.scheduled_dialog_select_timezone")}
          id={`${formId}-timezone`}
          onChange={actions.setTimezone}
          options={TIMEZONE_OPTIONS.map((timezone) => ({
            label: timezone,
            value: timezone,
          }))}
          surface="dialog"
          value={schedule.timezone}
        />
      </UiField>



      <UiCheckboxRow
        checked={form.enabled}
        label={t("capability.scheduled_dialog_enabled")}
        onChange={formActions.setEnabled}
      />

  </>;
}
