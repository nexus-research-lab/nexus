// INPUT: 单次运行的日期/时间投影、禁用规则、月份导航与选择命令。
// OUTPUT: 复用共享动作、Typography 和浮层的日历时间选择器。
// POS: Scheduled 单次运行字段组合；不拥有日历计算或计划草稿。

"use client";

import { type RefObject } from "react";

import { CalendarClock, ChevronLeft, ChevronRight } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import type { TranslationKey } from "@/shared/i18n/messages";
import { UiIconButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import { UiChoiceButton } from "@/shared/ui/form/choice";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

import { PickerPopover } from "./picker-popover";
import { PickerTrigger } from "./picker-trigger";
import {
  HOUR_12_OPTIONS,
  MERIDIEM_OPTIONS,
  MINUTE_OPTIONS,
  SECOND_OPTIONS,
  type Meridiem,
} from "./picker-types";
import { TimePickerColumn } from "./time-picker-column";

interface CalendarDay {
  label: string;
  muted: boolean;
  value: string;
}

interface SingleRunPickerProps {
  anchorRef: RefObject<HTMLButtonElement | null>;
  display: string;
  hour12: string;
  isDateDisabled: (value: string) => boolean;
  isHourDisabled: (value: string) => boolean;
  isOpen: boolean;
  isMeridiemDisabled: (value: Meridiem) => boolean;
  isMinuteDisabled: (value: string) => boolean;
  isSecondDisabled: (value: string) => boolean;
  meridiem: Meridiem;
  minute: string;
  monthLabel: string;
  onClose: () => void;
  onDateSelect: (value: string) => void;
  onHourSelect: (value: string) => void;
  onMeridiemSelect: (value: Meridiem) => void;
  onMinuteSelect: (value: string) => void;
  onNextMonth: () => void;
  onPrevMonth: () => void;
  onSecondSelect: (value: string) => void;
  onToggle: () => void;
  second: string;
  selectedDate: string;
  visibleDays: CalendarDay[];
}

const CALENDAR_WEEKDAY_KEYS: TranslationKey[] = [
  "capability.scheduled_dialog_weekday_sun",
  "capability.scheduled_dialog_weekday_mon",
  "capability.scheduled_dialog_weekday_tue",
  "capability.scheduled_dialog_weekday_wed",
  "capability.scheduled_dialog_weekday_thu",
  "capability.scheduled_dialog_weekday_fri",
  "capability.scheduled_dialog_weekday_sat",
];

export function SingleRunPicker(props: SingleRunPickerProps) {
  const { t } = useI18n();
  const {
    anchorRef,
    display,
    hour12,
    isDateDisabled,
    isHourDisabled,
    isOpen,
    isMeridiemDisabled,
    isMinuteDisabled,
    isSecondDisabled,
    meridiem,
    minute,
    monthLabel,
    onClose,
    onDateSelect,
    onHourSelect,
    onMeridiemSelect,
    onMinuteSelect,
    onNextMonth,
    onPrevMonth,
    onSecondSelect,
    onToggle,
    second,
    selectedDate,
    visibleDays,
  } = props;
  const pickerLabel = t("capability.scheduled_dialog_schedule");

  return (
    <div className="dialog-field">
      <PickerTrigger
        anchorRef={anchorRef}
        display={display}
        icon={CalendarClock}
        isOpen={isOpen}
        label={pickerLabel}
        onToggle={onToggle}
      />
      <PickerPopover
        anchorRef={anchorRef}
        ariaLabel={pickerLabel}
        isOpen={isOpen}
        onClose={onClose}
      >
        <div className="grid gap-4 md:grid-cols-[196px_minmax(0,1fr)]">
          <div className="space-y-3">
            <div className="flex items-center justify-between">
              <UiIconButton
                aria-label={t("capability.scheduled_dialog_previous_month")}
                onClick={onPrevMonth}
                size="sm"
                variant="ghost"
              >
                <ChevronLeft aria-hidden="true" className="h-4 w-4" />
              </UiIconButton>
              <span className={getUiTypographyClassName({
                role: "supporting",
                tone: "strong",
                weight: "semibold",
              })}>{monthLabel}</span>
              <UiIconButton
                aria-label={t("capability.scheduled_dialog_next_month")}
                onClick={onNextMonth}
                size="sm"
                variant="ghost"
              >
                <ChevronRight aria-hidden="true" className="h-4 w-4" />
              </UiIconButton>
            </div>
            <div className={cn(
              "grid grid-cols-7 gap-1.5 text-center",
              getUiTypographyClassName({ role: "caption", tone: "muted" }),
            )}>
              {CALENDAR_WEEKDAY_KEYS.map((key) => <div key={key}>{t(key)}</div>)}
            </div>
            <div className="grid grid-cols-7 gap-1.5">
              {visibleDays.map((day) => {
                const isSelected = day.value === selectedDate;
                const isDisabled = isDateDisabled(day.value);
                return (
                  <UiChoiceButton
                    active={isSelected}
                    disabled={isDisabled}
                    key={day.value}
                    muted={day.muted}
                    onClick={() => onDateSelect(day.value)}
                    variant="calendar"
                  >
                    {day.label}
                  </UiChoiceButton>
                );
              })}
            </div>
          </div>
          <div className="grid grid-cols-4 gap-2">
            <TimePickerColumn
              getLabel={(value) => t(value === "am"
                ? "capability.scheduled_dialog_meridiem_am"
                : "capability.scheduled_dialog_meridiem_pm")}
              isDisabled={isMeridiemDisabled}
              onSelect={onMeridiemSelect}
              options={MERIDIEM_OPTIONS}
              value={meridiem}
            />
            <TimePickerColumn
              isDisabled={isHourDisabled}
              onSelect={onHourSelect}
              options={HOUR_12_OPTIONS}
              value={hour12}
            />
            <TimePickerColumn
              isDisabled={isMinuteDisabled}
              onSelect={onMinuteSelect}
              options={MINUTE_OPTIONS}
              value={minute}
            />
            <TimePickerColumn
              isDisabled={isSecondDisabled}
              onSelect={onSecondSelect}
              options={SECOND_OPTIONS}
              value={second}
            />
          </div>
        </div>
      </PickerPopover>
    </div>
  );
}
