"use client";

import { type RefObject } from "react";

import { useI18n } from "@/shared/i18n/i18n-context";
import type { TranslationKey } from "@/shared/i18n/messages";

import { PickerPopover } from "./picker-popover";
import {
  HOUR_12_OPTIONS,
  MERIDIEM_OPTIONS,
  MINUTE_OPTIONS,
  SECOND_OPTIONS,
  type Meridiem,
} from "./picker-types";
import {
  getPickerDateButtonClassName,
  PICKER_TRIGGER_CLASS_NAME,
} from "./picker-styles";
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

  return (
    <div className="dialog-field">
      <button
        className={PICKER_TRIGGER_CLASS_NAME}
        onClick={onToggle}
        ref={anchorRef}
        type="button"
      >
        <span>{display}</span>
        <span className="text-xl text-(--text-default)">+</span>
      </button>
      <PickerPopover anchorRef={anchorRef} isOpen={isOpen} onClose={onClose}>
        <div className="grid gap-4 md:grid-cols-[196px,minmax(0,1fr)]">
          <div className="space-y-3">
            <div className="flex items-center justify-between">
              <button className="text-sm font-semibold text-(--text-default)" onClick={onPrevMonth} type="button">
                {t("capability.scheduled_dialog_previous_month")}
              </button>
              <span className="text-base font-semibold text-(--text-strong)">{monthLabel}</span>
              <button className="text-sm font-semibold text-(--text-default)" onClick={onNextMonth} type="button">
                {t("capability.scheduled_dialog_next_month")}
              </button>
            </div>
            <div className="grid grid-cols-7 gap-1.5 text-center text-xs text-(--text-muted)">
              {CALENDAR_WEEKDAY_KEYS.map((key) => <div key={key}>{t(key)}</div>)}
            </div>
            <div className="grid grid-cols-7 gap-1.5">
              {visibleDays.map((day) => {
                const isSelected = day.value === selectedDate;
                const isDisabled = isDateDisabled(day.value);
                return (
                  <button
                    className={getPickerDateButtonClassName(isSelected, {
                      disabled: isDisabled,
                      muted: day.muted,
                    })}
                    disabled={isDisabled}
                    key={day.value}
                    onClick={() => onDateSelect(day.value)}
                    type="button"
                  >
                    {day.label}
                  </button>
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
